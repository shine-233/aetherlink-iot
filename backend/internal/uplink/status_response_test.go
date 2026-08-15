// 文件用途：覆盖上行消息模块 status response 行为的 Go 测试。
// 核心逻辑：验证总线分发、状态/响应解析和错误边界等上行处理契约，主要围绕 func newQuietStatusUplink、func newQuietResponseUplink、func TestStatusUplinkParseStatusAcceptsOnlyExactOnlineOfflinePayloads、func TestResponseUplinkParseResponseMapsDeviceAckToLogOutcome 等声明展开。
// 关键注意事项：测试需关注异步通道容量、关闭状态和消息类型路由，避免竞态断言。
// 重构建议：后续可补充网关子设备、自动化触发和存储失败的场景化用例。

package uplink

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func newQuietStatusUplink() *StatusUplink {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	return NewStatusUplink(StatusUplinkConfig{Logger: logger})
}

func newQuietResponseUplink() *ResponseUplink {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	return NewResponseUplink(ResponseUplinkConfig{Logger: logger})
}

func TestStatusUplinkParseStatusAcceptsOnlyExactOnlineOfflinePayloads(t *testing.T) {
	uplink := newQuietStatusUplink()

	tests := []struct {
		name    string
		payload []byte
		want    int16
	}{
		{name: "offline", payload: []byte("0"), want: 0},
		{name: "online", payload: []byte("1"), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := uplink.parseStatus(tt.payload)
			if err != nil {
				t.Fatalf("parseStatus(%q) returned error: %v", tt.payload, err)
			}
			if got != tt.want {
				t.Fatalf("parseStatus(%q) = %d, want %d", tt.payload, got, tt.want)
			}
		})
	}

	invalidPayloads := []struct {
		payload []byte
		wantErr string
	}{
		{payload: []byte(""), wantErr: "invalid status value:  (expected 0 or 1)"},
		{payload: []byte("2"), wantErr: "invalid status value: 2 (expected 0 or 1)"},
		{payload: []byte("true"), wantErr: "invalid status value: true (expected 0 or 1)"},
		{payload: []byte(" 1 "), wantErr: "invalid status value:  1  (expected 0 or 1)"},
	}
	for _, tt := range invalidPayloads {
		t.Run("reject "+string(tt.payload), func(t *testing.T) {
			if _, err := uplink.parseStatus(tt.payload); err == nil || err.Error() != tt.wantErr {
				t.Fatalf("parseStatus(%q) error = %v, want %q", tt.payload, err, tt.wantErr)
			}
		})
	}
}

func TestResponseUplinkParseResponseMapsDeviceAckToLogOutcome(t *testing.T) {
	uplink := newQuietResponseUplink()

	tests := []struct {
		name        string
		payload     []byte
		wantMessage string
		wantSuccess bool
	}{
		{
			name:        "successful command acknowledgement",
			payload:     []byte(`{"result":0,"message":"success","ts":1609143039,"method":"set_led"}`),
			wantMessage: "",
			wantSuccess: true,
		},
		{
			name:        "failed command prefers errcode",
			payload:     []byte(`{"result":1,"message":"failed","errcode":"E_DEVICE_BUSY","method":"set_led"}`),
			wantMessage: "E_DEVICE_BUSY",
			wantSuccess: false,
		},
		{
			name:        "failed command falls back to message",
			payload:     []byte(`{"result":1,"message":"timeout","method":"set_led"}`),
			wantMessage: "timeout",
			wantSuccess: false,
		},
		{
			name:        "failed command without details keeps raw payload",
			payload:     []byte(`{"result":1}`),
			wantMessage: `{"result":1}`,
			wantSuccess: false,
		},
		{
			name:        "malformed acknowledgement is a failed raw response",
			payload:     []byte(`not-json`),
			wantMessage: "not-json",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMessage, gotSuccess := uplink.parseResponse(tt.payload)
			if gotSuccess != tt.wantSuccess {
				t.Fatalf("parseResponse success = %v, want %v", gotSuccess, tt.wantSuccess)
			}
			if gotMessage != tt.wantMessage {
				t.Fatalf("parseResponse message = %q, want %q", gotMessage, tt.wantMessage)
			}
		})
	}
}
