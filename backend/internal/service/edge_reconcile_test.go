package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanEdgeReconcileNodeGateBlocksEverything(t *testing.T) {
	resources := []EdgeReconcileResource{
		{ResourceType: "rule_chain", ResourceID: "r1", EdgeRevision: ptrInt64(1), CloudRevision: ptrInt64(2)},
	}

	// 离线：发不出去，逐个资源判定没有意义
	items := PlanEdgeReconcile(EdgeNodeHealthOffline, true, "", resources)
	require.Len(t, items, 1)
	require.Equal(t, EdgeReconcileSkip, items[0].Action)
	require.Contains(t, items[0].Reason, "not reachable")
	require.Equal(t, 0, EdgeReconcileSyncCount(items))

	// unknown 同样不得下发：状态不明时覆盖边缘内容，事后无法确认它收没收到
	items = PlanEdgeReconcile(EdgeNodeHealthUnknown, true, "", resources)
	require.Equal(t, EdgeReconcileSkip, items[0].Action)
	require.Equal(t, 0, EdgeReconcileSyncCount(items))
}

func TestPlanEdgeReconcileVersionGateBlocksEverything(t *testing.T) {
	resources := []EdgeReconcileResource{
		{ResourceType: "rule_chain", ResourceID: "r1", EdgeRevision: ptrInt64(1), CloudRevision: ptrInt64(2)},
	}
	items := PlanEdgeReconcile(EdgeNodeHealthOnline, false, "node 1.0 < min 2.0", resources)
	require.Len(t, items, 1)
	require.Equal(t, EdgeReconcileSkip, items[0].Action)
	require.Contains(t, items[0].Reason, "version gate")
	require.Equal(t, 0, EdgeReconcileSyncCount(items))
}

func TestPlanEdgeReconcilePerResourceDecisions(t *testing.T) {
	resources := []EdgeReconcileResource{
		// 落后 -> 下发
		{ResourceType: "rule_chain", ResourceID: "behind", EdgeRevision: ptrInt64(1), CloudRevision: ptrInt64(3)},
		// 一致 -> 跳过
		{ResourceType: "rule_chain", ResourceID: "same", EdgeRevision: ptrInt64(2), CloudRevision: ptrInt64(2)},
		// 超前 -> 人工
		{ResourceType: "rule_chain", ResourceID: "ahead", EdgeRevision: ptrInt64(9), CloudRevision: ptrInt64(2)},
	}
	items := PlanEdgeReconcile(EdgeNodeHealthOnline, true, "", resources)
	require.Len(t, items, 3)

	byID := map[string]EdgeReconcileItem{}
	for _, item := range items {
		byID[item.ResourceID] = item
	}
	require.Equal(t, EdgeReconcileSync, byID["behind"].Action)
	require.Equal(t, EdgeReconcileSkip, byID["same"].Action)
	require.Equal(t, EdgeReconcileNeedsAttention, byID["ahead"].Action)

	require.Equal(t, 1, EdgeReconcileSyncCount(items))
	require.True(t, EdgeReconcileBlocked(items), "ahead-of-cloud must be a blocking human event")
}

// 边缘从未上报过修订号（nil）不等于"已同步"——那意味着不知道边缘有什么，
// 应当下发一次让它收敛。跳过它会让边缘永远停在旧版本。
func TestPlanEdgeReconcileNeverReportedSendsToConverge(t *testing.T) {
	resources := []EdgeReconcileResource{
		{ResourceType: "dashboard", ResourceID: "d1", EdgeRevision: nil, CloudRevision: ptrInt64(1)},
	}
	items := PlanEdgeReconcile(EdgeNodeHealthOnline, true, "", resources)
	require.Len(t, items, 1)
	require.Equal(t, EdgeReconcileSync, items[0].Action)
	require.Contains(t, items[0].Reason, "never reported")
	require.Equal(t, 1, EdgeReconcileSyncCount(items))
	require.False(t, EdgeReconcileBlocked(items))
}

// 云端修订号本身非法：数据有问题，交给人工而不是猜一个值下发。
func TestPlanEdgeReconcileInvalidCloudRevisionNeedsAttention(t *testing.T) {
	zero := int64(0)
	resources := []EdgeReconcileResource{
		{ResourceType: "dashboard", ResourceID: "d1", EdgeRevision: ptrInt64(1), CloudRevision: &zero},
		{ResourceType: "dashboard", ResourceID: "d2", EdgeRevision: ptrInt64(1), CloudRevision: nil},
	}
	items := PlanEdgeReconcile(EdgeNodeHealthOnline, true, "", resources)
	require.Len(t, items, 2)
	for _, item := range items {
		require.Equal(t, EdgeReconcileNeedsAttention, item.Action)
	}
	require.True(t, EdgeReconcileBlocked(items))
}

// 降级节点仍可下发（它还在，只是心跳慢），不得与 offline 混为一谈。
func TestPlanEdgeReconcileDegradedStillSyncs(t *testing.T) {
	resources := []EdgeReconcileResource{
		{ResourceType: "rule_chain", ResourceID: "r1", EdgeRevision: ptrInt64(1), CloudRevision: ptrInt64(2)},
	}
	items := PlanEdgeReconcile(EdgeNodeHealthDegraded, true, "", resources)
	require.Equal(t, EdgeReconcileSync, items[0].Action)
	require.Equal(t, 1, EdgeReconcileSyncCount(items))
}

func TestPlanEdgeReconcileEmptyResources(t *testing.T) {
	items := PlanEdgeReconcile(EdgeNodeHealthOnline, true, "", nil)
	require.Empty(t, items)
	require.Equal(t, 0, EdgeReconcileSyncCount(items))
	require.False(t, EdgeReconcileBlocked(items))
}
