package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func replayTestRecords(nodeID string, nodeType string) []RuleChainReplayRecord {
	return []RuleChainReplayRecord{{
		ExecID:   "e1",
		ChainID:  "c1",
		NodeID:   nodeID,
		NodeType: nodeType,
		Payload:  map[string]any{"temp": 1},
		Metadata: map[string]any{"keep": "v"},
	}}
}

func TestRuleChainReplayRequiresSourceExecutionID(t *testing.T) {
	graph := failureEdgeGraph(t, []string{"t", "main"}, "")
	_, err := ReplayRuleChainExecution(context.Background(), graph,
		replayTestRecords("main", RuleChainActionWebhook),
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"},
		RuleChainReplayOptions{})
	require.Error(t, err, "没有来源执行 ID 的回放无法审计，必须拒绝")
	require.Contains(t, err.Error(), "source execution id")
}

func TestRuleChainReplayRejectsEmptyRecords(t *testing.T) {
	graph := failureEdgeGraph(t, []string{"t", "main"}, "")
	_, err := ReplayRuleChainExecution(context.Background(), graph, nil,
		&RuleChainContext{}, RuleChainReplayOptions{ReplayOf: "e1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one record")
}

// 回放命中副作用节点且未显式确认时，一个节点都不许跑。
func TestRuleChainReplayRefusesSideEffectsWithoutConfirmation(t *testing.T) {
	calls := withRecordingWebhook(t, func(string) bool { return false })
	graph := failureEdgeGraph(t, []string{"t", "main"}, "")

	_, err := ReplayRuleChainExecution(context.Background(), graph,
		replayTestRecords("main", RuleChainActionWebhook),
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"},
		RuleChainReplayOptions{ReplayOf: "e1"})

	require.Error(t, err, "重放副作用必须显式确认")
	require.Contains(t, err.Error(), "side-effect")
	require.Contains(t, err.Error(), RuleChainActionWebhook)
	require.Empty(t, *calls, "闸门必须先拦住，绝不放行到一半才发现有副作用")
}

func TestRuleChainReplayRunsSideEffectsAfterConfirmation(t *testing.T) {
	calls := withRecordingWebhook(t, func(string) bool { return false })
	graph := failureEdgeGraph(t, []string{"t", "main"}, "")

	errs, err := ReplayRuleChainExecution(context.Background(), graph,
		replayTestRecords("main", RuleChainActionWebhook),
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"},
		RuleChainReplayOptions{ReplayOf: "e1", ConfirmSideEffects: true})

	require.NoError(t, err)
	require.Empty(t, errs)
	require.Equal(t, []string{"https://hooks.example.com/main"}, *calls)
}

// 无副作用节点不需要确认即可重放。
func TestRuleChainReplayAllowsNonSideEffectNodes(t *testing.T) {
	graph, err := ParseRuleChainGraph(`{
	  "nodes":[
	    {"id":"t","type":"trigger.telemetry"},
	    {"id":"map","type":"transform.mapping"}
	  ],
	  "edges":[{"from":"t","to":"map"}]
	}`)
	require.NoError(t, err)
	graph.ChainID = "c1"

	errs, err := ReplayRuleChainExecution(context.Background(), graph,
		replayTestRecords("map", RuleChainTransformMapping),
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"},
		RuleChainReplayOptions{ReplayOf: "e1", ConfirmSideEffects: false})

	require.NoError(t, err, "无副作用节点不应被闸门拦住")
	require.Empty(t, errs, "未确认副作用也应能重放无副作用节点")
}

// 节点类型漂移后，旧输入不再适用。
func TestRuleChainReplayRejectsNodeTypeDrift(t *testing.T) {
	calls := withRecordingWebhook(t, func(string) bool { return false })
	graph := failureEdgeGraph(t, []string{"t", "main"}, "")

	errs, err := ReplayRuleChainExecution(context.Background(), graph,
		replayTestRecords("main", RuleChainFilterThreshold), // 实际节点是 action.webhook
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"},
		RuleChainReplayOptions{ReplayOf: "e1", ConfirmSideEffects: true})

	require.NoError(t, err)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), "type changed")
	require.Empty(t, *calls, "类型漂移时不得用旧输入重跑")
}

func TestRuleChainReplayReportsMissingNode(t *testing.T) {
	graph := failureEdgeGraph(t, []string{"t", "main"}, "")
	errs, err := ReplayRuleChainExecution(context.Background(), graph,
		replayTestRecords("ghost", RuleChainActionWebhook),
		&RuleChainContext{},
		RuleChainReplayOptions{ReplayOf: "e1", ConfirmSideEffects: true})
	require.NoError(t, err)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), "no longer exists")
}

func TestRuleChainReplayMarkedMetadata(t *testing.T) {
	meta := replayMarkedMetadata(map[string]any{"keep": "v"}, "e1")
	require.Equal(t, true, meta[ruleChainMetaReplay])
	require.Equal(t, "e1", meta[ruleChainMetaReplayOf])
	require.Equal(t, "v", meta["keep"], "既有 metadata 不应被丢弃")
}

// recorder 未接线时旁路，不留存任何载荷。
func TestRuleChainReplayRecorderBypassWhenUnwired(t *testing.T) {
	require.Nil(t, ruleChainReplayRecorder)
	withRecordingWebhook(t, func(string) bool { return false })
	graph := failureEdgeGraph(t, []string{"t", "main"}, "")
	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"}, map[string]any{"temp": 1})
	require.Empty(t, errs, "未接线时不得改变执行行为")
}

func TestRuleChainReplayRecorderCapturesInput(t *testing.T) {
	var got []RuleChainReplayRecord
	ruleChainReplayRecorder = func(rec RuleChainReplayRecord) { got = append(got, rec) }
	t.Cleanup(func() { ruleChainReplayRecorder = nil })
	withRecordingWebhook(t, func(string) bool { return false })

	graph := failureEdgeGraph(t, []string{"t", "main"}, "")
	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{TenantID: "t1", DeviceID: "d1"}, map[string]any{"temp": 42})

	require.Empty(t, errs)
	require.Len(t, got, 2, "触发节点与动作节点各一条")
	require.Equal(t, "t", got[0].NodeID)
	require.Equal(t, "main", got[1].NodeID)
	require.Equal(t, RuleChainActionWebhook, got[1].NodeType)
	require.Equal(t, "c1", got[1].ChainID)
	require.Equal(t, 42, got[1].Payload["temp"], "回放必须能拿到当时的输入")
	require.True(t, got[1].Pass)
	require.False(t, got[1].At.IsZero())
}
