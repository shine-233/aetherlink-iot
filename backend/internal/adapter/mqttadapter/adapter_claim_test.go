// 文件用途：MQTT 适配器的设备自主认领（TB-12）契约测试。
// 核心逻辑：覆盖前置校验路径——载荷畸形、缺少 device_id、缺少 secretKey、secretKey 长度越界。
// 这些路径在触达数据库之前即被防御拒绝。
package mqttadapter

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func newClaimTestAdapter(t *testing.T) *Adapter {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	return NewAdapter(nil, nil, logger)
}

func TestHandleDeviceClaimMessageRejectsInvalidJSON(t *testing.T) {
	adapter := newClaimTestAdapter(t)

	err := adapter.HandleDeviceClaimMessage([]byte("not-a-json"), TopicPatternDeviceClaim)
	if err == nil {
		t.Fatal("非 JSON 载荷必须被拒绝")
	}
	if !strings.Contains(err.Error(), "invalid device claim JSON") {
		t.Fatalf("期望 JSON 解析失败错误，实际：%v", err)
	}
}

func TestHandleDeviceClaimMessageRejectsMissingDeviceID(t *testing.T) {
	adapter := newClaimTestAdapter(t)

	err := adapter.HandleDeviceClaimMessage([]byte(`{"secretKey":"my-claim-key-1234"}`), TopicPatternDeviceClaim)
	if err == nil {
		t.Fatal("缺少 device_id 的认领消息必须被拒绝")
	}
	if !strings.Contains(err.Error(), "device_id missing") {
		t.Fatalf("期望缺少 device_id 错误，实际：%v", err)
	}
}

func TestHandleDeviceClaimMessageRejectsMissingSecretKey(t *testing.T) {
	adapter := newClaimTestAdapter(t)

	err := adapter.HandleDeviceClaimMessage([]byte(`{"device_id":"dev-123","durationMs":60000}`), TopicPatternDeviceClaim)
	if err == nil {
		t.Fatal("缺少 secretKey/claimKey 的认领消息必须被拒绝")
	}
	if !strings.Contains(err.Error(), "secretKey or claimKey is required") {
		t.Fatalf("期望缺少 secretKey 错误，实际：%v", err)
	}
}

func TestHandleDeviceClaimMessageRejectsShortSecretKey(t *testing.T) {
	adapter := newClaimTestAdapter(t)

	err := adapter.HandleDeviceClaimMessage([]byte(`{"device_id":"dev-123","secretKey":"abc"}`), TopicPatternDeviceClaim)
	if err == nil {
		t.Fatal("长度小于 4 的 secretKey 必须被拒绝")
	}
	if !strings.Contains(err.Error(), "between 4 and 128") {
		t.Fatalf("期望长度越界错误，实际：%v", err)
	}
}
