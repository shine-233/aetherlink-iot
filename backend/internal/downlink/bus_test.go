// 文件用途：覆盖下行消息模块 bus 行为的 Go 测试。
// 核心逻辑：验证总线发布订阅、处理器编解码和 MQTT 发布调用的下行契约，主要围绕 func TestBusPublishSubscribeRoutes、func TestBusCloseClosesAllSubscriptions 等声明展开。
// 关键注意事项：测试需保持消息类型、订阅关闭和发布错误路径的断言明确。
// 重构建议：后续可补充更多设备配置和编码失败场景，提升下行链路回归信心。

package downlink

import (
	"context"
	"errors"
	"testing"
)

func TestBusPublishSubscribeRoutes(t *testing.T) {
	bus := NewBus(2)
	bus.mu.Lock()
	bus.running = true
	bus.ctx = context.Background()
	bus.mu.Unlock()
	t.Cleanup(bus.Close)

	command := &Message{DeviceID: "dev-1", DeviceNumber: "number-1", Type: MessageTypeCommand, Data: []byte(`{"cmd":"reset"}`)}
	attributeSet := &Message{DeviceID: "dev-1", DeviceNumber: "number-1", Type: MessageTypeAttributeSet, Data: []byte(`{"mode":"auto"}`)}
	attributeGet := &Message{DeviceID: "dev-1", DeviceNumber: "number-1", Type: MessageTypeAttributeGet, Data: []byte(`{"key":"mode"}`)}
	telemetry := &Message{DeviceID: "dev-1", DeviceNumber: "number-1", Type: MessageTypeTelemetry, Data: []byte(`{"interval":30}`)}

	for name, err := range map[string]error{
		"command":       bus.PublishCommand(command),
		"attribute set": bus.PublishAttributeSet(attributeSet),
		"attribute get": bus.PublishAttributeGet(attributeGet),
		"telemetry":     bus.PublishTelemetry(telemetry),
	} {
		if err != nil {
			t.Fatalf("%s admission failed: %v", name, err)
		}
	}
}

func TestBusRejectsPublishBeforeStartAndInvalidMessages(t *testing.T) {
	bus := NewBus(1)
	valid := &Message{DeviceID: "dev-1", DeviceNumber: "number-1", Type: MessageTypeCommand, Data: []byte(`{"cmd":"reset"}`)}
	if err := bus.PublishCommand(valid); !errors.Is(err, ErrBusNotStarted) {
		t.Fatalf("publish before start error = %v, want %v", err, ErrBusNotStarted)
	}

	ctx, cancel := context.WithCancel(context.Background())
	handler := NewHandler(&mockPublisher{}, &mockProcessor{}, newHandlerTestLogger())
	if err := bus.Start(ctx, handler); err != nil {
		t.Fatalf("start bus: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		bus.Close()
	})
	if err := bus.PublishCommand(&Message{DeviceID: "dev-1"}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("invalid publish error = %v, want %v", err, ErrInvalidMessage)
	}
}

func TestBusStartValidatesDependencies(t *testing.T) {
	bus := NewBus(1)
	if err := bus.Start(context.Background(), nil); !errors.Is(err, ErrBusUnavailable) {
		t.Fatalf("nil handler error = %v, want %v", err, ErrBusUnavailable)
	}
	invalid := NewBus(-1)
	handler := NewHandler(&mockPublisher{}, &mockProcessor{}, newHandlerTestLogger())
	if err := invalid.Start(context.Background(), handler); !errors.Is(err, ErrBusUnavailable) {
		t.Fatalf("invalid buffer start error = %v, want %v", err, ErrBusUnavailable)
	}
}

func TestBusCloseClosesAllSubscriptions(t *testing.T) {
	bus := NewBus(1)
	bus.Close()

	assertClosed := func(name string, ch <-chan *Message) {
		t.Helper()
		if msg, ok := <-ch; ok || msg != nil {
			t.Fatalf("%s channel should be closed, got msg=%#v ok=%v", name, msg, ok)
		}
	}

	assertClosed("command", bus.SubscribeCommand())
	assertClosed("attribute set", bus.SubscribeAttributeSet())
	assertClosed("attribute get", bus.SubscribeAttributeGet())
	assertClosed("telemetry", bus.SubscribeTelemetry())
}
