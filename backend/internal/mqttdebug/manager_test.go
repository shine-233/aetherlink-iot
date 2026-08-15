package mqttdebug

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeTransport struct {
	mu        sync.Mutex
	connected bool
	hooks     TransportHooks
	handlers  map[string]func(IncomingMessage)
	published []IncomingMessage
}

type fakeUplinkSource struct {
	mu      sync.Mutex
	handler func(TrustedUplinkMessage)
}

func (source *fakeUplinkSource) Start(handler func(TrustedUplinkMessage)) (func(), error) {
	source.mu.Lock()
	source.handler = handler
	source.mu.Unlock()
	return func() {
		source.mu.Lock()
		source.handler = nil
		source.mu.Unlock()
	}, nil
}

func (*fakeUplinkSource) DroppedMessages() uint64 {
	return 0
}

func (source *fakeUplinkSource) emit(message TrustedUplinkMessage) {
	source.mu.Lock()
	handler := source.handler
	source.mu.Unlock()
	if handler != nil {
		handler(message)
	}
}

func (transport *fakeTransport) Connect(context.Context) error {
	transport.mu.Lock()
	transport.connected = true
	hook := transport.hooks.OnConnect
	transport.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

func (transport *fakeTransport) IsConnected() bool {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return transport.connected
}

func (transport *fakeTransport) Subscribe(topic string, _ byte, handler func(IncomingMessage)) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.handlers[topic] = handler
	return nil
}

func (transport *fakeTransport) Unsubscribe(topic string) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	delete(transport.handlers, topic)
	return nil
}

func (transport *fakeTransport) Publish(topic string, qos byte, payload []byte) error {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.published = append(transport.published, IncomingMessage{Topic: topic, QoS: qos, Payload: append([]byte(nil), payload...)})
	return nil
}

func (transport *fakeTransport) Close() {
	transport.mu.Lock()
	transport.connected = false
	transport.mu.Unlock()
}

func (transport *fakeTransport) emit(filter string, message IncomingMessage) {
	transport.mu.Lock()
	handler := transport.handlers[filter]
	transport.mu.Unlock()
	if handler != nil {
		handler(message)
	}
}

func TestManagerInterfaceIsolatesSharedTopicsAndClosesSession(t *testing.T) {
	var transport *fakeTransport
	uplinkSource := &fakeUplinkSource{}
	manager := NewManager(Config{
		Broker:       "tcp://broker.invalid:1883",
		SessionTTL:   time.Minute,
		UplinkSource: uplinkSource,
		TransportFactory: func(config TransportConfig) (Transport, error) {
			transport = &fakeTransport{hooks: config.Hooks, handlers: map[string]func(IncomingMessage){}}
			return transport, nil
		},
	}, nil)
	defer manager.Stop()

	ctx := context.Background()
	scope := Scope{TenantID: "tenant-1", UserID: "user-1", DeviceID: "device-1", DeviceNumber: "D-001"}
	opened, err := manager.Open(ctx, scope)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !opened.Connected || opened.SessionID == "" {
		t.Fatalf("unexpected open snapshot: %#v", opened)
	}

	if _, err := manager.Apply(ctx, scope, opened.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry", QoS: 0}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	uplinkSource.emit(TrustedUplinkMessage{
		DeviceID: "other",
		TenantID: scope.TenantID,
		Topic:    "devices/telemetry",
		Payload:  []byte(`{"values":{"t":99}}`),
	})
	uplinkSource.emit(TrustedUplinkMessage{
		DeviceID: scope.DeviceID,
		TenantID: scope.TenantID,
		Topic:    "devices/telemetry",
		Payload:  []byte(`{"t":20}`),
	})

	if _, err := manager.Apply(ctx, scope, opened.SessionID, Command{Action: ActionPublish, Topic: "devices/command/D-001/request-1", Payload: `{"value":1}`}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := manager.Apply(ctx, scope, opened.SessionID, Command{Action: ActionPublish, Topic: "devices/command/OTHER/request-1"}); !errors.Is(err, ErrTopicDenied) {
		t.Fatalf("cross-device publish err=%v, want ErrTopicDenied", err)
	}

	snapshot, err := manager.Snapshot(ctx, scope, opened.SessionID, 0, 200)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	inbound := 0
	for _, message := range snapshot.Messages {
		if message.Direction == "inbound" {
			inbound++
			if message.Payload == `{"values":{"t":99}}` {
				t.Fatal("other-device payload leaked into the debug session")
			}
		}
	}
	if inbound != 1 {
		t.Fatalf("inbound messages=%d, want exactly one scoped payload: %#v", inbound, snapshot.Messages)
	}

	otherScope := scope
	otherScope.UserID = "user-2"
	if _, err := manager.Snapshot(ctx, otherScope, opened.SessionID, 0, 20); !errors.Is(err, ErrSessionScope) {
		t.Fatalf("other user snapshot err=%v, want ErrSessionScope", err)
	}
	if err := manager.Close(ctx, scope, opened.SessionID); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := manager.Snapshot(ctx, scope, opened.SessionID, 0, 20); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("closed snapshot err=%v, want ErrSessionNotFound", err)
	}
}

func TestDebugClientIDFitsStrictMQTT31Limit(t *testing.T) {
	clientID := debugClientID("12345678-1234-1234-1234-123456789012")
	if len(clientID) > 23 {
		t.Fatalf("debug client id length = %d, want <= 23: %q", len(clientID), clientID)
	}
}

func TestSnapshotRateLimitDoesNotConsumeCommandResponseSnapshots(t *testing.T) {
	manager := NewManager(Config{
		Broker:                "tcp://broker.invalid:1883",
		SessionTTL:            time.Minute,
		MaxSnapshotsPerSecond: 2,
		TransportFactory: func(config TransportConfig) (Transport, error) {
			return &fakeTransport{hooks: config.Hooks, handlers: map[string]func(IncomingMessage){}}, nil
		},
	}, nil)
	defer manager.Stop()

	ctx := context.Background()
	scope := Scope{TenantID: "tenant-1", UserID: "user-1", DeviceID: "device-1", DeviceNumber: "D-001"}
	opened, err := manager.Open(ctx, scope)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for index := 0; index < 2; index++ {
		if _, err := manager.Snapshot(ctx, scope, opened.SessionID, 0, 20); err != nil {
			t.Fatalf("snapshot %d: %v", index+1, err)
		}
	}
	if _, err := manager.Snapshot(ctx, scope, opened.SessionID, 0, 20); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("third snapshot err=%v, want ErrRateLimited", err)
	}
	if _, err := manager.Apply(ctx, scope, opened.SessionID, Command{
		Action:  ActionPublish,
		Topic:   "devices/command/D-001/request-1",
		Payload: `{"value":1}`,
	}); err != nil {
		t.Fatalf("command response snapshot should bypass GET budget: %v", err)
	}
}

func TestWithManagerDefaultsAppliesLocalBoundsWithoutOverwritingExplicitValues(t *testing.T) {
	config := withManagerDefaults(Config{
		SessionTTL:            11 * time.Minute,
		MaxSessions:           7,
		PayloadMaxBytes:       1234,
		MaxSnapshotsPerSecond: 9,
	})
	if config.SessionTTL != 11*time.Minute || config.MaxSessions != 7 || config.PayloadMaxBytes != 1234 || config.MaxSnapshotsPerSecond != 9 {
		t.Fatalf("explicit manager defaults were overwritten: %+v", config)
	}
	if config.ConnectTimeout != 5*time.Second || config.ActionTimeout != 5*time.Second ||
		config.MaxSessionsPerUser != 3 || config.MaxSubscriptions != 8 || config.MessageCapacity != 200 ||
		config.PublishMaxBytes != 64*1024 || config.MaxInboundPerSecond != 100 ||
		config.MaxInboundBytesPerSecond != 256*1024 || config.OpenCooldown != 2*time.Second ||
		config.MaxCommandsPerSecond != 10 || config.MaxPublishBytesPerSecond != 256*1024 {
		t.Fatalf("unexpected manager defaults: %+v", config)
	}
}
