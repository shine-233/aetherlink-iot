// 文件用途：edgerules 执行器单测（TB-21 scoped v1）——修订号重连去重/升级重装载/命中落地/去重窗口。
// 核心逻辑：LoadSnapshot 四态行为 + ProcessTelemetry 求值→去重→EventSink 全链路，时钟注入免 sleep。
// 关键注意事项：全部进程内，无 DB/broker；MemorySink 收集断言。
package edgerules

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// testClock 可步进时钟（去重窗口测试用）。
type testClock struct{ current time.Time }

func (c *testClock) Now() time.Time { return c.current }
func (c *testClock) Advance(d time.Duration) {
	c.current = c.current.Add(d)
}

// newTestExecutor 构造带内存落地与注入时钟的执行器。
func newTestExecutor(t *testing.T, now *testClock) (*Executor, *MemorySink) {
	t.Helper()
	sink := &MemorySink{}
	x := NewExecutor(WithSink(sink), WithNow(now.Now), WithTenantID("tenant-edge"))
	return x, sink
}

// loadEnvelope 构造指定资源与修订号的快照信封。
func loadEnvelope(resourceID string, revision int64, graph string) []byte {
	return []byte(fmt.Sprintf(`{"type":"rule_chain","resource_id":%q,"name":"链-%s","content":%s,"revision":%d,"version":1,"generated_at":"2026-09-25T00:00:00Z"}`,
		resourceID, resourceID, graph, revision))
}

// highTempGraph 高温阈值链（阈值 80）。
const highTempGraph = `{
  "nodes": [
    {"id": "t", "type": "trigger.telemetry"},
    {"id": "f", "type": "filter.threshold", "config": {"key": "temperature", "op": ">", "value": 80}},
    {"id": "a", "type": "action.alarm", "config": {"name": "高温", "severity": "H"}}
  ],
  "edges": [{"from": "t", "to": "f"}, {"from": "f", "to": "a"}]
}`

// highTempGraphV2 升级版：阈值降到 50（用于修订号升级重装载语义）。
const highTempGraphV2 = `{
  "nodes": [
    {"id": "t", "type": "trigger.telemetry"},
    {"id": "f", "type": "filter.threshold", "config": {"key": "temperature", "op": ">", "value": 50}},
    {"id": "a", "type": "action.alarm", "config": {"name": "高温", "severity": "H"}}
  ],
  "edges": [{"from": "t", "to": "f"}, {"from": "f", "to": "a"}]
}`

func TestLoadSnapshotFirstLoadAndDedup(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, _ := newTestExecutor(t, clock)

	// 首次装载 revision 1。
	res := x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph))
	if res.Status != LoadStatusLoaded || res.ResourceID != "chain-1" || res.Revision != 1 {
		t.Fatalf("first load = %+v", res)
	}
	// 重连重放同一修订号：去重跳过，不重复执行。
	res = x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph))
	if res.Status != LoadStatusUnchanged {
		t.Fatalf("replay load = %+v, want unchanged", res)
	}
	// 装载状态未被破坏。
	if rev, ok := x.LoadedRevision("chain-1"); !ok || rev != 1 {
		t.Fatalf("LoadedRevision = %d %v", rev, ok)
	}
}

func TestLoadSnapshotRevisionUpgradeReloads(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, _ := newTestExecutor(t, clock)

	if res := x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph)); res.Status != LoadStatusLoaded {
		t.Fatalf("v1 load = %+v", res)
	}
	// 80 不触发 v1（阈值 80 是 >，不含有边界）——用 81 验证 v1 命中、60 不命中。
	if evs := x.ProcessTelemetry("dev-1", map[string]any{"temperature": 60}, time.Time{}); len(evs) != 0 {
		t.Fatalf("v1 should not fire at 60, got %+v", evs)
	}
	// 升级到 revision 2（阈值降到 50）：升级重装载后 60 应命中。
	res := x.LoadSnapshot(loadEnvelope("chain-1", 2, highTempGraphV2))
	if res.Status != LoadStatusLoaded || res.Revision != 2 {
		t.Fatalf("upgrade load = %+v", res)
	}
	evs := x.ProcessTelemetry("dev-1", map[string]any{"temperature": 60}, time.Time{})
	if len(evs) != 1 || evs[0].Revision != 2 {
		t.Fatalf("v2 should fire at 60 with revision 2, got %+v", evs)
	}
	// 回退到 revision 1：regression 跳过，仍跑 v2 语义（60 继续命中）。
	res = x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph))
	if res.Status != LoadStatusRegression {
		t.Fatalf("regression load = %+v", res)
	}
	if evs := x.ProcessTelemetry("dev-1", map[string]any{"temperature": 60}, time.Time{}); len(evs) != 1 {
		t.Fatalf("after regression graph must stay at v2, got %+v", evs)
	}
}

func TestLoadSnapshotRejectsInvalid(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, _ := newTestExecutor(t, clock)
	if res := x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph)); res.Status != LoadStatusLoaded {
		t.Fatalf("load = %+v", res)
	}
	cases := []struct {
		name string
		raw  []byte
	}{
		{"bad json", []byte(`{`)},
		{"wrong type", []byte(`{"type":"dashboard","resource_id":"c1","revision":9}`)},
		{"zero revision", []byte(`{"type":"rule_chain","resource_id":"c1","revision":0,"content":{"nodes":[],"edges":[]}}`)},
		{"invalid graph", loadEnvelope("chain-1", 9, `{"nodes":[],"edges":[]}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := x.LoadSnapshot(tc.raw)
			if res.Status != LoadStatusRejected {
				t.Fatalf("load = %+v, want rejected", res)
			}
		})
	}
	// 拒收后仍跑旧版（fail-closed：保留当前版本继续跑）。
	if rev, ok := x.LoadedRevision("chain-1"); !ok || rev != 1 {
		t.Fatalf("loaded revision must stay 1, got %d %v", rev, ok)
	}
}

func TestProcessTelemetryFullFlow(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, sink := newTestExecutor(t, clock)
	if res := x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph)); res.Status != LoadStatusLoaded {
		t.Fatalf("load = %+v", res)
	}

	// 未命中。
	if evs := x.ProcessTelemetry("dev-1", map[string]any{"temperature": 20}, clock.current); len(evs) != 0 {
		t.Fatalf("no fire expected, got %+v", evs)
	}
	// 命中：事件字段完整。
	ts := clock.current.Add(time.Minute)
	evs := x.ProcessTelemetry("dev-1", map[string]any{"temperature": 91}, ts)
	if len(evs) != 1 {
		t.Fatalf("hit expected, got %+v", evs)
	}
	ev := evs[0]
	if ev.ID == "" {
		t.Fatal("event id required")
	}
	if ev.ResourceID != "chain-1" || ev.Revision != 1 || ev.NodeID != "a" {
		t.Fatalf("event = %+v", ev)
	}
	if ev.AlarmName != "高温" || ev.Severity != "H" || ev.DeviceID != "dev-1" || ev.TenantID != "tenant-edge" {
		t.Fatalf("event = %+v", ev)
	}
	if ev.Condition != "temperature > 80" {
		t.Fatalf("condition = %q", ev.Condition)
	}
	// 无显式 content 时以条件文本留痕（对照 calcfield detail）。
	if !strings.Contains(ev.Content, "condition") {
		t.Fatalf("content = %q", ev.Content)
	}
	if ev.TriggeredAt != ts {
		t.Fatalf("triggered_at = %v, want %v", ev.TriggeredAt, ts)
	}
	// EventSink 收到同一事件。
	if got := sink.Snapshot(); len(got) != 1 || got[0].ID != ev.ID {
		t.Fatalf("sink = %+v", got)
	}
}

func TestProcessTelemetryRetriggerDedupWindow(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, sink := newTestExecutor(t, clock)
	graph := `{
	  "nodes": [
	    {"id": "t", "type": "trigger.telemetry"},
	    {"id": "a", "type": "action.alarm", "config": {"name": "高温", "retrigger_dedup_ms": 60000}}
	  ],
	  "edges": [{"from": "t", "to": "a"}]
	}`
	if res := x.LoadSnapshot(loadEnvelope("chain-1", 1, graph)); res.Status != LoadStatusLoaded {
		t.Fatalf("load = %+v", res)
	}
	values := map[string]any{"temperature": 99}

	// 第一次命中。
	if evs := x.ProcessTelemetry("dev-1", values, clock.current); len(evs) != 1 {
		t.Fatalf("first fire expected, got %+v", evs)
	}
	// 窗口内重复触发：去重跳过（不落新事件、不进 sink）。
	clock.Advance(30 * time.Second)
	if evs := x.ProcessTelemetry("dev-1", values, clock.current); len(evs) != 0 {
		t.Fatalf("window dedup expected, got %+v", evs)
	}
	if got := len(sink.Snapshot()); got != 1 {
		t.Fatalf("sink count = %d, want 1", got)
	}
	// 窗口外：再次命中。
	clock.Advance(31 * time.Second)
	if evs := x.ProcessTelemetry("dev-1", values, clock.current); len(evs) != 1 {
		t.Fatalf("post-window fire expected, got %+v", evs)
	}
	// 不同设备不共享窗口。
	if evs := x.ProcessTelemetry("dev-2", values, clock.current); len(evs) != 1 {
		t.Fatalf("other device should fire, got %+v", evs)
	}
	// 未配置窗口（默认 0）：不去重。
	x2, sink2 := newTestExecutor(t, clock)
	if res := x2.LoadSnapshot(loadEnvelope("chain-2", 1, highTempGraph)); res.Status != LoadStatusLoaded {
		t.Fatalf("load = %+v", res)
	}
	v2 := map[string]any{"temperature": 99}
	_ = x2.ProcessTelemetry("dev-1", v2, clock.current)
	if evs := x2.ProcessTelemetry("dev-1", v2, clock.current); len(evs) != 1 {
		t.Fatalf("no-window config must fire every time, got %+v", evs)
	}
	if got := len(sink2.Snapshot()); got != 2 {
		t.Fatalf("sink2 count = %d, want 2", got)
	}
}

func TestProcessTelemetryMultiResource(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, sink := newTestExecutor(t, clock)
	graphA := `{"nodes":[{"id":"t","type":"trigger.telemetry"},{"id":"a","type":"action.alarm","config":{"name":"A告警"}}],"edges":[{"from":"t","to":"a"}]}`
	graphB := `{"nodes":[{"id":"t","type":"trigger.telemetry"},{"id":"b","type":"action.alarm","config":{"name":"B告警","severity":"L"}}],"edges":[{"from":"t","to":"b"}]}`
	if res := x.LoadSnapshot(loadEnvelope("chain-b", 1, graphB)); res.Status != LoadStatusLoaded {
		t.Fatalf("load b = %+v", res)
	}
	if res := x.LoadSnapshot(loadEnvelope("chain-a", 1, graphA)); res.Status != LoadStatusLoaded {
		t.Fatalf("load a = %+v", res)
	}
	if got := x.LoadedResources(); len(got) != 2 || got[0] != "chain-a" || got[1] != "chain-b" {
		t.Fatalf("resources = %v", got)
	}
	evs := x.ProcessTelemetry("dev-1", map[string]any{"temperature": 99}, clock.current)
	if len(evs) != 2 {
		t.Fatalf("both chains should fire, got %+v", evs)
	}
	// 遍历顺序按 resource_id 排序，命中顺序确定。
	if evs[0].AlarmName != "A告警" || evs[1].AlarmName != "B告警" {
		t.Fatalf("order = [%s, %s], want [A告警, B告警]", evs[0].AlarmName, evs[1].AlarmName)
	}
	if evs[1].Severity != "L" {
		t.Fatalf("severity = %s, want L", evs[1].Severity)
	}
	if len(sink.Snapshot()) != 2 {
		t.Fatalf("sink = %d events", len(sink.Snapshot()))
	}
}

func TestLoadSnapshotSurfacesUnsupportedTypes(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, _ := newTestExecutor(t, clock)
	graph := `{"nodes":[{"id":"t","type":"trigger.telemetry"},{"id":"m","type":"transform.mapping","config":{}},{"id":"a","type":"action.alarm","config":{"name":"x"}}],"edges":[{"from":"t","to":"m"},{"from":"m","to":"a"}]}`
	res := x.LoadSnapshot(loadEnvelope("chain-1", 1, graph))
	if res.Status != LoadStatusLoaded {
		t.Fatalf("load = %+v, want loaded", res)
	}
	if len(res.UnsupportedTypes) != 1 || res.UnsupportedTypes[0] != "transform.mapping" {
		t.Fatalf("unsupported = %v, want [transform.mapping]", res.UnsupportedTypes)
	}
}

func TestProcessTelemetryEmptyValues(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	x, sink := newTestExecutor(t, clock)
	if res := x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph)); res.Status != LoadStatusLoaded {
		t.Fatalf("load = %+v", res)
	}
	if evs := x.ProcessTelemetry("dev-1", nil, clock.current); evs != nil {
		t.Fatalf("nil values should short-circuit, got %+v", evs)
	}
	if len(sink.Snapshot()) != 0 {
		t.Fatal("sink should be empty")
	}
}

// failingSink 记录落地失败的 sink。
type failingSink struct{ calls int }

func (s *failingSink) RecordAlarm(event AlarmEvent) error {
	s.calls++
	return fmt.Errorf("disk full")
}

func TestSinkFailureDoesNotLoseEvent(t *testing.T) {
	clock := &testClock{current: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	sink := &failingSink{}
	x := NewExecutor(WithSink(sink), WithNow(clock.Now))
	if res := x.LoadSnapshot(loadEnvelope("chain-1", 1, highTempGraph)); res.Status != LoadStatusLoaded {
		t.Fatalf("load = %+v", res)
	}
	evs := x.ProcessTelemetry("dev-1", map[string]any{"temperature": 99}, clock.current)
	if len(evs) != 1 {
		t.Fatalf("event must still be returned on sink failure, got %+v", evs)
	}
	if sink.calls != 1 {
		t.Fatalf("sink calls = %d", sink.calls)
	}
}
