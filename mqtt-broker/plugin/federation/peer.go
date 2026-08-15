// 文件用途：维护 plugin\federation\peer.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package federation

import (
	"container/list"
	"sync"
	"time"

	"github.com/hashicorp/serf/serf"
	"google.golang.org/grpc"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription"
)

const (
	defaultEventQueueMaxBufferSize = 10000
	eventFetchBatchSize            = 100
	peerStreamOpenTimeout          = 10 * time.Second
)

type peerState byte

const (
	peerStateStopped peerState = iota + 1
	peerStateStreaming
)

// peerResumeState carries the local client-side state that must survive peer
// reconnects or address-driven restarts.
type peerResumeState struct {
	sessionID string
	queue     queue
}

// peer represents a remote node which act as the event stream server.
type peer struct {
	fed       *Federation
	localName string
	member    serf.Member
	exit      chan struct{}
	// Local session id advertised in Hello. Keep it stable across reconnects so
	// the remote peer can resume from NextEventId; regenerate it only for a new
	// membership identity.
	sessionID string
	queue     queue
	stopOnce  sync.Once
	// stateMu guards the following fields
	stateMu sync.Mutex
	state   peerState
	// client-side stream
	stream *stream
}

type stream struct {
	queue   queue
	conn    *grpc.ClientConn
	client  Federation_EventStreamClient
	close   chan struct{}
	errOnce sync.Once
	err     error
	wg      sync.WaitGroup
}

// interface for testing
type queue interface {
	clear()
	close()
	open()
	setReadPosition(id uint64)
	add(event *Event)
	fetchEvents() []*Event
	ack(id uint64)
}

// eventQueue stores the events that are ready to send.
type eventQueue struct {
	cond     *sync.Cond
	nextID   uint64
	l        *list.List
	nextRead *list.Element
	closed   bool
	maxSize  int
}

func newEventQueue() *eventQueue {
	return &eventQueue{
		cond:    sync.NewCond(&sync.Mutex{}),
		nextID:  0,
		l:       list.New(),
		closed:  false,
		maxSize: defaultEventQueueMaxBufferSize,
	}
}

func (e *eventQueue) clear() {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	e.nextID = 0
	e.l = list.New()
	e.nextRead = nil
	e.closed = false
}

func (e *eventQueue) close() {
	e.setClosed(true)
}

func (e *eventQueue) open() {
	e.setClosed(false)
}

func (e *eventQueue) setClosed(closed bool) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	e.closed = closed
	e.cond.Signal()
}

func (e *eventQueue) setReadPosition(id uint64) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	e.nextRead = e.findReadPositionLocked(id)
}

func (e *eventQueue) add(event *Event) {
	e.cond.L.Lock()
	defer func() {
		e.cond.L.Unlock()
		e.cond.Signal()
	}()
	event.Id = e.nextID
	e.nextID++
	e.dropOldestIfFull()
	elem := e.l.PushBack(event)
	if e.nextRead == nil {
		e.nextRead = elem
	}
}

func (e *eventQueue) dropOldestIfFull() {
	for e.maxSize > 0 && e.l.Len() >= e.maxSize {
		front := e.l.Front()
		if front == nil {
			return
		}
		if e.nextRead == front {
			e.nextRead = front.Next()
		}
		e.l.Remove(front)
	}
}

func (e *eventQueue) fetchEvents() []*Event {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	for (e.l.Len() == 0 || e.nextRead == nil) && !e.closed {
		e.cond.Wait()
	}
	if e.closed {
		return nil
	}
	return e.fetchBatchLocked(eventFetchBatchSize)
}

func (e *eventQueue) ack(id uint64) {
	e.cond.L.Lock()
	defer func() {
		e.cond.L.Unlock()
		e.cond.Signal()
	}()
	e.ackLocked(id)
}

func (e *eventQueue) findReadPositionLocked(id uint64) *list.Element {
	// If the requested event was dropped because the bounded queue overflowed,
	// resume at the nearest retained event. CleanStart is the protocol signal
	// for a full snapshot when the remote cannot resume safely.
	for elem := e.l.Front(); elem != nil; elem = elem.Next() {
		ev := elem.Value.(*Event)
		if ev.Id >= id {
			return elem
		}
	}
	return nil
}

func (e *eventQueue) fetchBatchLocked(limit int) []*Event {
	events := make([]*Event, 0, limit)
	elem := e.nextRead
	for i := 0; i < limit && elem != nil; i++ {
		events = append(events, elem.Value.(*Event))
		elem = elem.Next()
	}
	e.nextRead = elem
	return events
}

func (e *eventQueue) ackLocked(id uint64) {
	var next *list.Element
	for elem := e.l.Front(); elem != nil; elem = next {
		next = elem.Next()
		req := elem.Value.(*Event)
		if req.Id <= id {
			if e.nextRead == elem {
				e.nextRead = next
			}
			e.l.Remove(elem)
		}
		if req.Id == id {
			return
		}
	}
}

func (p *peer) stop() {
	if p.exit != nil {
		p.stopOnce.Do(func() {
			close(p.exit)
		})
	}
	p.stateMu.Lock()
	state := p.state
	activeStream := p.stream
	if state == peerStateStreaming && activeStream != nil {
		activeStream.closeConn()
	}
	p.state = peerStateStopped
	p.stateMu.Unlock()
	if state == peerStateStreaming && activeStream != nil {
		activeStream.wg.Wait()
	}
}

func (p *peer) resumeState() peerResumeState {
	return peerResumeState{
		sessionID: p.sessionID,
		queue:     p.queue,
	}
}

func (p *peer) applyServerHello(sh *ServerHello) {
	if sh.CleanStart {
		p.queue.clear()
		p.enqueueCleanStartSnapshot()
	}
	p.queue.setReadPosition(sh.NextEventId)
}

func (p *peer) enqueueCleanStartSnapshot() {
	// CleanStart is the federation protocol's full-state reconciliation path:
	// replay current subscriptions and retained messages before opening the
	// incremental event stream.
	p.fed.localSubStore.Lock()
	for k := range p.fed.localSubStore.topics {
		shareName, topicFilter := subscription.SplitTopic(k)
		p.queue.add(&Event{
			Event: &Event_Subscribe{Subscribe: &Subscribe{
				ShareName:   shareName,
				TopicFilter: topicFilter,
			}},
		})
	}
	p.fed.localSubStore.Unlock()

	p.fed.retainedStore.Iterate(func(message *gmqtt.Message) bool {
		// Retained messages are replayed as the sender's current full-state snapshot.
		// The federation protocol does not carry retained-message write timestamps,
		// so concurrent retained writes converge by stream delivery order.
		p.queue.add(&Event{
			Event: &Event_Message{
				Message: messageToEvent(message.Copy()),
			},
		})
		return true
	})
}
