// 文件用途：验证心跳服务和离线监控的 Redis/状态发布边界。
// 核心逻辑：使用 stub 或 fake Redis 行为断言心跳写入、过期判断和离线状态发布。
// 关键注意事项：心跳误判会影响设备在线状态，测试需覆盖 Redis 错误、重复离线和发布失败。
// 重构建议：引入时钟和发布器接口夹具，补齐过期扫描、并发心跳和外部发布失败测试。
package service

import (
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/model"
)

type heartbeatTestPublisher struct {
	events []heartbeatTestOfflineEvent
	err    error
}

type heartbeatTestOfflineEvent struct {
	deviceID string
	source   string
}

func heartbeatTestRequireError(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}
	if err.Error() != want {
		t.Fatalf("heartbeat error = %q, want %q", err.Error(), want)
	}
}

func (p *heartbeatTestPublisher) PublishStatusOffline(deviceID, source string) error {
	p.events = append(p.events, heartbeatTestOfflineEvent{deviceID: deviceID, source: source})
	return p.err
}

func TestHeartbeatServiceSetKeysRejectInvalidDurationsBeforeRedis(t *testing.T) {
	service := NewHeartbeatService(nil, logrus.New())

	heartbeatTestRequireError(t, service.SetHeartbeat("device-1", 0), "invalid heartbeat interval: 0")
	heartbeatTestRequireError(t, service.SetHeartbeat("device-1", -10), "invalid heartbeat interval: -10")
	heartbeatTestRequireError(t, service.SetTimeout("device-1", 0), "invalid timeout: 0")
	heartbeatTestRequireError(t, service.SetTimeout("device-1", -10), "invalid timeout: -10")
}

func TestHeartbeatServiceRefreshNoopsWhenConfigMissingOrEmpty(t *testing.T) {
	service := NewHeartbeatService(nil, logrus.New())
	device := &model.Device{ID: "device-1"}

	if err := service.RefreshHeartbeat(device, nil); err != nil {
		t.Fatalf("nil config should be a no-op: %v", err)
	}
	if err := service.RefreshHeartbeat(device, &HeartbeatConfig{}); err != nil {
		t.Fatalf("empty config should be a no-op: %v", err)
	}
}

func TestHeartbeatMonitorIgnoresUnrelatedExpiredKeys(t *testing.T) {
	publisher := &heartbeatTestPublisher{}
	monitor := NewHeartbeatMonitor(nil, publisher, logrus.New())

	for _, payload := range []string{
		"session:user-1",
		"device:missing-type",
		"device:device-1:telemetry",
		"device:device-1:heartbeat:extra",
	} {
		monitor.handleExpiredKey(&redis.Message{Payload: payload})
	}

	if len(publisher.events) != 0 {
		t.Fatalf("unrelated expired keys should not publish offline events: %#v", publisher.events)
	}
}

func TestHeartbeatMonitorPublishesHeartbeatAndTimeoutOfflineSources(t *testing.T) {
	publisher := &heartbeatTestPublisher{}
	monitor := NewHeartbeatMonitor(nil, publisher, logrus.New())

	monitor.handleExpiredKey(&redis.Message{Payload: "device:device-1:heartbeat"})
	monitor.handleExpiredKey(&redis.Message{Payload: "device:device-2:timeout"})

	want := []heartbeatTestOfflineEvent{
		{deviceID: "device-1", source: "heartbeat_expired"},
		{deviceID: "device-2", source: "timeout_expired"},
	}
	if len(publisher.events) != len(want) {
		t.Fatalf("offline events = %#v, want %#v", publisher.events, want)
	}
	for i := range want {
		if publisher.events[i] != want[i] {
			t.Fatalf("offline event %d = %#v, want %#v", i, publisher.events[i], want[i])
		}
	}
}

func TestHeartbeatMonitorContinuesWhenPublisherReturnsError(t *testing.T) {
	publisher := &heartbeatTestPublisher{err: errors.New("flow bus unavailable")}
	monitor := NewHeartbeatMonitor(nil, publisher, logrus.New())

	monitor.handleExpiredKey(&redis.Message{Payload: "device:device-1:heartbeat"})

	if len(publisher.events) != 1 {
		t.Fatalf("publisher error should not hide attempted offline event, got %#v", publisher.events)
	}
	if publisher.events[0].deviceID != "device-1" || publisher.events[0].source != "heartbeat_expired" {
		t.Fatalf("unexpected attempted offline event: %#v", publisher.events[0])
	}
}

func TestHeartbeatMonitorAllowsMissingPublisherWithoutPanic(t *testing.T) {
	monitor := NewHeartbeatMonitor(nil, nil, logrus.New())

	monitor.handleExpiredKey(&redis.Message{Payload: "device:device-1:timeout"})
}
