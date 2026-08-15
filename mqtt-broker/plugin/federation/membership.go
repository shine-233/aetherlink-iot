// 文件用途：维护 plugin\federation\membership.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package federation

import (
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
)

// iSerf is the interface for *serf.Serf.
// It is used for test.
type iSerf interface {
	Join(existing []string, ignoreOld bool) (int, error)
	RemoveFailedNode(node string) error
	Leave() error
	Members() []serf.Member
	Shutdown() error
}

var servePeerEventStream = func(p *peer) {
	p.serveEventStream()
}

func (f *Federation) startSerf(t *time.Timer) error {
	defer func() {
		t.Reset(f.config.RetryInterval)
	}()
	if _, err := f.serf.Join(f.config.RetryJoin, true); err != nil {
		return err
	}
	go f.eventHandler()
	return nil
}

func (f *Federation) eventHandler() {
	for {
		select {
		case evt := <-f.serfEventCh:
			f.handleSerfEvent(evt)
		case <-f.exit:
			f.stopAllPeers()
			return
		}
	}
}

func (f *Federation) handleSerfEvent(evt serf.Event) {
	switch evt.EventType() {
	case serf.EventMemberJoin:
		f.nodeJoin(evt.(serf.MemberEvent))
	case serf.EventMemberLeave, serf.EventMemberFailed, serf.EventMemberReap:
		f.nodeFail(evt.(serf.MemberEvent))
	case serf.EventMemberUpdate:
		f.nodeUpdate(evt.(serf.MemberEvent))
	case serf.EventUser, serf.EventQuery:
	default:
	}
}

func (f *Federation) stopAllPeers() {
	f.memberMu.Lock()
	defer f.memberMu.Unlock()
	for _, v := range f.peers {
		v.stop()
	}
}

func (f *Federation) newPeer(member serf.Member, sessionID string, eventQueue queue) *peer {
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	if eventQueue == nil {
		eventQueue = newEventQueue()
	}
	return &peer{
		fed:       f,
		member:    member,
		exit:      make(chan struct{}),
		sessionID: sessionID,
		queue:     eventQueue,
		localName: f.nodeName,
	}
}

func (f *Federation) startPeer(member serf.Member, sessionID string, eventQueue queue) *peer {
	p := f.newPeer(member, sessionID, eventQueue)
	f.peers[member.Name] = p
	go servePeerEventStream(p)
	return p
}

func (f *Federation) restartPeer(member serf.Member, p *peer) {
	f.startPeer(member, p.resumeState().sessionID, p.resumeState().queue)
}

func federationAddress(member serf.Member) string {
	if member.Tags == nil {
		return ""
	}
	return member.Tags["fed_addr"]
}

func (f *Federation) shouldIgnoreMember(member serf.Member) bool {
	return member.Name == f.nodeName
}

func (f *Federation) forEachTrackedMember(event serf.MemberEvent, fn func(member serf.Member)) {
	for _, member := range event.Members {
		if f.shouldIgnoreMember(member) {
			continue
		}
		fn(member)
	}
}

func (f *Federation) withTrackedMembers(event serf.MemberEvent, fn func(member serf.Member)) {
	f.memberMu.Lock()
	defer f.memberMu.Unlock()
	f.forEachTrackedMember(event, fn)
}

func (f *Federation) nodeJoin(member serf.MemberEvent) {
	f.withTrackedMembers(member, func(member serf.Member) {
		log.Info("member joined", zap.String("node_name", member.Name))
		if _, ok := f.peers[member.Name]; !ok {
			f.startPeer(member, "", nil)
		}
	})
}

func (f *Federation) nodeUpdate(member serf.MemberEvent) {
	f.withTrackedMembers(member, f.syncUpdatedPeer)
}

func (f *Federation) syncUpdatedPeer(member serf.Member) {
	p, ok := f.peers[member.Name]
	if !ok {
		log.Info("member updated, opening stream client", zap.String("node_name", member.Name))
		f.startPeer(member, "", nil)
		return
	}
	oldAddr := federationAddress(p.member)
	newAddr := federationAddress(member)
	if oldAddr == newAddr {
		return
	}
	restartPeerForAddressChange(f, member, p, oldAddr, newAddr)
}

func restartPeerForAddressChange(f *Federation, member serf.Member, p *peer, oldAddr string, newAddr string) {
	// A Serf tag update can move only the advertised federation gRPC address.
	// Preserve the session id and queue so the restarted stream can resume the
	// existing protocol sequence instead of forcing a clean-start snapshot.
	log.Info("member updated, restarting stream client",
		zap.String("node_name", member.Name),
		zap.String("old_fed_addr", oldAddr),
		zap.String("new_fed_addr", newAddr))
	p.stop()
	f.restartPeer(member, p)
}

func (f *Federation) nodeFail(member serf.MemberEvent) {
	f.withTrackedMembers(member, func(member serf.Member) {
		f.removeFailedPeer(member.Name)
	})
}

func (f *Federation) removeFailedPeer(nodeName string) {
	p, ok := f.peers[nodeName]
	if !ok {
		return
	}
	log.Error("node failed, close stream client", zap.String("node_name", nodeName))
	p.stop()
	delete(f.peers, nodeName)
	_ = f.fedSubStore.UnsubscribeAll(nodeName)
	f.sessionMgr.del(nodeName)
}
