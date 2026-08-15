// 文件用途：覆盖下行消息模块 bus 行为的 Go 测试。
// 核心逻辑：验证总线发布订阅、处理器编解码和 MQTT 发布调用的下行契约，主要围绕 func TestBusPublishSubscribeRoutes、func TestBusCloseClosesAllSubscriptions 等声明展开。
// 关键注意事项：测试需保持消息类型、订阅关闭和发布错误路径的断言明确。
// 重构建议：后续可补充更多设备配置和编码失败场景，提升下行链路回归信心。

package downlink

import "testing"

func TestBusPublishSubscribeRoutes(t *testing.T) {
	bus := NewBus(2)
	t.Cleanup(bus.Close)

	command := &Message{DeviceID: "dev-1", Type: MessageTypeCommand, Data: []byte(`{"cmd":"reset"}`)}
	attributeSet := &Message{DeviceID: "dev-1", Type: MessageTypeAttributeSet, Data: []byte(`{"mode":"auto"}`)}
	attributeGet := &Message{DeviceID: "dev-1", Type: MessageTypeAttributeGet, Data: []byte(`{"key":"mode"}`)}
	telemetry := &Message{DeviceID: "dev-1", Type: MessageTypeTelemetry, Data: []byte(`{"interval":30}`)}

	bus.PublishCommand(command)
	bus.PublishAttributeSet(attributeSet)
	bus.PublishAttributeGet(attributeGet)
	bus.PublishTelemetry(telemetry)

	if got := <-bus.SubscribeCommand(); got != command {
		t.Fatalf("command route mismatch: got %#v want %#v", got, command)
	}
	if got := <-bus.SubscribeAttributeSet(); got != attributeSet {
		t.Fatalf("attribute set route mismatch: got %#v want %#v", got, attributeSet)
	}
	if got := <-bus.SubscribeAttributeGet(); got != attributeGet {
		t.Fatalf("attribute get route mismatch: got %#v want %#v", got, attributeGet)
	}
	if got := <-bus.SubscribeTelemetry(); got != telemetry {
		t.Fatalf("telemetry route mismatch: got %#v want %#v", got, telemetry)
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
