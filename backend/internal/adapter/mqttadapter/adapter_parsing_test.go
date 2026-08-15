// 文件用途：验证 MQTT adapter 的设备消息解析行为。
// 核心逻辑：覆盖 JSON payload、topic 字段和消息类型解析，确保上行数据进入正确后端模型。
// 关键注意事项：测试应聚焦协议边界，不依赖真实 broker，避免把解析单测变成外部集成测试。
// 重构建议：可继续补充错误 JSON、缺字段和网关嵌套 payload 的表驱动用例。

package mqttadapter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func newParsingTestAdapter() *Adapter {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	return NewAdapter(nil, nil, logger)
}

func encodePublicPayload(t *testing.T, deviceID string, values []byte) []byte {
	t.Helper()

	body, err := json.Marshal(publicPayload{
		DeviceId: deviceID,
		Values:   values,
	})
	if err != nil {
		t.Fatalf("marshal public payload: %v", err)
	}
	return body
}

func assertExactAdapterError(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}
	if err.Error() != want {
		t.Fatalf("error mismatch:\n got: %q\nwant: %q", err.Error(), want)
	}
}

func TestVerifyPayloadContracts(t *testing.T) {
	adapter := newParsingTestAdapter()

	valid, err := adapter.verifyPayload(encodePublicPayload(t, "rdi-device-001", []byte(`{"temperature":26.5}`)))
	if err != nil {
		t.Fatalf("valid payload should pass: %v", err)
	}
	if valid.DeviceId != "rdi-device-001" || string(valid.Values) != `{"temperature":26.5}` {
		t.Fatalf("decoded payload mismatch: %#v values=%s", valid, string(valid.Values))
	}

	tests := []struct {
		name    string
		body    []byte
		wantErr string
	}{
		{
			name:    "invalid json",
			body:    []byte(`not-json`),
			wantErr: "failed to unmarshal payload: invalid character 'o' in literal null (expecting 'u')",
		},
		{
			name:    "empty device id",
			body:    encodePublicPayload(t, "", []byte(`{"temperature":26.5}`)),
			wantErr: "device_id cannot be empty",
		},
		{
			name:    "empty values",
			body:    encodePublicPayload(t, "rdi-device-001", nil),
			wantErr: "values cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := adapter.verifyPayload(tt.body)
			assertExactAdapterError(t, err, tt.wantErr)
		})
	}
}

func TestDecodeStatusPayloadUsesBrokerUplinkEnvelope(t *testing.T) {
	adapter := newParsingTestAdapter()

	decoded, err := adapter.decodeStatusPayload(encodePublicPayload(t, "device-001", []byte("1")))
	if err != nil {
		t.Fatalf("status envelope should decode: %v", err)
	}
	if string(decoded) != "1" {
		t.Fatalf("decoded status payload = %q, want %q", decoded, "1")
	}

	if _, err := adapter.decodeStatusPayload([]byte(`{"device_id":"device-001","values":""}`)); err == nil {
		t.Fatal("empty status envelope values should be rejected")
	}
}

func TestAdapterTopicAndPayloadParsing(t *testing.T) {
	adapter := newParsingTestAdapter()

	if got := adapter.detectMessageType("devices/telemetry/rdi-device-001", "telemetry"); got != "telemetry" {
		t.Fatalf("device message type mismatch: %q", got)
	}
	if got := adapter.detectMessageType("gateway/telemetry/rdi-gateway-001", "telemetry"); got != "gateway_telemetry" {
		t.Fatalf("gateway message type mismatch: %q", got)
	}
	if got := adapter.detectMessageType("gateway", "event"); got != "event" {
		t.Fatalf("short topic should not be treated as gateway: %q", got)
	}

	messageID, err := adapter.parseAttributeOrEventTopic("devices/attributes/msg-001")
	if err != nil || messageID != "msg-001" {
		t.Fatalf("message id parse mismatch: id=%q err=%v", messageID, err)
	}
	_, err = adapter.parseAttributeOrEventTopic("devices/attributes/")
	assertExactAdapterError(t, err, "message_id is empty in topic: devices/attributes/")
	_, err = adapter.parseAttributeOrEventTopic("devices")
	assertExactAdapterError(t, err, "invalid topic format: devices (expected at least 3 parts)")

	if got := adapter.parseEventMethod([]byte(`{"method":"dry_contact_alarm"}`)); got != "dry_contact_alarm" {
		t.Fatalf("event method mismatch: %q", got)
	}
	if got := adapter.parseEventMethod([]byte(`bad-json`)); got != "" {
		t.Fatalf("bad event JSON should return empty method, got %q", got)
	}

	if got := adapter.extractDeviceIDFromPayload([]byte(`{"device_id":"rdi-device-001","values":"e30="}`)); got != "rdi-device-001" {
		t.Fatalf("device id mismatch: %q", got)
	}
	if got := adapter.extractDeviceIDFromPayload([]byte(`bad-json`)); got != "" {
		t.Fatalf("bad payload should return empty device id, got %q", got)
	}
}

func TestMQTTUplinkSourceIDUsesTrustedOriginAndProtocolMessageIdentity(t *testing.T) {
	base := mqttUplinkSourceID("tenant-1", "gateway-1", "event", "message-1")
	if len(base) != 64 {
		t.Fatalf("source id length = %d, want 64 hex characters", len(base))
	}
	if repeated := mqttUplinkSourceID("tenant-1", "gateway-1", "event", " message-1 "); repeated != base {
		t.Fatalf("trimmed retry source id = %q, want %q", repeated, base)
	}
	if empty := mqttUplinkSourceID("tenant-1", "gateway-1", "event", ""); empty != "" {
		t.Fatalf("empty message id produced source id %q", empty)
	}

	tests := []struct {
		name      string
		tenantID  string
		deviceID  string
		dataType  string
		messageID string
	}{
		{name: "tenant", tenantID: "tenant-2", deviceID: "gateway-1", dataType: "event", messageID: "message-1"},
		{name: "trusted origin device", tenantID: "tenant-1", deviceID: "gateway-2", dataType: "event", messageID: "message-1"},
		{name: "data type", tenantID: "tenant-1", deviceID: "gateway-1", dataType: "attribute", messageID: "message-1"},
		{name: "protocol message", tenantID: "tenant-1", deviceID: "gateway-1", dataType: "event", messageID: "message-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mqttUplinkSourceID(tt.tenantID, tt.deviceID, tt.dataType, tt.messageID)
			if got == base {
				t.Fatalf("changed %s retained source id %q", tt.name, got)
			}
		})
	}
}
