// 文件用途：edgerules 图解析单测（TB-21 scoped v1）——结构校验/配置校验/未知类型容忍。
// 核心逻辑：正例（最小可用图）+ 负例矩阵（空图/坏 JSON/重复 ID/悬空边/自环/成环/超限/
//
//	阈值缺 key/op/value/坏 op/NaN/告警缺 name/坏 severity/负窗口）+ 未知类型只记录不拒收。
//
// 关键注意事项：期望语义与云端 rule_chain_graph.go 对齐；本文件不依赖 DB/broker。
package edgerules

import (
	"strings"
	"testing"
)

// validGraph 最小可用链：遥测触发 → 阈值 → 告警。
const validGraph = `{
  "nodes": [
    {"id": "t", "type": "trigger.telemetry"},
    {"id": "f", "type": "filter.threshold", "config": {"key": "temperature", "op": ">", "value": 80}},
    {"id": "a", "type": "action.alarm", "config": {"name": "高温告警", "severity": "H"}}
  ],
  "edges": [
    {"from": "t", "to": "f"},
    {"from": "f", "to": "a"}
  ]
}`

func TestParseGraphValid(t *testing.T) {
	g, unsupported, err := ParseGraph([]byte(validGraph))
	if err != nil {
		t.Fatalf("ParseGraph(valid) error = %v", err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("ParseGraph(valid) unsupported = %v, want empty", unsupported)
	}
	if len(g.Nodes) != 3 || len(g.Edges) != 2 {
		t.Fatalf("ParseGraph(valid) nodes=%d edges=%d, want 3/2", len(g.Nodes), len(g.Edges))
	}
	if g.NodeByID("f") == nil || g.NodeByID("missing") != nil {
		t.Fatalf("NodeByID lookup broken")
	}
}

func TestParseGraphEmptyAndBadJSON(t *testing.T) {
	if _, _, err := ParseGraph([]byte("   ")); err == nil {
		t.Fatal("ParseGraph(blank) should fail")
	}
	if _, _, err := ParseGraph([]byte("{not-json")); err == nil {
		t.Fatal("ParseGraph(bad json) should fail")
	}
}

func TestParseGraphStructuralRejections(t *testing.T) {
	cases := []struct {
		name   string
		graph  string
		expect string
	}{
		{"empty nodes", `{"nodes":[],"edges":[]}`, "must not be empty"},
		{"duplicate id", `{"nodes":[{"id":"a","type":"trigger.telemetry"},{"id":"a","type":"action.alarm","config":{"name":"x"}}],"edges":[]}`, "duplicate node id"},
		{"dangling edge", `{"nodes":[{"id":"a","type":"trigger.telemetry"}],"edges":[{"from":"a","to":"ghost"}]}`, "references unknown node"},
		{"self loop", `{"nodes":[{"id":"a","type":"trigger.telemetry"}],"edges":[{"from":"a","to":"a"}]}`, "self-loop"},
		{"cycle", `{"nodes":[
			{"id":"t","type":"trigger.telemetry"},
			{"id":"f","type":"filter.threshold","config":{"key":"k","op":">","value":1}},
			{"id":"a","type":"action.alarm","config":{"name":"x"}}],
		 "edges":[{"from":"t","to":"f"},{"from":"f","to":"a"},{"from":"a","to":"f"}]}`, "cycle"},
		{"trigger not root", `{"nodes":[
			{"id":"t","type":"trigger.telemetry"},
			{"id":"f","type":"filter.threshold","config":{"key":"k","op":">","value":1}}],
		 "edges":[{"from":"f","to":"t"},{"from":"t","to":"f"}]}`, "must be a root"},
		{"alarm as root", `{"nodes":[
			{"id":"t","type":"trigger.telemetry"},
			{"id":"a","type":"action.alarm","config":{"name":"x"}}],
		 "edges":[]}`, "cannot be a root"},
		{"threshold as root", `{"nodes":[
			{"id":"t","type":"trigger.telemetry"},
			{"id":"f","type":"filter.threshold","config":{"key":"k","op":">","value":1}}],
		 "edges":[]}`, "cannot be a root"},
		{"bad edge kind", `{"nodes":[{"id":"a","type":"trigger.telemetry"},{"id":"b","type":"action.alarm","config":{"name":"x"}}],"edges":[{"from":"a","to":"b","kind":"weird"}]}`, "unknown kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ParseGraph([]byte(tc.graph))
			if err == nil {
				t.Fatalf("ParseGraph(%s) should fail", tc.name)
			}
			if tc.expect != "" && !strings.Contains(err.Error(), tc.expect) {
				t.Fatalf("ParseGraph(%s) error = %v, want contains %q", tc.name, err, tc.expect)
			}
		})
	}
}

func TestParseGraphThresholdConfigRejections(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
	}{
		{"missing key", `{"op":">","value":1}`},
		{"missing op", `{"key":"k","value":1}`},
		{"unsupported op", `{"key":"k","op":"~","value":1}`},
		{"value not number", `{"key":"k","op":">","value":"hot"}`},
		{"value NaN", `{"key":"k","op":">","value":"NaN"}`},
		{"value Inf", `{"key":"k","op":">","value":"Inf"}`},
		{"nil config", `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graph := `{"nodes":[{"id":"t","type":"trigger.telemetry"},{"id":"f","type":"filter.threshold","config":` + tc.cfg + `}],"edges":[{"from":"t","to":"f"}]}`
			_, _, err := ParseGraph([]byte(graph))
			if err == nil {
				t.Fatalf("threshold cfg %s should be rejected", tc.cfg)
			}
		})
	}
}

func TestParseGraphAlarmConfigRejections(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
	}{
		{"missing name", `{"severity":"H"}`},
		{"blank name", `{"name":"   "}`},
		{"bad severity", `{"name":"x","severity":"CRITICAL"}`},
		{"negative dedup", `{"name":"x","retrigger_dedup_ms":-1}`},
		{"nil config", `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graph := `{"nodes":[{"id":"t","type":"trigger.telemetry"},{"id":"a","type":"action.alarm","config":` + tc.cfg + `}],"edges":[{"from":"t","to":"a"}]}`
			_, _, err := ParseGraph([]byte(graph))
			if err == nil {
				t.Fatalf("alarm cfg %s should be rejected", tc.cfg)
			}
		})
	}
}

func TestParseGraphAlarmSeverityDefaultAndDedup(t *testing.T) {
	graph := `{"nodes":[{"id":"t","type":"trigger.telemetry"},{"id":"a","type":"action.alarm","config":{"name":"x","retrigger_dedup_ms":5000}}],"edges":[{"from":"t","to":"a"}]}`
	g, _, err := ParseGraph([]byte(graph))
	if err != nil {
		t.Fatalf("ParseGraph error = %v", err)
	}
	cfg, err := parseAlarmConfig(g.NodeByID("a").Config)
	if err != nil {
		t.Fatalf("parseAlarmConfig error = %v", err)
	}
	if cfg.Severity != "H" {
		t.Fatalf("default severity = %q, want H", cfg.Severity)
	}
	if cfg.RetriggerDedupMs != 5000 {
		t.Fatalf("dedup ms = %v, want 5000", cfg.RetriggerDedupMs)
	}
}

func TestParseGraphUnknownTypeTolerated(t *testing.T) {
	// 云端图可能带 28 种节点；边缘 v1 只认子集。未知类型不拒收，由求值期阻断。
	graph := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"m","type":"transform.mapping","config":{"fields":{"a":"b"}}},
		{"id":"f","type":"filter.threshold","config":{"key":"k","op":">","value":1}},
		{"id":"a","type":"action.alarm","config":{"name":"x"}}],
	 "edges":[{"from":"t","to":"m"},{"from":"m","to":"f"},{"from":"f","to":"a"}]}`
	g, unsupported, err := ParseGraph([]byte(graph))
	if err != nil {
		t.Fatalf("ParseGraph(unknown types) error = %v", err)
	}
	if len(unsupported) != 1 || unsupported[0] != "transform.mapping" {
		t.Fatalf("unsupported = %v, want [transform.mapping]", unsupported)
	}
	if len(g.Nodes) != 4 {
		t.Fatalf("nodes = %d, want 4", len(g.Nodes))
	}
}

func TestParseGraphSizeLimit(t *testing.T) {
	huge := `{"nodes":[{"id":"a","type":"trigger.telemetry","name":"` + strings.Repeat("x", MaxGraphBytes) + `"}],"edges":[]}`
	if _, _, err := ParseGraph([]byte(huge)); err == nil {
		t.Fatal("oversize graph should be rejected")
	}
}

func TestParseGraphNodeLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"nodes":[`)
	for i := 0; i <= MaxNodes; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"id":"n` + strings.Repeat("x", i+1) + `","type":"trigger.telemetry"}`)
	}
	sb.WriteString(`],"edges":[]}`)
	if _, _, err := ParseGraph([]byte(sb.String())); err == nil {
		t.Fatal("node count over limit should be rejected")
	}
}

func TestSupportedNodeTypes(t *testing.T) {
	got := SupportedNodeTypes()
	if len(got) != 3 {
		t.Fatalf("SupportedNodeTypes = %v, want 3 entries", got)
	}
	for _, tt := range got {
		if !IsSupportedNodeType(tt) {
			t.Fatalf("IsSupportedNodeType(%q) = false", tt)
		}
	}
	if IsSupportedNodeType("transform.mapping") {
		t.Fatal("transform.mapping should not be supported in v1")
	}
}
