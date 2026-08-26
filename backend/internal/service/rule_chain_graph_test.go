// 文件用途：规则链图模型校验回归测试（ROADMAP B2）。
// 核心逻辑：覆盖合法 DAG 通过、环检测（Kahn）、未知类型、边引用完整性、触发器缺失。
// 关键注意事项：新增节点类型时需同步扩展本文件的合法用例。
package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const validLinearGraph = `{
  "nodes": [
    {"id":"t1","type":"trigger.telemetry"},
    {"id":"f1","type":"filter.threshold","config":{"key":"temperature","op":">","value":30}},
    {"id":"a1","type":"action.webhook","config":{"url":"http://127.0.0.1:1/hook"}}
  ],
  "edges":[{"from":"t1","to":"f1"},{"from":"f1","to":"a1"}]
}`

func TestParseRuleChainGraphAcceptsValidDag(t *testing.T) {
	graph, err := ParseRuleChainGraph(validLinearGraph)
	require.NoError(t, err)
	require.Len(t, graph.Nodes, 3)
	require.Len(t, graph.Roots(), 1)
	require.Equal(t, "t1", graph.Roots()[0].ID)
	require.Len(t, graph.Successors("f1"), 1)
}

func TestParseRuleChainGraphDetectsCycle(t *testing.T) {
	graph := `{"nodes":[
		{"id":"t1","type":"trigger.telemetry"},
		{"id":"n2","type":"transform.mapping","config":{"fields":{"a":"b"}}},
		{"id":"n3","type":"transform.mapping","config":{"fields":{"b":"c"}}}
	],"edges":[
		{"from":"t1","to":"n2"},{"from":"n2","to":"n3"},{"from":"n3","to":"n2"}
	]}`
	_, err := ParseRuleChainGraph(graph)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "cycle"), err.Error())
}

func TestParseRuleChainGraphRejectsUnknownTypeAndBadEdges(t *testing.T) {
	_, err := ParseRuleChainGraph(`{"nodes":[{"id":"t1","type":"trigger.telepathy"}]}`)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "unknown type"), err.Error())

	_, err = ParseRuleChainGraph(`{"nodes":[{"id":"t1","type":"trigger.telemetry"}],"edges":[{"from":"t1","to":"ghost"}]}`)
	require.Error(t, err)

	_, err = ParseRuleChainGraph(`{"nodes":[{"id":"n1","type":"transform.mapping","config":{"fields":{"a":"b"}}}]}`)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "trigger"), err.Error())
}

func TestParseRuleChainGraphToleratesEmpty(t *testing.T) {
	_, err := ParseRuleChainGraph("   ")
	require.Error(t, err, "empty graph must fail (no trigger)")
}
