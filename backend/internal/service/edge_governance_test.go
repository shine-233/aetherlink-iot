package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/stretchr/testify/require"
)

func TestClassifyEdgeNodeHealth(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time {
		v := now.Add(-d)
		return &v
	}
	future := now.Add(time.Hour)
	zero := time.Time{}

	cases := []struct {
		name     string
		lastSeen *time.Time
		want     EdgeNodeHealth
	}{
		{"从未上报", nil, EdgeNodeHealthUnknown},
		{"零值时间", &zero, EdgeNodeHealthUnknown},
		{"心跳在未来（时钟异常）", &future, EdgeNodeHealthUnknown},
		{"刚上报", at(10 * time.Second), EdgeNodeHealthOnline},
		{"接近降级阈值", at(edgeNodeDegradedAfter - time.Second), EdgeNodeHealthOnline},
		{"刚过降级阈值", at(edgeNodeDegradedAfter + time.Second), EdgeNodeHealthDegraded},
		{"接近离线阈值", at(edgeNodeOfflineAfter - time.Second), EdgeNodeHealthDegraded},
		{"超过离线阈值", at(edgeNodeOfflineAfter + time.Second), EdgeNodeHealthOffline},
		{"很久没上报", at(24 * time.Hour), EdgeNodeHealthOffline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ClassifyEdgeNodeHealth(tc.lastSeen, now))
		})
	}
}

// 时钟异常与"从未上报"都不得被乐观地当成健康。
func TestClassifyEdgeNodeHealthNeverOptimistic(t *testing.T) {
	now := time.Now()
	require.NotEqual(t, EdgeNodeHealthOnline, ClassifyEdgeNodeHealth(nil, now))
	future := now.Add(time.Minute)
	require.NotEqual(t, EdgeNodeHealthOnline, ClassifyEdgeNodeHealth(&future, now),
		"心跳时间在未来说明时钟异常，不得当成在线")
}

func TestCheckEdgeVersionCompatibility(t *testing.T) {
	cases := []struct {
		name    string
		node    string
		min     string
		wantOK  bool
		wantMsg string
	}{
		{"相等", "1.2.3", "1.2.3", true, ""},
		{"更高", "2.0.0", "1.9.9", true, ""},
		{"段数不同按零补齐", "1.2", "1.2.0", true, ""},
		{"更旧", "1.2.2", "1.2.3", false, "older than required"},
		{"主版本更旧", "1.9.9", "2.0.0", false, "older than required"},
		{"节点版本为空", "", "1.0.0", false, "empty"},
		{"最低版本为空", "1.0.0", "", false, "empty"},
		{"节点版本含非数值", "1.2.x", "1.0.0", false, "not numeric dotted"},
		{"最低版本含非数值", "1.0.0", "1.0.x", false, "not numeric dotted"},
		{"节点版本为字母", "abc", "1.0.0", false, "not numeric dotted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := CheckEdgeVersionCompatibility(tc.node, tc.min)
			require.Equal(t, tc.wantOK, ok, "reason=%s", reason)
			if tc.wantMsg == "" {
				require.Empty(t, reason)
			} else {
				require.Contains(t, reason, tc.wantMsg)
			}
		})
	}
}

func pendingTask(id, resourceID, content string) *model.EdgeSyncTask {
	payload := `{"type":"rule_chain","resource_id":"` + resourceID + `","content":` + content + `}`
	return &model.EdgeSyncTask{
		ID:           id,
		TenantID:     "t1",
		ResourceType: model.EdgeResourceRuleChain,
		ResourceID:   resourceID,
		Payload:      payload,
		Status:       edgeSyncStatusPending,
	}
}

// 同一资源内容不同的在途快照必须被判为冲突。
func TestDetectEdgeSyncConflictDifferentContent(t *testing.T) {
	pending := []*model.EdgeSyncTask{pendingTask("task-1", "res-1", `{"a":1}`)}
	conflict := DetectEdgeSyncConflict(pending, model.EdgeResourceRuleChain, "res-1", `{"a":2}`)
	require.NotNil(t, conflict, "内容不同的在途快照必须上报冲突，不得静默覆盖")
	require.Equal(t, "task-1", conflict.PendingTaskID)
	require.Equal(t, "res-1", conflict.ResourceID)
	require.Equal(t, model.EdgeResourceRuleChain, conflict.ResourceType)
	require.NotEmpty(t, conflict.Reason)
}

// 内容一致属幂等重发，不算冲突。
func TestDetectEdgeSyncConflictSameContentIsIdempotent(t *testing.T) {
	pending := []*model.EdgeSyncTask{pendingTask("task-1", "res-1", `{"a":1}`)}
	require.Nil(t, DetectEdgeSyncConflict(pending, model.EdgeResourceRuleChain, "res-1", `{"a":1}`),
		"内容相同的重发应视为幂等")
}

func TestDetectEdgeSyncConflictIgnoresOtherResources(t *testing.T) {
	pending := []*model.EdgeSyncTask{pendingTask("task-1", "res-other", `{"a":1}`)}
	require.Nil(t, DetectEdgeSyncConflict(pending, model.EdgeResourceRuleChain, "res-1", `{"a":2}`))
}

func TestDetectEdgeSyncConflictRespectsResourceType(t *testing.T) {
	pending := []*model.EdgeSyncTask{pendingTask("task-1", "res-1", `{"a":1}`)}
	require.Nil(t, DetectEdgeSyncConflict(pending, model.EdgeResourceDashboard, "res-1", `{"a":2}`),
		"不同资源类型不应互相判定冲突")
}

func TestDetectEdgeSyncConflictRequiresResourceID(t *testing.T) {
	pending := []*model.EdgeSyncTask{pendingTask("task-1", "res-1", `{"a":1}`)}
	require.Nil(t, DetectEdgeSyncConflict(pending, model.EdgeResourceRuleChain, "", `{"a":2}`))
}

// 在途快照解析失败时按"内容不同"处理：宁可升级为人工确认，也不静默放行。
func TestDetectEdgeSyncConflictUnparsablePendingIsConflict(t *testing.T) {
	pending := []*model.EdgeSyncTask{{
		ID:           "task-1",
		ResourceType: model.EdgeResourceRuleChain,
		ResourceID:   "res-1",
		Payload:      `not-json`,
		Status:       edgeSyncStatusPending,
	}}
	conflict := DetectEdgeSyncConflict(pending, model.EdgeResourceRuleChain, "res-1", `{"a":2}`)
	require.NotNil(t, conflict, "解析不了的在途快照必须升级为冲突，不得视为无冲突放行")
	require.Equal(t, "task-1", conflict.PendingTaskID)
}

func TestDetectEdgeSyncConflictSkipsNilTasks(t *testing.T) {
	pending := []*model.EdgeSyncTask{nil, pendingTask("task-1", "res-1", `{"a":1}`)}
	require.NotNil(t, DetectEdgeSyncConflict(pending, model.EdgeResourceRuleChain, "res-1", `{"a":2}`))
}

func TestEdgeSyncPayloadContent(t *testing.T) {
	require.Equal(t, `{"a":1}`, edgeSyncPayloadContent(pendingTask("t", "r", `{"a":1}`).Payload))
	require.Empty(t, edgeSyncPayloadContent(""))
	require.Empty(t, edgeSyncPayloadContent("bad json"))
	require.Empty(t, edgeSyncPayloadContent(`{"type":"ota"}`), "无 content 字段时返回空")
}
