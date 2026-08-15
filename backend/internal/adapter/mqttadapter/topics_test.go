// 文件用途：验证 MQTT adapter topic 构造与解析契约。
// 核心逻辑：覆盖直连和网关设备 topic 的生成、匹配和消息类型映射。
// 关键注意事项：测试需要保持与设备文档和 broker 订阅一致，避免只验证本地字符串拼接。
// 重构建议：可将设备协议样例沉淀为 golden case，并同步覆盖非法 topic 分支。

package mqttadapter

import (
	"testing"

	"aetherlink-iot/backend/internal/uplink"

	"github.com/spf13/viper"
)

func TestBuildDeviceDownlinkTopics(t *testing.T) {
	deviceNumber := "rdi-device-001"
	messageID := "msg-20260627"

	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "attribute response",
			got:  BuildAttributeResponseTopic(deviceNumber, messageID),
			want: "devices/attributes/response/rdi-device-001/msg-20260627",
		},
		{
			name: "event response",
			got:  BuildEventResponseTopic(deviceNumber, messageID),
			want: "devices/event/response/rdi-device-001/msg-20260627",
		},
		{
			name: "attribute set",
			got:  BuildAttributeSetTopic(deviceNumber, messageID),
			want: "devices/attributes/set/rdi-device-001/msg-20260627",
		},
		{
			name: "attribute get",
			got:  BuildAttributeGetTopic(deviceNumber),
			want: "devices/attributes/get/rdi-device-001",
		},
		{
			name: "command",
			got:  BuildCommandTopic(deviceNumber, messageID),
			want: "devices/command/rdi-device-001/msg-20260627",
		},
		{
			name: "telemetry control",
			got:  BuildTelemetryControlTopic(deviceNumber),
			want: "devices/telemetry/control/rdi-device-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("topic mismatch: got %q want %q", tt.got, tt.want)
			}
		})
	}
}

func TestBuildGatewayDownlinkTopics(t *testing.T) {
	gatewayNumber := "rdi-gateway-001"
	messageID := "msg-20260627"

	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "gateway attribute set",
			got:  BuildGatewayAttributeSetTopic(gatewayNumber, messageID),
			want: "gateway/attributes/set/rdi-gateway-001/msg-20260627",
		},
		{
			name: "gateway attribute get",
			got:  BuildGatewayAttributeGetTopic(gatewayNumber),
			want: "gateway/attributes/get/rdi-gateway-001",
		},
		{
			name: "gateway command",
			got:  BuildGatewayCommandTopic(gatewayNumber, messageID),
			want: "gateway/command/rdi-gateway-001/msg-20260627",
		},
		{
			name: "gateway telemetry control",
			got:  BuildGatewayTelemetryControlTopic(gatewayNumber),
			want: "gateway/telemetry/control/rdi-gateway-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("topic mismatch: got %q want %q", tt.got, tt.want)
			}
		})
	}
}

func TestDetectResponseType(t *testing.T) {
	adapter := &Adapter{}

	tests := []struct {
		name  string
		topic string
		want  string
	}{
		{
			name:  "device command response",
			topic: "devices/command/response/msg-1",
			want:  uplink.MessageTypeCommandResponse,
		},
		{
			name:  "device attribute set response",
			topic: "devices/attributes/set/response/msg-1",
			want:  uplink.MessageTypeAttributeSetResponse,
		},
		{
			name:  "gateway command response",
			topic: "gateway/command/response/msg-1",
			want:  uplink.MessageTypeGatewayCommandResponse,
		},
		{
			name:  "gateway attribute set response",
			topic: "gateway/attributes/set/response/msg-1",
			want:  uplink.MessageTypeGatewayAttributeSetResponse,
		},
		{
			name:  "unknown response",
			topic: "devices/telemetry/response/msg-1",
			want:  "unknown_response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adapter.detectResponseType(tt.topic); got != tt.want {
				t.Fatalf("response type mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestGenSharedTopic(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	const topic = "devices/command/response/+"

	if got := genSharedTopic(topic); got != topic {
		t.Fatalf("shared subscription should be disabled by default: got %q want %q", got, topic)
	}

	viper.Set("mqtt.enable_shared_subscription", true)
	if got := genSharedTopic(topic); got != "$share/mygroup/"+topic {
		t.Fatalf("default shared group mismatch: got %q", got)
	}

	viper.Set("mqtt.shared_subscription_group", "aetherlink")
	if got := genSharedTopic(topic); got != "$share/aetherlink/"+topic {
		t.Fatalf("custom shared group mismatch: got %q", got)
	}
}
