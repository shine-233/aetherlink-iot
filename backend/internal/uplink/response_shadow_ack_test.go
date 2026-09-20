// 文件用途：影子 ACK 上行解析的定向证据（ROADMAP P0.2）。
// 只覆盖不触碰数据库的早退路径：畸形载荷、缺失 shadow_id、result 非 0。
// 真正落库的确认路径需要 PostgreSQL，故此处不谎称已验证。
package uplink

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func newTestResponseUplink() *ResponseUplink {
	logger := logrus.New()
	logger.SetOutput(nopWriter{})
	return NewResponseUplink(ResponseUplinkConfig{Logger: logger})
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestShadowAckRejectsMalformedPayload(t *testing.T) {
	f := newTestResponseUplink()
	// 不是合法 JSON：必须安全返回，不得 panic，也不得继续走命令日志逻辑。
	f.processShadowAck(&DeviceMessage{
		Type:     MessageTypeShadowAck,
		DeviceID: "dev-1",
		Payload:  []byte(`not-json`),
	})
}

func TestShadowAckRejectsMissingShadowID(t *testing.T) {
	f := newTestResponseUplink()
	// 缺 shadow_id 且 metadata 也无：不得尝试落库。
	f.processShadowAck(&DeviceMessage{
		Type:     MessageTypeShadowAck,
		DeviceID: "dev-1",
		Payload:  []byte(`{"result":0}`),
		Metadata: map[string]interface{}{},
	})
}

func TestShadowAckWithNonZeroResultIsNotAcknowledged(t *testing.T) {
	f := newTestResponseUplink()
	// 设备明确报错：保留 sent 等重试，绝不写成 delivered。
	// 该路径在调用服务前返回，因此不需要数据库。
	f.processShadowAck(&DeviceMessage{
		Type:     MessageTypeShadowAck,
		DeviceID: "dev-1",
		Payload:  []byte(`{"shadow_id":"shadow-1","result":1,"message":"device rejected"}`),
	})
}

func TestShadowAckTypeIsRoutedToResponseChannel(t *testing.T) {
	// 影子 ACK 必须能被路由，否则会被总线以 ErrUnknownMessageType 丢弃。
	routed := isResponseMessageType(MessageTypeShadowAck)
	if !routed {
		t.Fatal("shadow_ack must be routed to the response channel")
	}
	for _, known := range []string{
		MessageTypeCommandResponse,
		MessageTypeAttributeSetResponse,
		MessageTypeGatewayCommandResponse,
		MessageTypeGatewayAttributeSetResponse,
	} {
		if !isResponseMessageType(known) {
			t.Fatalf("%s must remain routed to the response channel", known)
		}
	}
	// 非响应类型不得误入响应通道。
	for _, other := range []string{
		MessageTypeTelemetry, MessageTypeAttribute, MessageTypeEvent, MessageTypeStatus,
	} {
		if isResponseMessageType(other) {
			t.Fatalf("%s must not be routed to the response channel", other)
		}
	}
}
