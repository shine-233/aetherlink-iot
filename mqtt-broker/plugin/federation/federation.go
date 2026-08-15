// 文件用途：维护 plugin\federation\federation.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package federation

import (
	"container/list"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
	"github.com/DrmagicE/gmqtt/persistence/subscription/mem"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/retained"
	"github.com/DrmagicE/gmqtt/server"
)

var _ server.Plugin = (*Federation)(nil)

const Name = "federation"

const defaultSessionSeenEventsCacheSize = 100

const (
	metadataNodeNameKey   = "node_name"
	metadataPeerSecretKey = "peer_secret"
)

func init() {
	if err := server.RegisterPlugin(Name, New); err != nil {
		panic(err)
	}
	config.RegisterDefaultPluginConfig(Name, &DefaultConfig)
}

func getSerfLogger(level string) (io.Writer, error) {
	logLevel := strings.ToUpper(level)
	var zapLevel zapcore.Level
	err := zapLevel.UnmarshalText([]byte(logLevel))
	if err != nil {
		return nil, err
	}
	zp, err := zap.NewStdLogAt(log, zapLevel)
	if err != nil {
		return nil, err
	}
	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(logLevel),
		Writer:   zp.Writer(),
	}
	return filter, nil
}

func getSerfConfig(cfg *Config, eventCh chan serf.Event, logOut io.Writer) *serf.Config {
	serfCfg := serf.DefaultConfig()
	serfCfg.SnapshotPath = cfg.SnapshotPath
	serfCfg.RejoinAfterLeave = cfg.RejoinAfterLeave
	serfCfg.NodeName = cfg.NodeName
	serfCfg.EventCh = eventCh
	host, port, _ := net.SplitHostPort(cfg.GossipAddr)
	if host != "" {
		serfCfg.MemberlistConfig.BindAddr = host
	}
	p, _ := strconv.Atoi(port)
	serfCfg.MemberlistConfig.BindPort = p

	// set advertise
	host, port, _ = net.SplitHostPort(cfg.AdvertiseGossipAddr)
	if host != "" {
		serfCfg.MemberlistConfig.AdvertiseAddr = host
	}
	p, _ = strconv.Atoi(port)
	serfCfg.MemberlistConfig.AdvertisePort = p

	serfCfg.Tags = map[string]string{"fed_addr": cfg.AdvertiseFedAddr}
	serfCfg.LogOutput = logOut
	serfCfg.MemberlistConfig.LogOutput = logOut
	return serfCfg
}

func New(config config.Config) (server.Plugin, error) {
	log = server.LoggerWithField(zap.String("plugin", Name))
	cfg := config.Plugins[Name].(*Config)
	f := &Federation{
		config:        cfg,
		nodeName:      cfg.NodeName,
		localSubStore: &localSubStore{},
		fedSubStore: &fedSubStore{
			TrieDB:     mem.NewStore(),
			sharedSent: map[string]uint64{},
		},
		serfEventCh: make(chan serf.Event, 10000),
		sessionMgr: &sessionMgr{
			sessions: map[string]*session{},
		},
		peers: make(map[string]*peer),
		exit:  make(chan struct{}),
		wg:    &sync.WaitGroup{},
	}
	logOut, err := getSerfLogger(config.Log.Level)
	if err != nil {
		return nil, err
	}
	serfCfg := getSerfConfig(cfg, f.serfEventCh, logOut)
	s, err := serf.Create(serfCfg)
	if err != nil {
		return nil, err
	}
	f.serf = s
	return f, nil
}

var log *zap.Logger

type Federation struct {
	config      *Config
	nodeName    string
	serfMu      sync.Mutex
	serf        iSerf
	serfEventCh chan serf.Event
	sessionMgr  *sessionMgr
	// localSubStore store the subscriptions for the local node.
	// The local node will only broadcast "new subscriptions" to other nodes.
	// "New subscription" is the first subscription for a topic name.
	// It means that if two client in the local node subscribe the same topic, only the first subscription will be broadcast.
	localSubStore *localSubStore
	// fedSubStore store federation subscription tree which take nodeName as the subscriber identifier.
	// It is used to determine which node the incoming message should be routed to.
	fedSubStore *fedSubStore
	// retainedStore store is the retained store of the gmqtt core.
	// Retained message will be broadcast to other nodes in the federation.
	retainedStore retained.Store
	publisher     server.Publisher
	exit          chan struct{}
	exitOnce      sync.Once
	grpcMu        sync.Mutex
	grpcServer    *grpc.Server
	grpcListener  net.Listener
	memberMu      sync.Mutex
	peers         map[string]*peer
	wg            *sync.WaitGroup
}

type fedSubStore struct {
	*mem.TrieDB
	sharedMu sync.Mutex
	// sharedSent store the number of shared topic sent.
	// It is used to select which node the message should be send to with round-robin strategy
	sharedSent map[string]uint64
}

type sessionMgr struct {
	sync.RWMutex
	sessions map[string]*session
}

func (s *sessionMgr) add(nodeName string, id string) (cleanStart bool, nextID uint64) {
	s.Lock()
	defer s.Unlock()
	if v, ok := s.sessions[nodeName]; ok && v.id == id {
		// Read nextEventID under the same lock used by EventStream.
		v.mu.Lock()
		nextID = v.nextEventID
		v.mu.Unlock()
	} else {
		// v.id != id indicates that the client side may recover from crash and need to rebuild the full state.
		cleanStart = true
		if ok {
			close(v.close)
		}
	}
	if cleanStart {
		s.sessions[nodeName] = &session{
			id:          id,
			nodeName:    nodeName,
			seenEvents:  newLRUCache(defaultSessionSeenEventsCacheSize),
			nextEventID: 0,
			close:       make(chan struct{}),
		}
	}
	return
}

func (s *sessionMgr) del(nodeName string) {
	s.Lock()
	defer s.Unlock()
	if sess := s.sessions[nodeName]; sess != nil {
		close(sess.close)
	}
	delete(s.sessions, nodeName)
}

func (s *sessionMgr) get(nodeName string) *session {
	s.RLock()
	defer s.RUnlock()
	return s.sessions[nodeName]
}

// ForceLeave forces a member of a Serf cluster to enter the "left" state.
// Note that if the member is still actually alive, it will eventually rejoin the cluster.
// The true purpose of this method is to force remove "failed" nodes
// See https://www.serf.io/docs/commands/force-leave.html for details.
func (f *Federation) ForceLeave(ctx context.Context, req *ForceLeaveRequest) (*empty.Empty, error) {
	if req.NodeName == "" {
		return nil, errors.New("host can not be empty")
	}
	return &empty.Empty{}, f.serf.RemoveFailedNode(req.NodeName)
}

// ListMembers lists all known members in the Serf cluster.
func (f *Federation) ListMembers(ctx context.Context, req *empty.Empty) (resp *ListMembersResponse, err error) {
	resp = &ListMembersResponse{}
	for _, v := range f.serf.Members() {
		resp.Members = append(resp.Members, &Member{
			Name:   v.Name,
			Addr:   net.JoinHostPort(v.Addr.String(), strconv.Itoa(int(v.Port))),
			Tags:   v.Tags,
			Status: Status(v.Status),
		})
	}
	return resp, nil
}

// Leave triggers a graceful leave for the local node.
// This is used to ensure other nodes see the node as "left" instead of "failed".
// Note that a leaved node cannot re-join the cluster unless you restart the leaved node.
func (f *Federation) Leave(ctx context.Context, req *empty.Empty) (resp *empty.Empty, err error) {
	return &empty.Empty{}, f.serf.Leave()
}

func (f *Federation) mustEmbedUnimplementedMembershipServer() {
	return
}

// Join tells the local node to join the an existing cluster.
// See https://www.serf.io/docs/commands/join.html for details.
func (f *Federation) Join(ctx context.Context, req *JoinRequest) (resp *empty.Empty, err error) {
	for k, v := range req.Hosts {
		req.Hosts[k], err = getAddr(v, DefaultGossipPort, "hosts", false)
		if err != nil {
			return &empty.Empty{}, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	_, err = f.serf.Join(req.Hosts, true)
	if err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

type localSubStore struct {
	localStore server.SubscriptionService
	sync.Mutex
	// [clientID][topicName]
	index map[string]map[string]struct{}
	// topics store the reference counter for each topic. (map[topicName]uint64)
	topics map[string]uint64
}

// init loads all subscriptions from gmqtt core into federation plugin.
func (l *localSubStore) init(sub server.SubscriptionService) {
	l.localStore = sub
	l.index = make(map[string]map[string]struct{})
	l.topics = make(map[string]uint64)
	l.Lock()
	defer l.Unlock()
	// copy and convert subscription tree into localSubStore
	sub.Iterate(func(clientID string, sub *gmqtt.Subscription) bool {
		l.subscribeLocked(clientID, sub.GetFullTopicName())
		return true
	}, subscription.IterationOptions{
		Type: subscription.TypeAll,
	})
}

// subscribe subscribe the topicName for the client and increase the reference counter of the topicName.
// It returns whether the subscription is new
func (l *localSubStore) subscribe(clientID string, topicName string) (new bool) {
	l.Lock()
	defer l.Unlock()
	return l.subscribeLocked(clientID, topicName)
}

func (l *localSubStore) subscribeLocked(clientID string, topicName string) (new bool) {
	if _, ok := l.index[clientID]; !ok {
		l.index[clientID] = make(map[string]struct{})
	}
	if _, ok := l.index[clientID][topicName]; !ok {
		l.index[clientID][topicName] = struct{}{}
		l.topics[topicName]++
		if l.topics[topicName] == 1 {
			return true
		}
	}
	return false
}

func (l *localSubStore) decTopicCounterLocked(topicName string) {
	if _, ok := l.topics[topicName]; ok {
		l.topics[topicName]--
		if l.topics[topicName] <= 0 {
			delete(l.topics, topicName)
		}
	}
}

// unsubscribe unsubscribe the topicName for the client and decrease the reference counter of the topicName.
// It returns whether the topicName is removed (reference counter == 0)
func (l *localSubStore) unsubscribe(clientID string, topicName string) (remove bool) {
	l.Lock()
	defer l.Unlock()
	if v, ok := l.index[clientID]; ok {
		if _, ok := v[topicName]; ok {
			delete(v, topicName)
			if len(v) == 0 {
				delete(l.index, clientID)
			}
			l.decTopicCounterLocked(topicName)
			return l.topics[topicName] == 0
		}
	}
	return false

}

// unsubscribeAll unsubscribes all topics for the given client.
// Typically, this function is called when the client session has terminated.
// It returns any topic that is removed.
func (l *localSubStore) unsubscribeAll(clientID string) (remove []string) {
	l.Lock()
	defer l.Unlock()
	for topicName := range l.index[clientID] {
		l.decTopicCounterLocked(topicName)
		if l.topics[topicName] == 0 {
			remove = append(remove, topicName)
		}
	}
	delete(l.index, clientID)
	return remove
}

type session struct {
	id       string
	nodeName string
	// mu protects nextEventID across Hello and EventStream.
	mu          sync.Mutex
	nextEventID uint64
	// seenEvents cache recently seen events to avoid duplicate events.
	seenEvents *lruCache
	close      chan struct{}
}

// lruCache is the cache for recently seen events.
type lruCache struct {
	l     *list.List
	items map[uint64]struct{}
	size  int
}

func newLRUCache(size int) *lruCache {
	return &lruCache{
		l:     list.New(),
		items: make(map[uint64]struct{}),
		size:  size,
	}
}

func (l *lruCache) set(id uint64) (exist bool) {
	if _, ok := l.items[id]; ok {
		return true
	}
	if l.size == len(l.items) {
		elem := l.l.Front()
		delete(l.items, elem.Value.(uint64))
		l.l.Remove(elem)
	}
	l.items[id] = struct{}{}
	l.l.PushBack(id)
	return false
}

func (f *Federation) peerSecret() string {
	if f == nil || f.config == nil {
		return ""
	}
	return f.config.PeerSecret
}

func getPeerMetadataFromContext(ctx context.Context, rpcName string) (metadata.MD, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, "", status.Errorf(codes.DataLoss, "%s: failed to get metadata", rpcName)
	}
	s := md.Get(metadataNodeNameKey)
	if len(s) == 0 {
		return nil, "", status.Errorf(codes.InvalidArgument, "%s: missing node_name metadata", rpcName)
	}
	nodeName := s[0]
	if nodeName == "" {
		return nil, "", status.Errorf(codes.InvalidArgument, "%s: missing node_name metadata", rpcName)
	}
	return md, nodeName, nil
}

func (f *Federation) authenticatePeer(ctx context.Context, rpcName string) (string, error) {
	md, nodeName, err := getPeerMetadataFromContext(ctx, rpcName)
	if err != nil {
		return "", err
	}
	expected := f.peerSecret()
	if expected == "" {
		return "", status.Errorf(codes.Unauthenticated, "%s: federation peer_secret is not configured", rpcName)
	}
	got := md.Get(metadataPeerSecretKey)
	if len(got) == 0 || got[0] == "" {
		return "", status.Errorf(codes.Unauthenticated, "%s: missing peer_secret metadata", rpcName)
	}
	if subtle.ConstantTimeCompare([]byte(got[0]), []byte(expected)) != 1 {
		return "", status.Errorf(codes.Unauthenticated, "%s: invalid peer_secret metadata", rpcName)
	}
	return nodeName, nil
}

// Hello is the handler for the handshake process before opening the event stream.
func (f *Federation) Hello(ctx context.Context, req *ClientHello) (resp *ServerHello, err error) {
	nodeName, err := f.authenticatePeer(ctx, "Hello")
	if err != nil {
		return nil, err
	}
	f.memberMu.Lock()
	p := f.peers[nodeName]
	f.memberMu.Unlock()
	if p == nil {
		return nil, status.Errorf(codes.Internal, "Hello: the node [%s] has not yet joined", nodeName)
	}

	cleanStart, nextID := f.sessionMgr.add(nodeName, req.SessionId)
	if cleanStart {
		_ = f.fedSubStore.UnsubscribeAll(nodeName)
	}
	resp = &ServerHello{
		CleanStart:  cleanStart,
		NextEventId: nextID,
	}
	return resp, nil
}

func (f *Federation) eventStreamHandler(sess *session, in *Event) (*Ack, error) {
	eventID := in.Id
	// duplicated event, ignore it
	if sess.seenEvents.set(eventID) {
		log.Warn("ignore duplicated event", zap.String("event", in.String()))
		return &Ack{
			EventId: eventID,
		}, nil
	}
	if sub := in.GetSubscribe(); sub != nil {
		_, _ = f.fedSubStore.Subscribe(sess.nodeName, &gmqtt.Subscription{
			ShareName:   sub.ShareName,
			TopicFilter: sub.TopicFilter,
		})
		return &Ack{EventId: eventID}, nil
	}
	if msg := in.GetMessage(); msg != nil {
		pubMsg := eventToMessage(msg)
		f.publisher.Publish(pubMsg)
		if pubMsg.Retained {
			f.retainedStore.AddOrReplace(pubMsg)
		}
		return &Ack{EventId: eventID}, nil
	}
	if unsub := in.GetUnsubscribe(); unsub != nil {
		_ = f.fedSubStore.Unsubscribe(sess.nodeName, unsub.TopicName)
		return &Ack{EventId: eventID}, nil
	}
	return nil, status.Errorf(codes.InvalidArgument, "EventStream: unsupported event body for event id %d", eventID)
}

func (f *Federation) EventStream(stream Federation_EventStreamServer) (err error) {
	defer func() {
		if err != nil && err != io.EOF {
			log.Error("EventStream error", zap.Error(err))
		}
	}()
	nodeName, err := f.authenticatePeer(stream.Context(), "EventStream")
	if err != nil {
		return err
	}
	sess := f.sessionMgr.get(nodeName)
	if sess == nil {
		return status.Errorf(codes.Internal, "EventStream: node [%s] does not exist", nodeName)
	}

	type recvResult struct {
		event *Event
		err   error
	}
	for {
		recvCh := make(chan recvResult, 1)
		go func() {
			event, recvErr := stream.Recv()
			recvCh <- recvResult{event: event, err: recvErr}
		}()

		var in *Event
		select {
		case <-sess.close:
			return fmt.Errorf("EventStream: the session of node [%s] has been closed", nodeName)
		case <-stream.Context().Done():
			return stream.Context().Err()
		case result := <-recvCh:
			if result.err == io.EOF {
				return nil
			}
			if result.err != nil {
				return result.err
			}
			in = result.event
		}

		if ce := log.Check(zapcore.DebugLevel, "event received"); ce != nil {
			ce.Write(zap.String("event", in.String()))
		}

		ack, handleErr := f.eventStreamHandler(sess, in)
		if handleErr != nil {
			return handleErr
		}

		if sendErr := stream.Send(ack); sendErr != nil {
			return sendErr
		}
		if ce := log.Check(zapcore.DebugLevel, "event ack sent"); ce != nil {
			ce.Write(zap.Uint64("id", ack.EventId))
		}
		sess.mu.Lock()
		sess.nextEventID = ack.EventId + 1
		sess.mu.Unlock()
	}
}

func (f *Federation) mustEmbedUnimplementedFederationServer() {
	return
}

func (f *Federation) setGRPCServer(srv *grpc.Server, l net.Listener) {
	f.grpcMu.Lock()
	defer f.grpcMu.Unlock()
	f.grpcServer = srv
	f.grpcListener = l
}

func (f *Federation) stopGRPCServer() {
	f.grpcMu.Lock()
	srv := f.grpcServer
	l := f.grpcListener
	f.grpcServer = nil
	f.grpcListener = nil
	f.grpcMu.Unlock()

	if srv != nil {
		srv.Stop()
	}
	if l != nil {
		_ = l.Close()
	}
}

func (f *Federation) stopPeers() {
	if f.exit == nil {
		return
	}
	f.exitOnce.Do(func() {
		close(f.exit)
	})
	f.memberMu.Lock()
	peers := make([]*peer, 0, len(f.peers))
	for _, p := range f.peers {
		peers = append(peers, p)
	}
	f.memberMu.Unlock()
	for _, p := range peers {
		p.stop()
	}
}

var registerAPI = func(service server.Server, f *Federation) error {
	apiRegistrar := service.APIRegistrar()
	RegisterMembershipServer(apiRegistrar, f)
	err := apiRegistrar.RegisterHTTPHandler(RegisterMembershipHandlerFromEndpoint)
	return err
}

func (f *Federation) Load(service server.Server) error {
	err := registerAPI(service, f)
	if err != nil {
		return err
	}
	f.localSubStore.init(service.SubscriptionService())
	f.retainedStore = service.RetainedService()
	f.publisher = service.Publisher()
	srv := grpc.NewServer()
	RegisterFederationServer(srv, f)
	l, err := net.Listen("tcp", f.config.FedAddr)
	if err != nil {
		return err
	}
	f.setGRPCServer(srv, l)
	go func() {
		// Recover from unexpected gRPC Serve panics without crashing the broker.
		defer func() {
			if re := recover(); re != nil {
				log.Error("grpc Serve panic", zap.Any("recover", re))
			}
		}()
		err := srv.Serve(l)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) && !errors.Is(err, net.ErrClosed) {
			log.Error("grpc Serve error", zap.Error(err))
		}
	}()
	t := time.NewTimer(0)
	timeout := time.NewTimer(f.config.RetryTimeout)
	for {
		select {
		case <-timeout.C:
			log.Error("retry timeout", zap.Error(err))
			f.stopGRPCServer()
			if err != nil {
				err = fmt.Errorf("retry timeout: %s", err.Error())
				return err
			}
			return errors.New("retry timeout")
		case <-t.C:
			err = f.startSerf(t)
			if err == nil {
				log.Info("retry join succeed")
				return nil
			}
			log.Info("retry join failed", zap.Error(err))
		}
	}
}

func (f *Federation) Unload() error {
	f.stopGRPCServer()
	f.stopPeers()
	if f.serf == nil {
		return nil
	}
	err := f.serf.Leave()
	shutdownErr := f.serf.Shutdown()
	if err != nil {
		return err
	}
	return shutdownErr
}

func (f *Federation) Name() string {
	return Name
}

func messageToEvent(msg *gmqtt.Message) *Message {
	eventMsg := &Message{
		TopicName:       msg.Topic,
		Payload:         msg.Payload,
		Qos:             uint32(msg.QoS),
		Retained:        msg.Retained,
		ContentType:     msg.ContentType,
		CorrelationData: string(msg.CorrelationData),
		MessageExpiry:   msg.MessageExpiry,
		PayloadFormat:   uint32(msg.PayloadFormat),
		ResponseTopic:   msg.ResponseTopic,
	}
	for _, v := range msg.UserProperties {
		ppt := &UserProperty{
			K: make([]byte, len(v.K)),
			V: make([]byte, len(v.V)),
		}
		copy(ppt.K, v.K)
		copy(ppt.V, v.V)
		eventMsg.UserProperties = append(eventMsg.UserProperties, ppt)
	}
	return eventMsg
}

func eventToMessage(event *Message) *gmqtt.Message {
	pubMsg := &gmqtt.Message{
		QoS:             byte(event.Qos),
		Retained:        event.Retained,
		Topic:           event.TopicName,
		Payload:         event.Payload,
		ContentType:     event.ContentType,
		CorrelationData: []byte(event.CorrelationData),
		MessageExpiry:   event.MessageExpiry,
		PayloadFormat:   packets.PayloadFormat(event.PayloadFormat),
		ResponseTopic:   event.ResponseTopic,
	}
	for _, v := range event.UserProperties {
		pubMsg.UserProperties = append(pubMsg.UserProperties, packets.UserProperty{
			K: v.K,
			V: v.V,
		})
	}
	return pubMsg
}
