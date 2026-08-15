// 文件用途：维护 plugin\federation\federation_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package federation

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription/mem"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/server"
)

const testPeerSecret = "cluster-test-secret"

func newTestFederation() *Federation {
	return &Federation{
		config: &Config{
			NodeName:   "node0",
			PeerSecret: testPeerSecret,
		},
		fedSubStore: &fedSubStore{
			TrieDB:     mem.NewStore(),
			sharedSent: map[string]uint64{},
		},
		sessionMgr: &sessionMgr{
			sessions: map[string]*session{},
		},
		peers: map[string]*peer{},
	}
}

func TestLocalSubStore_init(t *testing.T) {
	a := assert.New(t)
	var tt = struct {
		clientID []string
		topics   []*gmqtt.Subscription
		expected map[string]uint64
	}{
		clientID: []string{"client1", "client2", "client3"},
		topics: []*gmqtt.Subscription{
			{
				ShareName:   "abc",
				TopicFilter: "filter1",
			}, {
				TopicFilter: "filter2",
			}, {
				TopicFilter: "filter3",
			},
		},
		expected: map[string]uint64{
			"$share/abc/filter1": 3,
			"filter2":            3,
			"filter3":            3,
		},
	}
	l := &localSubStore{}
	subStore := mem.NewStore()
	for _, v := range tt.clientID {
		_, err := subStore.Subscribe(v, tt.topics...)
		a.Nil(err)
	}
	l.init(subStore)
	l.Lock()
	a.Equal(tt.expected, l.topics)
	l.Unlock()
}

func TestLocalSubStore_sub_unsub(t *testing.T) {
	a := assert.New(t)

	l := &localSubStore{}
	subStore := mem.NewStore()
	l.init(subStore)

	a.True(l.subscribe("client1", "topic1"))
	// test duplicated subscribe
	a.False(l.subscribe("client1", "topic1"))
	a.Equal(map[string]uint64{
		"topic1": 1,
	}, l.topics)
	a.Equal(map[string]map[string]struct{}{
		"client1": {
			"topic1": struct{}{},
		},
	}, l.index)

	// test duplicated subscribe
	a.False(l.subscribe("client2", "topic1"))
	a.Equal(map[string]uint64{
		"topic1": 2,
	}, l.topics)
	a.Equal(map[string]map[string]struct{}{
		"client1": {
			"topic1": struct{}{},
		},
		"client2": {
			"topic1": struct{}{},
		},
	}, l.index)

	a.True(l.subscribe("client3", "topic2"))
	a.Equal(map[string]uint64{
		"topic1": 2,
		"topic2": 1,
	}, l.topics)
	a.Equal(map[string]map[string]struct{}{
		"client1": {
			"topic1": struct{}{},
		},
		"client2": {
			"topic1": struct{}{},
		},
		"client3": {
			"topic2": struct{}{},
		},
	}, l.index)

	// test unsubscribe not exists topic
	a.False(l.unsubscribe("client4", "topic1"))
	a.Equal(map[string]uint64{
		"topic1": 2,
		"topic2": 1,
	}, l.topics)

	for i := 0; i < 1; i++ {
		a.False(l.unsubscribe("client2", "topic1"))
		a.Equal(map[string]uint64{
			"topic1": 1,
			"topic2": 1,
		}, l.topics)
		a.Equal(map[string]map[string]struct{}{
			"client1": {
				"topic1": struct{}{},
			},
			"client3": {
				"topic2": struct{}{},
			},
		}, l.index)
	}

	unsub := l.unsubscribeAll("client3")
	a.Equal([]string{"topic2"}, unsub)
	a.Equal(map[string]uint64{
		"topic1": 1,
	}, l.topics)

	a.Equal(map[string]map[string]struct{}{
		"client1": {
			"topic1": struct{}{},
		},
	}, l.index)

	a.Len(l.unsubscribeAll("client3"), 0)

	a.True(l.unsubscribe("client1", "topic1"))
	a.False(l.unsubscribe("client1", "topic1"))
}

func TestMessageToEvent(t *testing.T) {
	a := assert.New(t)
	var tt = []struct {
		msg      *gmqtt.Message
		expected *Message
	}{
		{
			msg: &gmqtt.Message{
				Dup:             true,
				QoS:             1,
				Retained:        true,
				Topic:           "topic1",
				Payload:         []byte("topic1"),
				PacketID:        1,
				ContentType:     "ct",
				CorrelationData: []byte("data"),
				MessageExpiry:   1,
				PayloadFormat:   1,
				ResponseTopic:   "respTopic",
				UserProperties: []packets.UserProperty{
					{
						K: []byte("K"),
						V: []byte("V"),
					},
				},
			},
			expected: &Message{
				TopicName:       "topic1",
				Payload:         []byte("topic1"),
				Qos:             1,
				Retained:        true,
				ContentType:     "ct",
				CorrelationData: "data",
				MessageExpiry:   1,
				PayloadFormat:   1,
				ResponseTopic:   "respTopic",
				UserProperties: []*UserProperty{
					{
						K: []byte("K"),
						V: []byte("V"),
					},
				},
			},
		},
	}
	for _, v := range tt {
		a.Equal(v.expected, messageToEvent(v.msg))
	}

}

func TestLRUCache(t *testing.T) {
	a := assert.New(t)
	lcache := newLRUCache(1)
	a.False(lcache.set(1))
	a.True(lcache.set(1))
	a.False(lcache.set(2))
	a.Len(lcache.items, 1)
	a.Equal(1, lcache.l.Len())
}

func TestFederation_eventStreamHandler(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	f := newTestFederation()

	pub := server.NewMockPublisher(ctrl)
	f.publisher = pub

	sess := &session{
		id:          "abc",
		nodeName:    "node1",
		nextEventID: 0,
		seenEvents:  newLRUCache(3),
	}
	var ack *Ack
	ack, err := f.eventStreamHandler(sess, &Event{
		Id: 0,
		Event: &Event_Subscribe{
			Subscribe: &Subscribe{
				ShareName:   "",
				TopicFilter: "a",
			},
		},
	})
	a.NoError(err)
	a.EqualValues(0, ack.EventId)
	sts, _ := f.fedSubStore.GetClientStats("node1")
	a.EqualValues(1, sts.SubscriptionsCurrent)

	msgEvent := &Event_Message{
		Message: &Message{
			TopicName: "a",
			Payload:   []byte("b"),
			Qos:       1,
		},
	}
	pub.EXPECT().Publish(eventToMessage(msgEvent.Message))
	ack, err = f.eventStreamHandler(sess, &Event{
		Id:    1,
		Event: msgEvent,
	})
	a.NoError(err)
	a.EqualValues(1, ack.EventId)
	ack, err = f.eventStreamHandler(sess, &Event{
		Id: 2,
		Event: &Event_Unsubscribe{
			Unsubscribe: &Unsubscribe{
				TopicName: "a",
			},
		},
	})
	a.NoError(err)
	sts, _ = f.fedSubStore.GetClientStats("node1")
	a.EqualValues(0, sts.SubscriptionsCurrent)
	a.EqualValues(2, ack.EventId)

	// send duplicated event
	ack, err = f.eventStreamHandler(sess, &Event{
		Id: 0,
		Event: &Event_Subscribe{
			Subscribe: &Subscribe{
				ShareName:   "",
				TopicFilter: "a",
			},
		},
	})
	a.NoError(err)
	a.EqualValues(0, ack.EventId)
	sts, _ = f.fedSubStore.GetClientStats("node1")
	a.EqualValues(0, sts.SubscriptionsCurrent)

}

func TestFederation_eventStreamHandlerRejectsUnsupportedEvent(t *testing.T) {
	a := assert.New(t)
	f := newTestFederation()
	sess := &session{
		id:         "abc",
		nodeName:   "node1",
		seenEvents: newLRUCache(3),
	}

	ack, err := f.eventStreamHandler(sess, &Event{Id: 9})

	a.Nil(ack)
	a.Error(err)
	a.Contains(err.Error(), "unsupported event body")
}

func TestFederation_getSerfConfig(t *testing.T) {
	a := assert.New(t)

	cfg := &Config{
		NodeName:         "node",
		FedAddr:          "127.0.0.1:1234",
		AdvertiseFedAddr: "127.0.0.1:1235",
		GossipAddr:       "127.0.0.1:1236",
		RetryInterval:    5 * time.Second,
		RetryTimeout:     10 * time.Second,
		SnapshotPath:     "./path",
		RejoinAfterLeave: true,
	}

	serfCfg := getSerfConfig(cfg, nil, nil)

	a.Equal(cfg.NodeName, serfCfg.NodeName)
	a.Equal(cfg.AdvertiseFedAddr, serfCfg.Tags["fed_addr"])
	host, port, _ := net.SplitHostPort(cfg.GossipAddr)
	a.Equal(host, serfCfg.MemberlistConfig.BindAddr)
	portNumber, _ := strconv.Atoi(port)
	a.EqualValues(portNumber, serfCfg.MemberlistConfig.BindPort)

	host, port, _ = net.SplitHostPort(cfg.AdvertiseGossipAddr)
	a.Equal(host, serfCfg.MemberlistConfig.AdvertiseAddr)
	portNumber, _ = strconv.Atoi(port)
	a.EqualValues(portNumber, serfCfg.MemberlistConfig.AdvertisePort)

	a.Equal(cfg.SnapshotPath, serfCfg.SnapshotPath)
	a.Equal(cfg.RejoinAfterLeave, serfCfg.RejoinAfterLeave)
}

func TestFederationLoadRetryTimeoutReleasesGRPCListener(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	a.NoError(err)
	addr := listener.Addr().String()
	a.NoError(listener.Close())

	oldRegisterAPI := registerAPI
	registerAPI = func(service server.Server, f *Federation) error {
		return nil
	}
	t.Cleanup(func() {
		registerAPI = oldRegisterAPI
	})

	mockSerf := NewMockiSerf(ctrl)
	mockSerf.EXPECT().Join(gomock.Any(), true).Return(0, errors.New("join failed")).AnyTimes()

	service := server.NewMockServer(ctrl)
	service.EXPECT().SubscriptionService().Return(mem.NewStore())
	service.EXPECT().RetainedService().Return(server.NewMockRetainedService(ctrl))
	service.EXPECT().Publisher().Return(server.NewMockPublisher(ctrl))

	f := &Federation{
		config: &Config{
			NodeName:      "node-load-failure",
			FedAddr:       addr,
			RetryInterval: time.Millisecond,
			RetryTimeout:  5 * time.Millisecond,
		},
		nodeName:      "node-load-failure",
		localSubStore: &localSubStore{},
		fedSubStore: &fedSubStore{
			TrieDB:     mem.NewStore(),
			sharedSent: map[string]uint64{},
		},
		serfEventCh: make(chan serf.Event, 1),
		sessionMgr: &sessionMgr{
			sessions: map[string]*session{},
		},
		serf:  mockSerf,
		peers: map[string]*peer{},
		exit:  make(chan struct{}),
		wg:    &sync.WaitGroup{},
	}

	err = f.Load(service)

	a.Error(err)
	a.Contains(err.Error(), "retry timeout")
	rebound, listenErr := net.Listen("tcp", addr)
	a.NoError(listenErr)
	if rebound != nil {
		a.NoError(rebound.Close())
	}
}

func TestFederationUnloadStopsGRPCListenerAndPeersIdempotently(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	a.NoError(err)
	addr := listener.Addr().String()
	srv := grpc.NewServer()

	mockSerf := NewMockiSerf(ctrl)
	mockSerf.EXPECT().Leave().Return(nil).Times(2)
	mockSerf.EXPECT().Shutdown().Return(nil).Times(2)

	peerToStop := &peer{exit: make(chan struct{})}
	f := &Federation{
		serf:     mockSerf,
		exit:     make(chan struct{}),
		peers:    map[string]*peer{"peer-a": peerToStop},
		memberMu: sync.Mutex{},
	}
	f.setGRPCServer(srv, listener)

	done := make(chan struct{})
	go func() {
		f.eventHandler()
		close(done)
	}()

	a.NoError(f.Unload())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("eventHandler did not exit")
	}
	select {
	case <-peerToStop.exit:
	default:
		t.Fatal("peer exit channel was not closed")
	}

	rebound, listenErr := net.Listen("tcp", addr)
	a.NoError(listenErr)
	if rebound != nil {
		a.NoError(rebound.Close())
	}

	a.NoError(f.Unload())
}

func TestFederationUnloadStillShutsDownSerfWhenLeaveFails(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSerf := NewMockiSerf(ctrl)
	leaveErr := errors.New("leave failed")
	mockSerf.EXPECT().Leave().Return(leaveErr)
	mockSerf.EXPECT().Shutdown().Return(nil)

	f := &Federation{
		serf: mockSerf,
		exit: make(chan struct{}),
	}

	err := f.Unload()

	a.ErrorIs(err, leaveErr)
}

func TestFederationUnloadStopsPeersWithoutEventHandler(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSerf := NewMockiSerf(ctrl)
	mockSerf.EXPECT().Leave().Return(nil)
	mockSerf.EXPECT().Shutdown().Return(nil)

	peerToStop := &peer{exit: make(chan struct{})}
	f := &Federation{
		serf:  mockSerf,
		exit:  make(chan struct{}),
		peers: map[string]*peer{"peer-a": peerToStop},
	}

	a.NoError(f.Unload())

	select {
	case <-peerToStop.exit:
	default:
		t.Fatal("peer exit channel was not closed")
	}
}

func TestFederation_nodeUpdateRestartsPeerWhenFederationAddressChanges(t *testing.T) {
	a := assert.New(t)
	oldServePeerEventStream := servePeerEventStream
	started := make(chan *peer, 2)
	servePeerEventStream = func(p *peer) {
		started <- p
	}
	t.Cleanup(func() {
		servePeerEventStream = oldServePeerEventStream
	})

	f := &Federation{
		nodeName: "local",
		peers:    make(map[string]*peer),
	}
	oldMember := serf.Member{
		Name: "node2",
		Tags: map[string]string{"fed_addr": "127.0.0.1:8901"},
	}
	f.nodeJoin(serf.MemberEvent{Members: []serf.Member{oldMember}})

	var first *peer
	select {
	case first = <-started:
	case <-time.After(time.Second):
		t.Fatal("nodeJoin did not start a peer stream")
	}

	f.nodeUpdate(serf.MemberEvent{Members: []serf.Member{oldMember}})
	select {
	case restarted := <-started:
		t.Fatalf("nodeUpdate restarted unchanged peer: %v", restarted)
	default:
	}

	updatedMember := serf.Member{
		Name: "node2",
		Tags: map[string]string{"fed_addr": "127.0.0.1:8903"},
	}
	f.nodeUpdate(serf.MemberEvent{Members: []serf.Member{updatedMember}})

	var second *peer
	select {
	case second = <-started:
	case <-time.After(time.Second):
		t.Fatal("nodeUpdate did not restart peer stream after fed_addr changed")
	}

	select {
	case <-first.exit:
	default:
		t.Fatal("old peer exit channel was not closed")
	}
	a.NotSame(first, second)
	a.Equal(first.sessionID, second.sessionID)
	a.Equal(first.queue, second.queue)
	a.Equal("127.0.0.1:8903", federationAddress(second.member))
}

func TestFederation_ListMembers(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p, _ := New(testConfig)
	f := p.(*Federation)

	mockSerf := NewMockiSerf(ctrl)
	f.serf = mockSerf
	mockSerf.EXPECT().Members().Return([]serf.Member{
		{
			Name:   "node1",
			Addr:   net.ParseIP("127.0.0.1"),
			Port:   1234,
			Tags:   map[string]string{"k": "v"},
			Status: serf.StatusAlive,
		}, {
			Name:   "node2",
			Addr:   net.ParseIP("127.0.0.2"),
			Port:   1234,
			Tags:   map[string]string{"k": "v"},
			Status: serf.StatusAlive,
		},
	})
	resp, err := f.ListMembers(context.Background(), nil)
	a.NoError(err)
	a.Equal([]*Member{
		{
			Name:   "node1",
			Addr:   "127.0.0.1:1234",
			Tags:   map[string]string{"k": "v"},
			Status: Status_STATUS_ALIVE,
		}, {
			Name:   "node2",
			Addr:   "127.0.0.2:1234",
			Tags:   map[string]string{"k": "v"},
			Status: Status_STATUS_ALIVE,
		},
	}, resp.Members)
}

func TestFederation_Join(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p, _ := New(testConfig)
	f := p.(*Federation)

	mockSerf := NewMockiSerf(ctrl)
	f.serf = mockSerf
	mockSerf.EXPECT().Join([]string{"127.0.0.1:" + DefaultGossipPort, "127.0.0.2:1234"}, true).Return(2, nil)
	_, err := f.Join(context.Background(), &JoinRequest{
		Hosts: []string{
			"127.0.0.1",
			"127.0.0.2:1234",
		},
	})
	a.NoError(err)
}

func TestFederation_Leave(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p, _ := New(testConfig)
	f := p.(*Federation)
	mockSerf := NewMockiSerf(ctrl)
	f.serf = mockSerf
	mockSerf.EXPECT().Leave()
	_, err := f.Leave(context.Background(), nil)
	a.NoError(err)
}

func TestFederation_ForceLeave(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p, _ := New(testConfig)
	f := p.(*Federation)
	mockSerf := NewMockiSerf(ctrl)
	f.serf = mockSerf
	mockSerf.EXPECT().RemoveFailedNode("node1")
	_, err := f.ForceLeave(context.Background(), &ForceLeaveRequest{
		NodeName: "node1",
	})
	a.NoError(err)
}

func mockMetaContext(nodeName string) context.Context {
	return mockMetaContextWithSecret(nodeName, testPeerSecret)
}

func mockMetaContextWithSecret(nodeName string, peerSecret string) context.Context {
	ctx := context.Background()
	values := map[string]string{metadataNodeNameKey: nodeName}
	if peerSecret != "" {
		values[metadataPeerSecretKey] = peerSecret
	}
	md := metadata.New(values)
	return metadata.NewIncomingContext(ctx, md)
}

type blockingEventStreamServer struct {
	ctx    context.Context
	recvCh chan *Event
	sendCh chan *Ack
}

func newBlockingEventStreamServer(ctx context.Context) *blockingEventStreamServer {
	return &blockingEventStreamServer{
		ctx:    ctx,
		recvCh: make(chan *Event, 1),
		sendCh: make(chan *Ack, 1),
	}
}

func (s *blockingEventStreamServer) SetHeader(metadata.MD) error  { return nil }
func (s *blockingEventStreamServer) SendHeader(metadata.MD) error { return nil }
func (s *blockingEventStreamServer) SetTrailer(metadata.MD)       {}
func (s *blockingEventStreamServer) Context() context.Context     { return s.ctx }
func (s *blockingEventStreamServer) SendMsg(interface{}) error    { return nil }
func (s *blockingEventStreamServer) RecvMsg(interface{}) error    { return nil }

func (s *blockingEventStreamServer) Send(ack *Ack) error {
	s.sendCh <- ack
	return nil
}

func (s *blockingEventStreamServer) Recv() (*Event, error) {
	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case event := <-s.recvCh:
		return event, nil
	}
}

var _ grpc.ServerStream = (*blockingEventStreamServer)(nil)
var _ Federation_EventStreamServer = (*blockingEventStreamServer)(nil)

func TestFederation_Hello(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	f := newTestFederation()
	clientNodeName := "node1"
	f.peers[clientNodeName] = &peer{}
	clientSid := "session_id"
	f.fedSubStore.Subscribe(clientNodeName, &gmqtt.Subscription{
		TopicFilter: "topicA",
	})
	ctx := mockMetaContext(clientNodeName)
	resp, err := f.Hello(ctx, &ClientHello{
		SessionId: clientSid,
	})
	a.NoError(err)
	// cleanStart == true on first time
	a.True(resp.CleanStart)
	a.Zero(resp.NextEventId)
	// clean subscription tree if cleanStart == true
	a.EqualValues(0, f.fedSubStore.GetStats().SubscriptionsCurrent)

	f.fedSubStore.Subscribe(clientNodeName, &gmqtt.Subscription{
		TopicFilter: "topicA",
	})
	resp, err = f.Hello(ctx, &ClientHello{
		SessionId: clientSid,
	})
	a.NoError(err)
	// cleanStart == true on second time
	a.False(resp.CleanStart)
	a.Zero(resp.NextEventId)
	a.EqualValues(1, f.fedSubStore.GetStats().SubscriptionsCurrent)
	a.Equal(clientNodeName, f.sessionMgr.sessions[clientNodeName].nodeName)
	a.Equal(clientSid, f.sessionMgr.sessions[clientNodeName].id)
	a.EqualValues(f.sessionMgr.sessions[clientNodeName].nextEventID, 0)

	// test next eventID
	f.sessionMgr.sessions[clientNodeName].nextEventID = 2

	resp, err = f.Hello(ctx, &ClientHello{
		SessionId: clientSid,
	})
	a.NoError(err)

	a.EqualValues(2, resp.NextEventId)
}

func TestFederationPeerAuthRejectsMissingSecret(t *testing.T) {
	a := assert.New(t)
	f := newTestFederation()
	clientNodeName := "node1"
	f.peers[clientNodeName] = &peer{}

	ctx := mockMetaContextWithSecret(clientNodeName, "")
	_, err := f.Hello(ctx, &ClientHello{SessionId: "session_id"})
	a.Error(err)
	a.Equal(codes.Unauthenticated, status.Code(err))

	f.sessionMgr.sessions[clientNodeName] = &session{
		id:         "session_id",
		nodeName:   clientNodeName,
		seenEvents: newLRUCache(defaultSessionSeenEventsCacheSize),
		close:      make(chan struct{}),
	}
	stream := newBlockingEventStreamServer(ctx)
	err = f.EventStream(stream)
	a.Error(err)
	a.Equal(codes.Unauthenticated, status.Code(err))
}

func TestFederationPeerAuthRejectsWrongSecret(t *testing.T) {
	a := assert.New(t)
	f := newTestFederation()
	clientNodeName := "node1"
	f.peers[clientNodeName] = &peer{}

	ctx := mockMetaContextWithSecret(clientNodeName, "wrong-secret")
	_, err := f.Hello(ctx, &ClientHello{SessionId: "session_id"})
	a.Error(err)
	a.Equal(codes.Unauthenticated, status.Code(err))
}

func TestFederationPeerAuthRejectsUnconfiguredSecret(t *testing.T) {
	a := assert.New(t)
	clientNodeName := "node1"
	f := &Federation{
		config: &Config{},
		peers: map[string]*peer{
			clientNodeName: {},
		},
	}

	_, err := f.Hello(mockMetaContext(clientNodeName), &ClientHello{SessionId: "session_id"})
	a.Error(err)
	a.Equal(codes.Unauthenticated, status.Code(err))
}

func TestFederationPeerAuthAllowsHelloAndEventStreamWithCorrectSecret(t *testing.T) {
	a := assert.New(t)
	f := newTestFederation()
	clientNodeName := "node1"
	f.peers[clientNodeName] = &peer{}

	ctx, cancel := context.WithCancel(mockMetaContext(clientNodeName))
	resp, err := f.Hello(ctx, &ClientHello{SessionId: "session_id"})
	a.NoError(err)
	a.True(resp.CleanStart)

	stream := newBlockingEventStreamServer(ctx)
	done := make(chan error, 1)
	go func() {
		done <- f.EventStream(stream)
	}()

	stream.recvCh <- &Event{
		Id: 0,
		Event: &Event_Subscribe{
			Subscribe: &Subscribe{TopicFilter: "topic/authenticated"},
		},
	}
	select {
	case ack := <-stream.sendCh:
		a.EqualValues(0, ack.EventId)
	case <-time.After(time.Second):
		t.Fatal("EventStream did not acknowledge authenticated peer event")
	}
	cancel()
	select {
	case err := <-done:
		a.Error(err)
	case <-time.After(time.Second):
		t.Fatal("EventStream did not return after context cancellation")
	}
}

func TestSessionMgrAddClosesReplacedSession(t *testing.T) {
	a := assert.New(t)
	nodeName := "node-reconnect"
	oldSession := &session{
		id:          "old-session",
		nodeName:    nodeName,
		seenEvents:  newLRUCache(defaultSessionSeenEventsCacheSize),
		nextEventID: 7,
		close:       make(chan struct{}),
	}
	mgr := &sessionMgr{
		sessions: map[string]*session{nodeName: oldSession},
	}

	cleanStart, nextID := mgr.add(nodeName, "new-session")

	a.True(cleanStart)
	a.Zero(nextID)
	select {
	case <-oldSession.close:
	default:
		t.Fatal("old session close channel was not closed")
	}
	a.Equal("new-session", mgr.sessions[nodeName].id)
	a.Zero(mgr.sessions[nodeName].nextEventID)
}

func TestFederationEventStreamIgnoresLateRecvAfterSessionClose(t *testing.T) {
	a := assert.New(t)
	log = zap.NewNop()

	nodeName := "node-close"
	sess := &session{
		id:          "session-close",
		nodeName:    nodeName,
		seenEvents:  newLRUCache(defaultSessionSeenEventsCacheSize),
		nextEventID: 0,
		close:       make(chan struct{}),
	}
	f := &Federation{
		config: &Config{PeerSecret: testPeerSecret},
		sessionMgr: &sessionMgr{
			sessions: map[string]*session{nodeName: sess},
		},
		fedSubStore: &fedSubStore{
			TrieDB:     mem.NewStore(),
			sharedSent: map[string]uint64{},
		},
	}

	ctx, cancel := context.WithCancel(mockMetaContext(nodeName))
	defer cancel()
	stream := newBlockingEventStreamServer(ctx)
	done := make(chan error, 1)
	go func() {
		done <- f.EventStream(stream)
	}()

	close(sess.close)
	select {
	case err := <-done:
		a.Error(err)
		a.Contains(err.Error(), "session of node [node-close] has been closed")
	case <-time.After(time.Second):
		t.Fatal("EventStream did not return after session close")
	}

	stream.recvCh <- &Event{
		Id: 7,
		Event: &Event_Subscribe{
			Subscribe: &Subscribe{TopicFilter: "topic/late"},
		},
	}
	select {
	case ack := <-stream.sendCh:
		t.Fatalf("late event was acked after EventStream returned: %+v", ack)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
}
