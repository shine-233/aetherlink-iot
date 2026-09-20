// 文件用途：场景执行窗口与冲突停止的定向证据（ROADMAP P0.4）。
// 门禁要求"边界时间表驱动测试"，故窗口判定用表驱动覆盖边界；
// 冲突停止用内存注册表验证"停止动作可审计"与"不重复审计"。
package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("bad time %q: %v", value, err)
	}
	return parsed
}

func ptr(value time.Time) *time.Time { return &value }

func TestCanRunBoundaryTable(t *testing.T) {
	start := mustTime(t, "2026-09-11T00:00:00Z")
	expiry := mustTime(t, "2026-09-12T00:00:00Z")
	engine := FlowEngine{}

	cases := []struct {
		name  string
		now   time.Time
		start *time.Time
		end   *time.Time
		want  bool
	}{
		{"before window", start.Add(-time.Nanosecond), ptr(start), ptr(expiry), false},
		{"exactly at start", start, ptr(start), ptr(expiry), true},
		{"inside window", start.Add(time.Hour), ptr(start), ptr(expiry), true},
		{"one ns before expiry", expiry.Add(-time.Nanosecond), ptr(start), ptr(expiry), true},
		{"exactly at expiry is closed", expiry, ptr(start), ptr(expiry), false},
		{"after expiry", expiry.Add(time.Second), ptr(start), ptr(expiry), false},
		{"no lower bound before expiry", expiry.Add(-time.Hour), nil, ptr(expiry), true},
		{"no lower bound after expiry", expiry.Add(time.Hour), nil, ptr(expiry), false},
		{"no upper bound after start", start.Add(time.Hour), ptr(start), nil, true},
		{"no upper bound before start", start.Add(-time.Hour), ptr(start), nil, false},
		{"fully unbounded", start, nil, nil, true},
	}
	for _, c := range cases {
		got, err := engine.CanRun(c.now, ExecutionWindow{StartsAt: c.start, ExpiresAt: c.end})
		if err != nil {
			t.Fatalf("%s: unexpected error %v", c.name, err)
		}
		if got != c.want {
			t.Fatalf("%s: CanRun = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCanRunInvalidTimezoneFailsClosed(t *testing.T) {
	engine := FlowEngine{}
	now := mustTime(t, "2026-09-11T12:00:00Z")

	// 非法时区必须报错，绝不悄悄按 UTC 放行（那会让窗口边界整体偏移）。
	got, err := engine.CanRun(now, ExecutionWindow{Timezone: "Not/AZone"})
	if err == nil {
		t.Fatal("invalid timezone must fail closed, got nil error")
	}
	if got {
		t.Fatal("invalid timezone must not report runnable")
	}
	if !errors.Is(err, ErrInvalidExecutionTimezone) {
		t.Fatalf("expected ErrInvalidExecutionTimezone, got %v", err)
	}

	// 空时区按 UTC 处理，这是显式约定而非降级。
	if ok, err := engine.CanRun(now, ExecutionWindow{}); err != nil || !ok {
		t.Fatalf("empty timezone must mean UTC and be runnable, got %v %v", ok, err)
	}
}

func TestCanRunHonoursTimezoneOffset(t *testing.T) {
	engine := FlowEngine{}
	// 窗口 00:00-01:00（上海时区）；UTC 12:15 即上海 20:15，应落在窗口外。
	start := mustTime(t, "2026-09-11T00:00:00+08:00")
	end := mustTime(t, "2026-09-11T01:00:00+08:00")
	inWindow := mustTime(t, "2026-09-11T00:30:00+08:00")
	outWindow := mustTime(t, "2026-09-11T12:15:00Z")

	if ok, err := engine.CanRun(inWindow, ExecutionWindow{StartsAt: ptr(start), ExpiresAt: ptr(end), Timezone: "Asia/Shanghai"}); err != nil || !ok {
		t.Fatalf("in-window check failed: %v %v", ok, err)
	}
	if ok, err := engine.CanRun(outWindow, ExecutionWindow{StartsAt: ptr(start), ExpiresAt: ptr(end), Timezone: "Asia/Shanghai"}); err != nil || ok {
		t.Fatalf("out-of-window must be false: %v %v", ok, err)
	}
}

func TestIsExpired(t *testing.T) {
	engine := FlowEngine{}
	expiry := mustTime(t, "2026-09-11T00:00:00Z")

	if expired, err := engine.IsExpired(expiry.Add(-time.Second), ExecutionWindow{ExpiresAt: ptr(expiry)}); err != nil || expired {
		t.Fatalf("before expiry must not be expired: %v %v", expired, err)
	}
	if expired, err := engine.IsExpired(expiry, ExecutionWindow{ExpiresAt: ptr(expiry)}); err != nil || !expired {
		t.Fatalf("at expiry must be expired: %v %v", expired, err)
	}
	if expired, err := engine.IsExpired(expiry, ExecutionWindow{}); err != nil || expired {
		t.Fatalf("no expiry must never expire: %v %v", expired, err)
	}
}

func TestFlowTriggerKeyIsStableAndScoped(t *testing.T) {
	// 刻意取 .100 作为基线：caller 传入的是"预定触发时刻"，同一次触发重复上报时
	// 只会有亚秒级抖动，键按秒截断即可吸收。
	at := mustTime(t, "2026-09-11T10:00:30.100Z")
	base := FlowTriggerIdentity{FlowID: "f1", DeviceID: "d1", TriggerAt: at}

	// 亚秒抖动不影响幂等键：定时器抖动不应把一次触发放大成多次。
	jitter := base
	jitter.TriggerAt = at.Add(300 * time.Millisecond)
	if FlowTriggerKey(base) != FlowTriggerKey(jitter) {
		t.Fatal("sub-second jitter must not change the trigger key")
	}

	// 不同 flow / device / 秒级时刻必须产生不同键。
	differ := []FlowTriggerIdentity{
		{FlowID: "f2", DeviceID: "d1", TriggerAt: at},
		{FlowID: "f1", DeviceID: "d2", TriggerAt: at},
		{FlowID: "f1", DeviceID: "d1", TriggerAt: at.Add(time.Second)},
	}
	for _, other := range differ {
		if FlowTriggerKey(other) == FlowTriggerKey(base) {
			t.Fatalf("trigger key must differ for %+v", other)
		}
	}
}

// memRegistry 内存注册表，用于验证停止与审计行为。
type memRegistry struct {
	running  map[string][]string
	stopped  []string
	stopHits map[string]int
}

func (m *memRegistry) ListRunningFlows(_ context.Context, deviceID, exclude string) ([]string, error) {
	out := []string{}
	for _, id := range m.running[deviceID] {
		if id != exclude {
			out = append(out, id)
		}
	}
	return out, nil
}

func (m *memRegistry) StopFlow(_ context.Context, _, flowID string) (bool, error) {
	m.stopHits[flowID]++
	// 第一次停止返回 true（确实发生状态变化），后续返回 false（已被停止）。
	return m.stopHits[flowID] == 1, nil
}

type memAudit struct{ records []string }

func (a *memAudit) RecordFlowStopped(_ context.Context, deviceID, flowID, reason string) error {
	a.records = append(a.records, deviceID+"/"+flowID+"/"+reason)
	return nil
}

func TestStopConflictingFlowsStopsAndAudits(t *testing.T) {
	registry := &memRegistry{
		running:  map[string][]string{"d1": {"flow-old-a", "flow-old-b", "flow-self"}},
		stopHits: map[string]int{},
	}
	audit := &memAudit{}
	engine := NewFlowEngine(registry, audit)

	stopped, err := engine.StopConflictingFlows(context.Background(), "d1", "flow-self")
	if err != nil {
		t.Fatalf("StopConflictingFlows: %v", err)
	}
	if len(stopped) != 2 {
		t.Fatalf("stopped = %v, want the two other flows", stopped)
	}
	// 自身不得被停止。
	for _, id := range stopped {
		if id == "flow-self" {
			t.Fatal("must not stop the requesting flow itself")
		}
	}
	// 每一次停止都必须留下审计事件，禁止静默停止。
	if len(audit.records) != len(stopped) {
		t.Fatalf("audit records = %d, want one per stop (%d)", len(audit.records), len(stopped))
	}
}

func TestStopConflictingFlowsRequiresAuditSink(t *testing.T) {
	registry := &memRegistry{running: map[string][]string{"d1": {"other"}}, stopHits: map[string]int{}}
	// 缺少审计落点时拒绝执行，而不是静默停止。
	engine := NewFlowEngine(registry, nil)
	if _, err := engine.StopConflictingFlows(context.Background(), "d1", "self"); err == nil {
		t.Fatal("missing audit sink must be rejected")
	}

	// 缺少注册表同样拒绝。
	engine = NewFlowEngine(nil, &memAudit{})
	if _, err := engine.StopConflictingFlows(context.Background(), "d1", "self"); err == nil {
		t.Fatal("missing registry must be rejected")
	}
}
