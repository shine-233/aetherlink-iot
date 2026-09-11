package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// failureEdgeNodeJSON 可选节点模板。只声明用例真正用到的节点，
// 否则未接入边的节点会因入度为 0 被判为非法根。
var failureEdgeNodeJSON = map[string]string{
	"t":    `{"id":"t","type":"trigger.telemetry"}`,
	"main": `{"id":"main","type":"action.webhook","config":{"url":"https://hooks.example.com/main"}}`,
	"ok":   `{"id":"ok","type":"action.webhook","config":{"url":"https://hooks.example.com/ok"}}`,
	"fb":   `{"id":"fb","type":"action.webhook","config":{"url":"https://hooks.example.com/fb"}}`,
}

// failureEdgeGraph 构造 触发 -> main，其余边由 edges 参数给出。
func failureEdgeGraph(t *testing.T, ids []string, edges string) *RuleChainGraph {
	t.Helper()
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		node, ok := failureEdgeNodeJSON[id]
		require.True(t, ok, "unknown test node id %q", id)
		parts = append(parts, node)
	}
	raw := fmt.Sprintf(`{"nodes":[%s],"edges":[{"from":"t","to":"main"}%s]}`,
		strings.Join(parts, ","), edges)
	graph, err := ParseRuleChainGraph(raw)
	require.NoError(t, err)
	graph.ChainID = "c1"
	return graph
}

// withRecordingWebhook 注入记录型 poster；failFor 决定该 URL 是否失败。
func withRecordingWebhook(t *testing.T, failFor func(url string) bool) *[]string {
	t.Helper()
	orig := ruleChainWebhookPoster
	calls := make([]string, 0, 4)
	ruleChainWebhookPoster = func(ctx context.Context, url string, body []byte) (*http.Response, error) {
		calls = append(calls, url)
		if failFor(url) {
			return nil, errors.New("boom")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	t.Cleanup(func() { ruleChainWebhookPoster = orig })
	return &calls
}

func TestRuleChainGraphRejectsUnknownEdgeKind(t *testing.T) {
	raw := `{"nodes":[
	  {"id":"t","type":"trigger.telemetry"},
	  {"id":"w","type":"action.webhook","config":{"url":"https://hooks.example.com/a"}}
	],"edges":[{"from":"t","to":"w","kind":"bogus"}]}`
	_, err := ParseRuleChainGraph(raw)
	require.Error(t, err, "非法边类型必须被拒绝，不得静默按 success 兜底")
	require.Contains(t, err.Error(), "unknown edge kind")
}

func TestRuleChainGraphAcceptsDeclaredEdgeKinds(t *testing.T) {
	for _, kind := range []string{"", RuleChainEdgeKindSuccess, RuleChainEdgeKindFailure} {
		edges := fmt.Sprintf(`,{"from":"main","to":"ok","kind":%q}`, kind)
		require.NotNil(t, failureEdgeGraph(t, []string{"t", "main", "ok"}, edges), "kind=%q 应被接受", kind)
	}
}

// 成功路径绝不经过 failure 边。
func TestRuleChainSuccessPathSkipsFailureEdges(t *testing.T) {
	calls := withRecordingWebhook(t, func(string) bool { return false })
	graph := failureEdgeGraph(t, []string{"t", "main", "ok", "fb"},
		`,{"from":"main","to":"ok"},{"from":"main","to":"fb","kind":"failure"}`)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"}, map[string]any{"temp": 1})

	require.Empty(t, errs)
	require.Equal(t, []string{
		"https://hooks.example.com/main",
		"https://hooks.example.com/ok",
	}, *calls, "成功时只应走成功边")
}

// 节点失败且有失败分支时，由失败分支接管，错误不再计入聚合 errs。
func TestRuleChainFailureEdgeTakesOverOnNodeError(t *testing.T) {
	calls := withRecordingWebhook(t, func(url string) bool {
		return strings.Contains(url, "/main")
	})
	graph := failureEdgeGraph(t, []string{"t", "main", "ok", "fb"},
		`,{"from":"main","to":"ok"},{"from":"main","to":"fb","kind":"failure"}`)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"}, map[string]any{"temp": 1})

	require.Empty(t, errs, "失败分支接管后不应再报聚合错误")
	require.Equal(t, []string{
		"https://hooks.example.com/main",
		"https://hooks.example.com/fb",
	}, *calls, "失败时应走失败边，不得走成功边")
}

// 失败分支自身失败时，其错误照常冒泡。
func TestRuleChainFailureBranchErrorSurfaces(t *testing.T) {
	calls := withRecordingWebhook(t, func(string) bool { return true })
	graph := failureEdgeGraph(t, []string{"t", "main", "fb"},
		`,{"from":"main","to":"fb","kind":"failure"}`)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"}, map[string]any{"temp": 1})

	require.Len(t, errs, 1, "失败分支自身失败必须冒泡")
	require.Contains(t, errs[0].Error(), "fb")
	require.Equal(t, []string{
		"https://hooks.example.com/main",
		"https://hooks.example.com/fb",
	}, *calls)
}

// 无失败分支时保持既有行为：错误计入 errs、下游不执行。
func TestRuleChainWithoutFailureEdgeKeepsError(t *testing.T) {
	calls := withRecordingWebhook(t, func(url string) bool {
		return strings.Contains(url, "/main")
	})
	graph := failureEdgeGraph(t, []string{"t", "main", "ok"}, `,{"from":"main","to":"ok"}`)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"}, map[string]any{"temp": 1})

	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), "main")
	require.Equal(t, []string{"https://hooks.example.com/main"}, *calls, "无失败分支时不应继续下游")
}

func TestRuleChainFailureMetadataCarriesFact(t *testing.T) {
	meta := ruleChainFailureMetadata(map[string]any{"keep": 1}, "n1", errors.New("boom"))
	require.Equal(t, "n1", meta[ruleChainMetaFailedNode])
	require.Equal(t, "boom", meta[ruleChainMetaError])
	require.Equal(t, 1, meta["keep"], "既有 metadata 不应被丢弃")
	require.Len(t, meta, 3)
}

// 存量 graph 没有 kind 字段，行为必须与改动前一致。
func TestRuleChainEdgeWithoutKindBehavesAsSuccess(t *testing.T) {
	calls := withRecordingWebhook(t, func(string) bool { return false })
	graph := failureEdgeGraph(t, []string{"t", "main", "ok"}, `,{"from":"main","to":"ok"}`)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"}, map[string]any{"temp": 1})

	require.Empty(t, errs)
	require.Equal(t, []string{
		"https://hooks.example.com/main",
		"https://hooks.example.com/ok",
	}, *calls, "无 kind 的边必须仍按成功边走")
}
