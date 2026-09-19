// backpressure_test.go 验证摄取回压账本与告警采样器（P2.3 短期 B 方案）。
// 核心不变量：received = accepted + dropped(channel_full + caller_context + bus_closed)
// 在进程内严格成立；响应链路满队列丢弃可精确计数；阻塞事件与耗时在队列满后被记录；
// 告警窗口判定为纯函数且采样器可演练触发。

package uplink

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func newTestBusWithAlert(bufferSize int, cfg BackpressureAlertConfig) *Bus {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	return NewBus(BusConfig{BufferSize: bufferSize, Alert: &cfg}, logger)
}

func accountingSnapshot(t *testing.T, bus *Bus) map[string]interface{} {
	t.Helper()
	stats := bus.GetChannelStats()
	acct, ok := stats["accounting"].(map[string]interface{})
	if !ok {
		t.Fatalf("GetChannelStats missing accounting snapshot: %#v", stats)
	}
	return acct
}

func acctCounter(t *testing.T, acct map[string]interface{}, key string) uint64 {
	t.Helper()
	raw, ok := acct[key].(map[string]uint64)
	if !ok {
		t.Fatalf("accounting missing %s map: %#v", key, acct)
	}
	return raw["telemetry"] + raw["attribute"] + raw["event"] + raw["status"] + raw["response"]
}

func acctScalar(t *testing.T, acct map[string]interface{}, key string) uint64 {
	t.Helper()
	raw, ok := acct[key].(uint64)
	if !ok {
		t.Fatalf("accounting missing %s scalar: %#v", key, acct)
	}
	return raw
}

// TestBusAccountingLedgerStaysBalanced 填满队列后验证账本自洽与逐类计数。
func TestBusAccountingLedgerStaysBalanced(t *testing.T) {
	bus := newTestBus(1)
	t.Cleanup(bus.Close)

	// 遥测 1 条接受（队列满），第 2 条在阻塞等待中被 ctx 超时拒绝。
	if err := bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"}); err != nil {
		t.Fatalf("publish telemetry: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := bus.PublishContext(ctx, &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-2"}); err == nil {
		t.Fatalf("expected context deadline drop on full telemetry queue")
	}

	// 响应队列容量 1：第 1 条接受，第 2 条走满即丢（显式丢弃策略）。
	if err := bus.PublishResponse(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-1"}); err != nil {
		t.Fatalf("publish response: %v", err)
	}
	if err := bus.PublishResponse(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-2"}); err == nil {
		t.Fatalf("expected ErrChannelFull on full response queue")
	}

	acct := accountingSnapshot(t, bus)
	received := acctScalar(t, acct, "received_total")
	accepted := acctScalar(t, acct, "accepted_total")
	dropped := acctScalar(t, acct, UplinkDroppedTotalKey)
	full := acctCounter(t, acct, "dropped_channel_full")
	callerCtx := acctCounter(t, acct, "dropped_caller_context")
	closed := acctCounter(t, acct, "dropped_bus_closed")

	if received != accepted+full+callerCtx+closed {
		t.Fatalf("ledger not balanced: received=%d accepted=%d full=%d callerCtx=%d closed=%d",
			received, accepted, full, callerCtx, closed)
	}
	if accepted != 2 {
		t.Fatalf("accepted = %d, want 2 (1 telemetry + 1 response)", accepted)
	}
	if full != 1 {
		t.Fatalf("dropped_channel_total = %d, want 1 (response queue full)", full)
	}
	if callerCtx != 1 {
		t.Fatalf("dropped_caller_context = %d, want 1 (blocking publish aborted by ctx)", callerCtx)
	}
	if dropped != 2 {
		t.Fatalf("uplink_dropped_total = %d, want 2", dropped)
	}

	// 遥测队列满触发过阻塞：事件至少 1 次，累计耗时 > 0。
	blockedEvents := acctCounter(t, acct, "blocked_events")
	if blockedEvents < 1 {
		t.Fatalf("blocked_events = %d, want >= 1 after filling telemetry queue", blockedEvents)
	}
	blockedSeconds := acct["blocked_seconds_total"].(map[string]float64)["telemetry"]
	if blockedSeconds <= 0 {
		t.Fatalf("blocked_seconds_total[telemetry] = %v, want > 0", blockedSeconds)
	}
}

// TestBusAccountingCountsAdmissionRejectionAndUnknownType 覆盖准入拒绝与未知类型。
func TestBusAccountingCountsAdmissionRejectionAndUnknownType(t *testing.T) {
	bus := newTestBus(2)
	t.Cleanup(bus.Close)

	if err := bus.Publish(&DeviceMessage{Type: "mystery", DeviceID: "dev-1"}); err == nil {
		t.Fatalf("expected unknown message type rejection")
	}

	// 直接占用发布闸门，模拟 closing 期间的准入拒绝。
	if err := bus.beginPublish(); err != nil {
		t.Fatalf("seed beginPublish: %v", err)
	}
	// 释放种子发布者：t.Cleanup 的 Close 会等待发布闸门排空，不释放即死锁。
	defer bus.publishers.Done()
	bus.closing = true
	if err := bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"}); err == nil {
		t.Fatalf("expected admission rejection while closing")
	}

	acct := accountingSnapshot(t, bus)
	if got := acctScalar(t, acct, "rejected_admission"); got != 1 {
		t.Fatalf("rejected_admission = %d, want 1", got)
	}
	if got := acctScalar(t, acct, "dropped_unknown_type"); got != 1 {
		t.Fatalf("dropped_unknown_type = %d, want 1", got)
	}
	// 准入拒绝不占 received，received 仍为 0。
	if got := acctScalar(t, acct, "received_total"); got != 0 {
		t.Fatalf("received_total = %d, want 0 (rejection happens before admission)", got)
	}
}

// TestEvaluateBackpressureDropRatio 表驱动覆盖告警判定纯函数。
func TestEvaluateBackpressureDropRatio(t *testing.T) {
	cases := []struct {
		name        string
		received    uint64
		dropped     uint64
		minReceived uint64
		threshold   float64
		wantAlert   bool
	}{
		{"no traffic", 0, 0, 100, 0.01, false},
		{"below min received", 50, 10, 100, 0.01, false},
		{"healthy", 1000, 0, 100, 0.01, false},
		{"at threshold not alerting", 1000, 10, 100, 0.01, false},
		{"above threshold", 1000, 11, 100, 0.01, true},
		{"all dropped", 100, 100, 100, 0.01, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, alert := evaluateBackpressureDropRatio(tc.received, tc.dropped, tc.minReceived, tc.threshold)
			if alert != tc.wantAlert {
				t.Fatalf("evaluateBackpressureDropRatio(%d,%d,%d,%v) alert = %v, want %v",
					tc.received, tc.dropped, tc.minReceived, tc.threshold, alert, tc.wantAlert)
			}
		})
	}
}

// TestBackpressureAlertSamplerFires 演练告警触发：压低窗口/下限，制造丢弃，
// 采样器应在窗口到期后记录告警状态（验收口径"告警阈值触发可演练"）。
func TestBackpressureAlertSamplerFires(t *testing.T) {
	bus := newTestBusWithAlert(1, BackpressureAlertConfig{
		Enabled:     true,
		Window:      50 * time.Millisecond,
		DropRatio:   0.5,
		MinReceived: 2,
	})
	t.Cleanup(bus.Close)

	// 响应队列容量 1：第 1 条接受，后 5 条瞬间产生 channel_full 丢弃，
	// 零耗时完成，确保全部落入同一采样窗口，不受操作系统调度精度影响。
	_ = bus.PublishResponse(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-1"})
	for i := 0; i < 5; i++ {
		_ = bus.PublishResponse(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-drop"})
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stats := bus.GetChannelStats()
		alert, ok := stats["backpressure_alert"].(map[string]interface{})
		if ok {
			if count, _ := alert["alert_count"].(uint64); count >= 1 {
				if alert["last_alert_at"] == nil {
					t.Fatalf("alert fired but last_alert_at is nil: %#v", alert)
				}
				if ratio, _ := alert["last_drop_ratio"].(float64); ratio <= 0.5 {
					t.Fatalf("last_drop_ratio = %v, want > 0.5", ratio)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("backpressure alert did not fire within deadline")
}

// TestDiagnosticsBusSnapshotNilSafe 验证未注册时快照安全返回 nil，注册后可读取。
func TestDiagnosticsBusSnapshotNilSafe(t *testing.T) {
	RegisterDiagnosticsBus(nil)
	if snapshot := DefaultBusSnapshot(); snapshot != nil {
		t.Fatalf("DefaultBusSnapshot without registration = %#v, want nil", snapshot)
	}

	bus := newTestBus(1)
	t.Cleanup(func() {
		RegisterDiagnosticsBus(nil)
		bus.Close()
	})
	RegisterDiagnosticsBus(bus)
	snapshot := DefaultBusSnapshot()
	if snapshot == nil {
		t.Fatalf("DefaultBusSnapshot after registration = nil")
	}
	if _, ok := snapshot["accounting"]; !ok {
		t.Fatalf("registered bus snapshot missing accounting: %#v", snapshot)
	}
}
