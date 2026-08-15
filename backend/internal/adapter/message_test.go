// 文件用途：定义后端设备消息适配层的通用消息模型测试。
// 核心逻辑：验证消息类型的网关分类规则，避免上行路由对类型字符串的理解漂移。
// 关键注意事项：新增 MessageType 时需同步补充正反例，防止网关消息误入直连路径。
// 重构建议：可将消息分类规则整理为共享表驱动契约，让生产代码和测试用例共用同一分类清单。

package adapter

import "testing"

func TestMessageTypeIsGatewayMessage(t *testing.T) {
	gatewayTypes := []MessageType{
		MessageTypeGatewayTelemetry,
		MessageTypeGatewayAttribute,
		MessageTypeGatewayEvent,
	}

	for _, msgType := range gatewayTypes {
		if !msgType.IsGatewayMessage() {
			t.Fatalf("%s should be treated as a gateway message", msgType)
		}
	}

	directTypes := []MessageType{
		MessageTypeTelemetry,
		MessageTypeAttribute,
		MessageTypeEvent,
		MessageTypeCommand,
		MessageType("unknown"),
	}

	for _, msgType := range directTypes {
		if msgType.IsGatewayMessage() {
			t.Fatalf("%s should not be treated as a gateway message", msgType)
		}
	}
}

func TestNewDeviceMessageAndMetadata(t *testing.T) {
	payload := []byte(`{"temperature":22}`)
	msg := NewDeviceMessage(MessageTypeTelemetry, "device-001", "tenant-a", payload)

	if msg.Type != MessageTypeTelemetry {
		t.Fatalf("type mismatch: got %q", msg.Type)
	}
	if msg.DeviceID != "device-001" || msg.TenantID != "tenant-a" {
		t.Fatalf("device/tenant mismatch: %#v", msg)
	}
	if string(msg.Payload) != string(payload) {
		t.Fatalf("payload mismatch: got %s", string(msg.Payload))
	}
	if msg.Timestamp <= 0 {
		t.Fatalf("timestamp should be set")
	}
	if msg.Metadata == nil {
		t.Fatalf("metadata map should be initialized")
	}

	if returned := msg.SetMetadata("message_id", "msg-1"); returned != msg {
		t.Fatalf("SetMetadata should return the same message for chaining")
	}
	if value, ok := msg.GetMetadata("message_id"); !ok || value != "msg-1" {
		t.Fatalf("metadata mismatch: got value=%v ok=%v", value, ok)
	}
	if value, ok := msg.GetMetadata("missing"); ok || value != nil {
		t.Fatalf("missing metadata should return nil,false: got value=%v ok=%v", value, ok)
	}
}

func TestMetadataHelpersHandleNilMap(t *testing.T) {
	msg := &DeviceMessage{}

	if value, ok := msg.GetMetadata("message_id"); ok || value != nil {
		t.Fatalf("nil metadata should return nil,false: got value=%v ok=%v", value, ok)
	}

	msg.SetMetadata("message_id", "msg-2")
	if value, ok := msg.GetMetadata("message_id"); !ok || value != "msg-2" {
		t.Fatalf("SetMetadata should initialize nil metadata map: got value=%v ok=%v", value, ok)
	}
}
