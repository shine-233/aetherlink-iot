// 文件用途：维护 server\server_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/persistence/queue"
	"github.com/DrmagicE/gmqtt/persistence/subscription/mem"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

type testNamedPlugin struct {
	name string
}

func (p testNamedPlugin) Load(Server) error        { return nil }
func (p testNamedPlugin) Unload() error            { return nil }
func (p testNamedPlugin) HookWrapper() HookWrapper { return HookWrapper{} }
func (p testNamedPlugin) Name() string             { return p.name }

type testHookPlugin struct {
	name  string
	hooks HookWrapper
}

func (p testHookPlugin) Load(Server) error        { return nil }
func (p testHookPlugin) Unload() error            { return nil }
func (p testHookPlugin) HookWrapper() HookWrapper { return p.hooks }
func (p testHookPlugin) Name() string             { return p.name }

func TestServerInitPluginHooksRejectsDuplicatePluginNames(t *testing.T) {
	originalPlugins := plugins
	plugins = make(map[string]NewPlugin)
	t.Cleanup(func() {
		plugins = originalPlugins
	})

	newDuplicateNamePlugin := func(config.Config) (Plugin, error) {
		return testNamedPlugin{name: "duplicate-name"}, nil
	}
	if err := RegisterPlugin("primary", newDuplicateNamePlugin); err != nil {
		t.Fatalf("register primary plugin: %v", err)
	}
	if err := RegisterPlugin("secondary", newDuplicateNamePlugin); err != nil {
		t.Fatalf("register secondary plugin: %v", err)
	}

	srv := &server{
		config: config.Config{
			PluginOrder: []string{"primary", "secondary"},
		},
	}

	err := srv.initPluginHooks()
	if err == nil || !strings.Contains(err.Error(), "duplicated plugin aliases") {
		t.Fatalf("initPluginHooks error = %v, want duplicate alias error", err)
	}
}

func TestServerInitPluginHooksRejectsUnknownPlugin(t *testing.T) {
	originalPlugins := plugins
	plugins = make(map[string]NewPlugin)
	t.Cleanup(func() {
		plugins = originalPlugins
	})

	srv := &server{
		config: config.Config{
			PluginOrder: []string{"missing"},
		},
	}

	err := srv.initPluginHooks()
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("initPluginHooks error = %v, want missing plugin error", err)
	}
}

func TestServerInitPluginHooksComposesOnReAuthWrapper(t *testing.T) {
	originalPlugins := plugins
	plugins = make(map[string]NewPlugin)
	t.Cleanup(func() {
		plugins = originalPlugins
	})

	wrapperCalled := false
	if err := RegisterPlugin("reauth", func(config.Config) (Plugin, error) {
		return testHookPlugin{
			name: "reauth",
			hooks: HookWrapper{
				OnReAuthWrapper: func(next OnReAuth) OnReAuth {
					return func(ctx context.Context, client Client, auth *packets.Auth) (*AuthResponse, error) {
						wrapperCalled = true
						resp, err := next(ctx, client, auth)
						if resp == nil {
							t.Fatal("next OnReAuth returned nil response")
						}
						resp.AuthData = []byte("wrapped")
						return resp, err
					}
				},
			},
		}, nil
	}); err != nil {
		t.Fatalf("register reauth plugin: %v", err)
	}

	srv := &server{
		config: config.Config{
			PluginOrder: []string{"reauth"},
		},
	}

	if err := srv.initPluginHooks(); err != nil {
		t.Fatalf("initPluginHooks: %v", err)
	}
	if srv.hooks.OnReAuth == nil {
		t.Fatal("OnReAuth hook was not composed")
	}

	resp, err := srv.hooks.OnReAuth(context.Background(), nil, &packets.Auth{})
	if err != nil {
		t.Fatalf("OnReAuth error: %v", err)
	}
	if !wrapperCalled {
		t.Fatal("OnReAuth wrapper was not called")
	}
	if string(resp.AuthData) != "wrapped" {
		t.Fatalf("AuthData = %q, want wrapped", string(resp.AuthData))
	}
}

func TestServerInitPluginHooksComposesOnBasicAuthWrapper(t *testing.T) {
	originalPlugins := plugins
	plugins = make(map[string]NewPlugin)
	t.Cleanup(func() {
		plugins = originalPlugins
	})

	wrapperCalled := false
	if err := RegisterPlugin("basic-auth", func(config.Config) (Plugin, error) {
		return testHookPlugin{
			name: "basic-auth",
			hooks: HookWrapper{
				OnBasicAuthWrapper: func(next OnBasicAuth) OnBasicAuth {
					return func(ctx context.Context, client Client, req *ConnectRequest) error {
						wrapperCalled = true
						req.Options.MaxInflight = 7
						return next(ctx, client, req)
					}
				},
			},
		}, nil
	}); err != nil {
		t.Fatalf("register basic-auth plugin: %v", err)
	}

	srv := &server{
		config: config.Config{
			PluginOrder: []string{"basic-auth"},
		},
	}

	if err := srv.initPluginHooks(); err != nil {
		t.Fatalf("initPluginHooks: %v", err)
	}
	if srv.hooks.OnBasicAuth == nil {
		t.Fatal("OnBasicAuth hook was not composed")
	}

	req := &ConnectRequest{Options: &AuthOptions{}}
	if err := srv.hooks.OnBasicAuth(context.Background(), nil, req); err != nil {
		t.Fatalf("OnBasicAuth error: %v", err)
	}
	if !wrapperCalled {
		t.Fatal("OnBasicAuth wrapper was not called")
	}
	if req.Options.MaxInflight != 7 {
		t.Fatalf("MaxInflight = %d, want wrapper mutation", req.Options.MaxInflight)
	}
}

func TestServerStopUnloadsPluginsWhenClientDrainContextTimesOut(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plg := NewMockPlugin(ctrl)
	plg.EXPECT().Name().Return("lifecycle").AnyTimes()
	plg.EXPECT().Unload().Return(nil).Times(1)

	clientClosed := make(chan struct{})
	srv := defaultServer()
	srv.plugins = []Plugin{plg}
	srv.clients["stuck-client"] = &client{closed: clientClosed}

	onStopCalled := false
	srv.hooks.OnStop = func(context.Context) {
		onStopCalled = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := srv.Stop(ctx)
	close(clientClosed)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Stop error = %v, want context canceled", err)
	}
	if !onStopCalled {
		t.Fatal("OnStop was not called after client drain context timeout")
	}
}

func TestServerStopTreatsNilContextAsBackground(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plg := NewMockPlugin(ctrl)
	plg.EXPECT().Name().Return("lifecycle").AnyTimes()
	plg.EXPECT().Unload().Return(nil).Times(1)

	srv := defaultServer()
	srv.plugins = []Plugin{plg}

	onStopCalled := false
	srv.hooks.OnStop = func(context.Context) {
		onStopCalled = true
	}

	if err := srv.Stop(nil); err != nil {
		t.Fatalf("Stop(nil) returned error: %v", err)
	}
	if !onStopCalled {
		t.Fatal("OnStop was not called")
	}
	select {
	case <-srv.exitedChan:
	default:
		t.Fatal("Stop(nil) did not close exitedChan")
	}
}

type testDeliverMsg struct {
	srv *server
}

func newTestDeliverMsg(ctrl *gomock.Controller, subscriber string) *testDeliverMsg {
	sub := mem.NewStore()
	srv := &server{
		subscriptionsDB: sub,
		queueStore:      make(map[string]queue.Store),
		config:          config.DefaultConfig(),
		statsManager:    newStatsManager(sub),
	}
	mockQueue := queue.NewMockStore(ctrl)
	srv.queueStore[subscriber] = mockQueue
	return &testDeliverMsg{
		srv: srv,
	}
}

func TestServer_deliverMessage(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	subscriber := "subCli"
	ts := newTestDeliverMsg(ctrl, subscriber)
	srcCli := "srcCli"
	msg := &gmqtt.Message{
		Topic:   "/abc",
		Payload: []byte("abc"),
		QoS:     2,
	}
	srv := ts.srv
	srv.subscriptionsDB.Subscribe(subscriber, &gmqtt.Subscription{
		ShareName:   "",
		TopicFilter: "/abc",
		QoS:         1,
	}, &gmqtt.Subscription{
		ShareName:   "",
		TopicFilter: "/+",
		QoS:         2,
	})

	mockQueue := srv.queueStore[subscriber].(*queue.MockStore)
	// test only once
	srv.config.MQTT.DeliveryMode = OnlyOnce
	mockQueue.EXPECT().Add(gomock.Any()).Do(func(elem *queue.Elem) {
		a.EqualValues(elem.MessageWithID.(*queue.Publish).QoS, 2)
	})

	a.True(srv.deliverMessage(srcCli, msg, defaultIterateOptions(msg.Topic)))

	// test overlap
	srv.config.MQTT.DeliveryMode = Overlap
	qos := map[byte]int{
		packets.Qos1: 0,
		packets.Qos2: 0,
	}
	mockQueue.EXPECT().Add(gomock.Any()).Do(func(elem *queue.Elem) {
		_, ok := qos[elem.MessageWithID.(*queue.Publish).QoS]
		a.True(ok)
		qos[elem.MessageWithID.(*queue.Publish).QoS]++
	}).Times(2)

	a.True(srv.deliverMessage(srcCli, msg, defaultIterateOptions(msg.Topic)))

	a.Equal(1, qos[packets.Qos1])
	a.Equal(1, qos[packets.Qos2])

	msg = &gmqtt.Message{
		Topic: "abcd",
	}
	a.False(srv.deliverMessage(srcCli, msg, defaultIterateOptions(msg.Topic)))

}

func TestServer_deliverMessage_sharedSubscription(t *testing.T) {
	a := assert.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	subscriber := "subCli"
	ts := newTestDeliverMsg(ctrl, subscriber)
	srcCli := "srcCli"
	msg := &gmqtt.Message{
		Topic:   "/abc",
		Payload: []byte("abc"),
		QoS:     2,
	}
	srv := ts.srv
	// add 2 shared and 2 non-shared subscription which both match the message topic: /abc
	srv.subscriptionsDB.Subscribe(subscriber, &gmqtt.Subscription{
		ShareName:   "abc",
		TopicFilter: "/abc",
		QoS:         1,
	}, &gmqtt.Subscription{
		ShareName:   "abc",
		TopicFilter: "/+",
		QoS:         2,
	}, &gmqtt.Subscription{
		TopicFilter: "#",
		QoS:         2,
	}, &gmqtt.Subscription{
		TopicFilter: "/abc",
		QoS:         1,
	})

	mockQueue := srv.queueStore[subscriber].(*queue.MockStore)
	// test only once
	qos := map[byte]int{
		packets.Qos1: 0,
		packets.Qos2: 0,
	}
	srv.config.MQTT.DeliveryMode = OnlyOnce
	mockQueue.EXPECT().Add(gomock.Any()).Do(func(elem *queue.Elem) {
		_, ok := qos[elem.MessageWithID.(*queue.Publish).QoS]
		a.True(ok)
		qos[elem.MessageWithID.(*queue.Publish).QoS]++

	}).Times(3)

	a.True(srv.deliverMessage(srcCli, msg, defaultIterateOptions(msg.Topic)))
	a.Equal(1, qos[packets.Qos1])
	a.Equal(2, qos[packets.Qos2])

	// test overlap
	srv.config.MQTT.DeliveryMode = Overlap
	qos = map[byte]int{
		packets.Qos1: 0,
		packets.Qos2: 0,
	}
	mockQueue.EXPECT().Add(gomock.Any()).Do(func(elem *queue.Elem) {
		_, ok := qos[elem.MessageWithID.(*queue.Publish).QoS]
		a.True(ok)
		qos[elem.MessageWithID.(*queue.Publish).QoS]++
	}).Times(4)
	a.True(srv.deliverMessage(srcCli, msg, defaultIterateOptions(msg.Topic)))
	a.Equal(2, qos[packets.Qos1])
	a.Equal(2, qos[packets.Qos2])

}

func TestDeliverHandlerSelectSharedSubscriberUsesTopicHashStrategy(t *testing.T) {
	a := assert.New(t)
	d := &deliverHandler{
		msg:      &gmqtt.Message{Topic: "/abc"},
		strategy: SharedSubBalanceTopicHash,
	}
	subscribers := []struct {
		clientID string
		sub      *gmqtt.Subscription
	}{
		{clientID: "first", sub: &gmqtt.Subscription{ShareName: "abc", TopicFilter: "/abc"}},
		{clientID: "second", sub: &gmqtt.Subscription{ShareName: "abc", TopicFilter: "/abc"}},
		{clientID: "third", sub: &gmqtt.Subscription{ShareName: "abc", TopicFilter: "/abc"}},
	}

	selected := d.selectSharedSubscriber(subscribers)

	a.Equal("third", selected.clientID)
}
