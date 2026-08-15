// 文件用途：维护 plugin\federation\peer_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package federation

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/subscription/mem"
	"github.com/DrmagicE/gmqtt/retained"
	"github.com/DrmagicE/gmqtt/retained/trie"
)

func TestEventQueueDropsOldestWhenBufferIsFull(t *testing.T) {
	a := assert.New(t)
	q := newEventQueue()
	q.maxSize = 2

	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "first"}}})
	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "second"}}})
	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "third"}}})

	events := q.fetchEvents()
	if a.Len(events, 2) {
		a.Equal("second", events[0].GetSubscribe().TopicFilter)
		a.Equal("third", events[1].GetSubscribe().TopicFilter)
	}
}

func TestEventQueueSetReadPositionUsesNearestRetainedEvent(t *testing.T) {
	a := assert.New(t)
	q := newEventQueue()
	q.maxSize = 2

	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "first"}}})
	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "second"}}})
	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "third"}}})

	q.setReadPosition(0)
	a.EqualValues(1, q.nextRead.Value.(*Event).Id)
	q.setReadPosition(2)
	a.EqualValues(2, q.nextRead.Value.(*Event).Id)
	q.setReadPosition(3)
	a.Nil(q.nextRead)
}

func TestEventQueueAckAdvancesNextReadWhenAckRemovesIt(t *testing.T) {
	a := assert.New(t)
	q := newEventQueue()

	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "first"}}})
	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "second"}}})
	q.add(&Event{Event: &Event_Subscribe{Subscribe: &Subscribe{TopicFilter: "third"}}})

	q.nextRead = q.l.Front().Next()
	q.ack(1)

	if a.NotNil(q.nextRead) {
		a.EqualValues(2, q.nextRead.Value.(*Event).Id)
	}
	events := q.fetchEvents()
	if a.Len(events, 1) {
		a.Equal("third", events[0].GetSubscribe().TopicFilter)
	}
}

func TestPeerStopHandlesStreamingPeerWithoutConn(t *testing.T) {
	p := &peer{
		exit:  make(chan struct{}),
		state: peerStateStreaming,
		stream: &stream{
			close: make(chan struct{}),
		},
	}

	p.stop()

	assert.Equal(t, peerStateStopped, p.state)
	select {
	case <-p.exit:
	default:
		t.Fatal("peer exit channel was not closed")
	}
}

func TestPeerServeStreamRejectsMissingFederationAddress(t *testing.T) {
	a := assert.New(t)
	backoff := time.NewTimer(time.Hour)
	defer backoff.Stop()
	p := &peer{
		member: serf.Member{
			Name: "node-remote",
			Tags: map[string]string{},
		},
	}

	err := p.serveStream(0, backoff)

	if a.Error(err) {
		a.Contains(err.Error(), "empty fed_addr")
		a.Contains(err.Error(), "node-remote")
	}
}

func TestStreamSetErrorHandlesMissingOptionalFields(t *testing.T) {
	s := &stream{
		close: make(chan struct{}),
	}

	s.setError(nil)
	s.setError(nil)

	select {
	case <-s.close:
	default:
		t.Fatal("stream close channel was not closed")
	}
}

func TestPeer_initStream_CleanStart(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockQueue := NewMockqueue(ctrl)

	ls := &localSubStore{}
	ls.init(mem.NewStore())

	retained := trie.NewStore()
	p := &peer{
		fed: &Federation{
			localSubStore: ls,
			retainedStore: retained,
		},
		localName: "",
		member: serf.Member{
			Name: "node2",
		},
		exit:      nil,
		sessionID: "sessionID",
		queue:     mockQueue,
		stream:    nil,
	}
	ls.subscribe("c1", "topicA")
	ls.subscribe("c2", "topicB")

	m1 := &gmqtt.Message{
		Topic: "topicA",
	}
	m2 := &gmqtt.Message{
		Topic: "topicB",
	}
	retained.AddOrReplace(m1)
	retained.AddOrReplace(m2)

	client := NewMockFederationClient(ctrl)

	client.EXPECT().Hello(gomock.Any(), &ClientHello{
		SessionId: p.sessionID,
	}).Return(&ServerHello{
		CleanStart:  true,
		NextEventId: 0,
	}, nil)

	gomock.InOrder(
		mockQueue.EXPECT().clear(),
		mockQueue.EXPECT().setReadPosition(uint64(0)),
		mockQueue.EXPECT().open(),
	)

	// The order of the events is not significant and also is not grantee to be sorted in any way.
	// So we had to collect them into map.
	subEvents := make(map[string]string)
	msgEvents := make(map[string]string)

	expectedSubEvents := map[string]*Event{
		"topicA": {
			Event: &Event_Subscribe{
				Subscribe: &Subscribe{
					TopicFilter: "topicA",
				},
			},
		},
		"topicB": {
			Event: &Event_Subscribe{
				Subscribe: &Subscribe{
					TopicFilter: "topicB",
				},
			},
		},
	}
	expectedMsgEvents := map[string]*Event{
		"topicA": {
			Event: &Event_Message{
				Message: messageToEvent(m1),
			},
		},
		"topicB": {
			Event: &Event_Message{
				Message: messageToEvent(m2),
			},
		},
	}
	mockQueue.EXPECT().add(gomock.Any()).Do(func(event *Event) {
		switch event.Event.(type) {
		case *Event_Subscribe:
			sub := event.Event.(*Event_Subscribe)
			subEvents[sub.Subscribe.TopicFilter] = event.String()
		case *Event_Message:
			msg := event.Event.(*Event_Message)
			msgEvents[msg.Message.TopicName] = event.String()
		default:
			a.FailNow("unexpected event type: %s", reflect.TypeOf(event.Event))
		}
	}).Times(4)

	client.EXPECT().EventStream(gomock.Any())
	_, err := p.initStream(client, nil)

	a.NoError(err)
	for k, v := range msgEvents {
		a.Equal(expectedMsgEvents[k].String(), v)
	}
	for k, v := range subEvents {
		a.Equal(expectedSubEvents[k].String(), v)
	}

}

func TestPeer_initStream_CleanStartFalse(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockQueue := NewMockqueue(ctrl)

	ls := &localSubStore{}
	ls.init(mem.NewStore())

	rt := retained.NewMockStore(ctrl)
	p := &peer{
		fed: &Federation{
			localSubStore: ls,
			retainedStore: rt,
		},
		localName: "",
		member: serf.Member{
			Name: "node2",
		},
		exit:      nil,
		sessionID: "sessionID",
		queue:     mockQueue,
		stream:    nil,
	}

	client := NewMockFederationClient(ctrl)
	client.EXPECT().Hello(gomock.Any(), &ClientHello{
		SessionId: p.sessionID,
	}).Return(&ServerHello{
		CleanStart:  false,
		NextEventId: 10,
	}, nil)

	gomock.InOrder(
		mockQueue.EXPECT().setReadPosition(uint64(10)),
		mockQueue.EXPECT().open(),
	)

	client.EXPECT().EventStream(gomock.Any())

	_, err := p.initStream(client, nil)
	a.NoError(err)

}

func TestPeerInitStreamSendsPeerSecretMetadata(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockQueue := NewMockqueue(ctrl)
	p := &peer{
		fed: &Federation{
			config: &Config{PeerSecret: testPeerSecret},
			localSubStore: &localSubStore{
				localStore: mem.NewStore(),
				index:      map[string]map[string]struct{}{},
				topics:     map[string]uint64{},
			},
		},
		localName: "node-local",
		member: serf.Member{
			Name: "node-remote",
		},
		sessionID: "sessionID",
		queue:     mockQueue,
	}

	client := NewMockFederationClient(ctrl)
	client.EXPECT().Hello(gomock.Any(), &ClientHello{
		SessionId: p.sessionID,
	}).DoAndReturn(func(ctx context.Context, hello *ClientHello, opts ...grpc.CallOption) (*ServerHello, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		a.True(ok)
		a.Equal([]string{"node-local"}, md.Get(metadataNodeNameKey))
		a.Equal([]string{testPeerSecret}, md.Get(metadataPeerSecretKey))
		return &ServerHello{
			CleanStart:  false,
			NextEventId: 3,
		}, nil
	})
	gomock.InOrder(
		mockQueue.EXPECT().setReadPosition(uint64(3)),
		mockQueue.EXPECT().open(),
	)
	client.EXPECT().EventStream(gomock.Any()).DoAndReturn(func(ctx context.Context, opts ...grpc.CallOption) (Federation_EventStreamClient, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		a.True(ok)
		a.Equal([]string{"node-local"}, md.Get(metadataNodeNameKey))
		a.Equal([]string{testPeerSecret}, md.Get(metadataPeerSecretKey))
		return nil, nil
	})

	_, err := p.initStream(client, nil)
	a.NoError(err)
}
