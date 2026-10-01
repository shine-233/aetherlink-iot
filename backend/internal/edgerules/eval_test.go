// 文件用途：edgerules 求值单测（TB-21 scoped v1）——阈值六算子/缺失点位/未知类型阻断/条件留痕。
// 核心逻辑：Evaluate 从 trigger.telemetry 走图；阈值语义逐条对照云端 ruleChainFilterThreshold。
// 关键注意事项：菱形图节点只求值一次（visited）；failure 边 v1 不走。
package edgerules

import (
	"fmt"
	"testing"
)

// mustGraph 解析辅助：失败即 Fatal。
func mustGraph(t *testing.T, raw string) *Graph {
	t.Helper()
	g, _, err := ParseGraph([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGraph error = %v", err)
	}
	return g
}

// thresholdChain 阈值+告警单链（两个 %s 依次为 op 与 value）。
const thresholdChain = `{
  "nodes": [
    {"id": "t", "type": "trigger.telemetry"},
    {"id": "f", "type": "filter.threshold", "config": {"key": "temperature", "op": "%s", "value": %s}},
    {"id": "a", "type": "action.alarm", "config": {"name": "高温", "severity": "M"}}
  ],
  "edges": [{"from": "t", "to": "f"}, {"from": "f", "to": "a"}]
}`

// thresholdGraph 构造阈值链图（失败即 Fatal）。
func thresholdGraph(t *testing.T, op, value string) *Graph {
	t.Helper()
	return mustGraph(t, fmt.Sprintf(thresholdChain, op, value))
}

func TestEvaluateThresholdOperators(t *testing.T) {
	cases := []struct {
		op      string
		value   string
		telem   float64
		wantHit bool
	}{
		{">", "80", 80.5, true},
		{">", "80", 80, false},
		{">=", "80", 80, true},
		{">=", "80", 79.9, false},
		{"<", "80", 79.9, true},
		{"<", "80", 80, false},
		{"<=", "80", 80, true},
		{"<=", "80", 80.1, false},
		{"==", "80", 80, true},
		{"==", "80", 80.0001, false},
		{"!=", "80", 81, true},
		{"!=", "80", 80, false},
	}
	for _, tc := range cases {
		t.Run(tc.op+"_"+tc.value, func(t *testing.T) {
			g := thresholdGraph(t, tc.op, tc.value)
			result := Evaluate(g, map[string]any{"temperature": tc.telem})
			if got := len(result.Fired); got != boolToInt(tc.wantHit) {
				t.Fatalf("fired = %d, want hit=%v (result=%+v)", got, tc.wantHit, result)
			}
		})
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestEvaluateMissingKeyAndNonNumeric(t *testing.T) {
	g := thresholdGraph(t, ">", "80")
	for name, values := range map[string]map[string]any{
		"missing key":   {"humidity": 90},
		"non numeric":   {"temperature": "scorching"},
		"bool value":    {"temperature": true},
		"null value":    {"temperature": nil},
		"empty payload": {},
	} {
		t.Run(name, func(t *testing.T) {
			result := Evaluate(g, values)
			if len(result.Fired) != 0 {
				t.Fatalf("%s should not fire, got %+v", name, result.Fired)
			}
		})
	}
}

func TestEvaluateStringNumericTelemetry(t *testing.T) {
	// 对照云端 toFloat：可解析数字字符串参与比较。
	g := thresholdGraph(t, ">", "80")
	result := Evaluate(g, map[string]any{"temperature": "81.5"})
	if len(result.Fired) != 1 {
		t.Fatalf("string numeric should fire, got %+v", result)
	}
}

func TestEvaluateConditionTrace(t *testing.T) {
	g := thresholdGraph(t, ">", "80")
	result := Evaluate(g, map[string]any{"temperature": 91})
	if len(result.Fired) != 1 {
		t.Fatalf("fired = %+v", result.Fired)
	}
	hit := result.Fired[0]
	if hit.Condition != "temperature > 80" {
		t.Fatalf("condition = %q", hit.Condition)
	}
	if hit.Severity != "M" || hit.AlarmName != "高温" {
		t.Fatalf("hit = %+v", hit)
	}
	if v, ok := toFloat(hit.Values["temperature"]); !ok || v != 91.0 {
		t.Fatalf("values snapshot = %+v", hit.Values)
	}
}

func TestEvaluateUnsupportedNodeBlocksPath(t *testing.T) {
	graph := `{
	  "nodes": [
	    {"id": "t", "type": "trigger.telemetry"},
	    {"id": "m", "type": "transform.mapping", "config": {"fields": {"a": "b"}}},
	    {"id": "f", "type": "filter.threshold", "config": {"key": "temperature", "op": ">", "value": 80}},
	    {"id": "a", "type": "action.alarm", "config": {"name": "高温"}}
	  ],
	  "edges": [{"from": "t", "to": "m"}, {"from": "m", "to": "f"}, {"from": "f", "to": "a"}]
	}`
	g := mustGraph(t, graph)
	result := Evaluate(g, map[string]any{"temperature": 100})
	if len(result.Fired) != 0 {
		t.Fatalf("unsupported node must block alarm, got %+v", result.Fired)
	}
	if len(result.Blocked) != 1 || result.Blocked[0] != "m" {
		t.Fatalf("blocked = %v, want [m]", result.Blocked)
	}
	if len(result.Evaluated) != 1 || result.Evaluated[0] != "t" {
		t.Fatalf("evaluated = %v, want [t]", result.Evaluated)
	}
}

func TestEvaluateDeviceOnlineTriggerNotFired(t *testing.T) {
	// trigger.device_online 不在 v1：不作为遥测入口（ParseGraph 的 unsupported 清单暴露该事实），
	// 遥测求值不会经过它，其下游告警不触发。
	graph := `{
	  "nodes": [
	    {"id": "t", "type": "trigger.device_online"},
	    {"id": "a", "type": "action.alarm", "config": {"name": "上线"}}
	  ],
	  "edges": [{"from": "t", "to": "a"}]
	}`
	_, unsupported, err := ParseGraph([]byte(graph))
	if err != nil {
		t.Fatalf("ParseGraph error = %v", err)
	}
	found := false
	for _, tt := range unsupported {
		if tt == "trigger.device_online" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unsupported = %v, want contains trigger.device_online", unsupported)
	}
	g := mustGraph(t, graph)
	result := Evaluate(g, map[string]any{"temperature": 100})
	if len(result.Fired) != 0 {
		t.Fatalf("device_online chain must not fire on telemetry, got %+v", result.Fired)
	}
	if len(result.Evaluated) != 0 {
		t.Fatalf("nothing should be evaluated without telemetry trigger, got %v", result.Evaluated)
	}
}

func TestEvaluateDiamondEvaluatesOnce(t *testing.T) {
	// 菱形：t → f1/f2 → a。visited 保证告警只命中一次。
	graph := `{
	  "nodes": [
	    {"id": "t", "type": "trigger.telemetry"},
	    {"id": "f1", "type": "filter.threshold", "config": {"key": "temperature", "op": ">", "value": 10}},
	    {"id": "f2", "type": "filter.threshold", "config": {"key": "humidity", "op": "<", "value": 50}},
	    {"id": "a", "type": "action.alarm", "config": {"name": "双条件"}}
	  ],
	  "edges": [{"from": "t", "to": "f1"}, {"from": "t", "to": "f2"}, {"from": "f1", "to": "a"}, {"from": "f2", "to": "a"}]
	}`
	g := mustGraph(t, graph)
	result := Evaluate(g, map[string]any{"temperature": 20, "humidity": 30})
	if len(result.Fired) != 1 {
		t.Fatalf("diamond should fire once, got %d (%+v)", len(result.Fired), result)
	}
}

func TestEvaluateFailureEdgeNotWalked(t *testing.T) {
	graph := `{
	  "nodes": [
	    {"id": "t", "type": "trigger.telemetry"},
	    {"id": "f", "type": "filter.threshold", "config": {"key": "temperature", "op": ">", "value": 80}},
	    {"id": "a", "type": "action.alarm", "config": {"name": "高温"}}
	  ],
	  "edges": [{"from": "t", "to": "f"}, {"from": "f", "to": "a", "kind": "failure"}]
	}`
	g := mustGraph(t, graph)
	result := Evaluate(g, map[string]any{"temperature": 100})
	if len(result.Fired) != 0 {
		t.Fatalf("failure edge must not fire alarm in v1, got %+v", result.Fired)
	}
	// 告警节点只经 failure 边可达：结构校验通过（入度 1），求值永不到达。
	if len(result.Evaluated) != 2 || result.Evaluated[0] != "t" || result.Evaluated[1] != "f" {
		t.Fatalf("evaluated = %v, want [t f]", result.Evaluated)
	}
}

func TestEvaluateNilAndEmpty(t *testing.T) {
	if r := Evaluate(nil, map[string]any{"k": 1}); r == nil || len(r.Fired) != 0 {
		t.Fatal("nil graph should yield empty result")
	}
	g := mustGraph(t, validGraph)
	if r := Evaluate(g, nil); r == nil || len(r.Fired) != 0 {
		t.Fatal("nil values should yield empty result")
	}
}
