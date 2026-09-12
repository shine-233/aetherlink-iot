package service

import (
	"math"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"github.com/stretchr/testify/require"
)

func ptrInt64(v int64) *int64 { return &v }

// 核心不变式：无法判断时必须报 unknown，绝不能乐观地当成"已同步"。
// 否则断云重连后边缘拿着旧内容继续跑，云端却以为它是最新的。
func TestClassifyEdgeSyncStateFailsClosedOnUnknown(t *testing.T) {
	cases := []struct {
		name  string
		edge  *int64
		cloud *int64
		want  EdgeSyncState
	}{
		{"both nil", nil, nil, EdgeSyncStateUnknown},
		{"edge nil", nil, ptrInt64(1), EdgeSyncStateUnknown},
		{"cloud nil", ptrInt64(1), nil, EdgeSyncStateUnknown},
		{"edge zero (never synced)", ptrInt64(0), ptrInt64(1), EdgeSyncStateUnknown},
		{"cloud zero", ptrInt64(1), ptrInt64(0), EdgeSyncStateUnknown},
		{"edge negative", ptrInt64(-1), ptrInt64(1), EdgeSyncStateUnknown},
		{"behind", ptrInt64(1), ptrInt64(2), EdgeSyncStateBehind},
		{"in sync", ptrInt64(3), ptrInt64(3), EdgeSyncStateInSync},
		// 边缘比云端还新是异常，必须暴露出来告警，不得静默当作一致。
		{"ahead is anomalous", ptrInt64(5), ptrInt64(2), EdgeSyncStateAhead},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ClassifyEdgeSyncState(tc.edge, tc.cloud))
		})
	}
}

func TestNextEdgeSyncRevisionMonotonic(t *testing.T) {
	// 从未同步 -> 第 1 版
	next, err := NextEdgeSyncRevision(0)
	require.NoError(t, err)
	require.Equal(t, int64(1), next)

	// 逐档递增，绝不回退
	current := int64(0)
	for i := 1; i <= 5; i++ {
		got, err := NextEdgeSyncRevision(current)
		require.NoError(t, err)
		require.Equal(t, int64(i), got)
		current = got
	}
}

// 修订号绝不能回绕：回绕会让边缘误判自己拿到的是旧版，从而错过更新。
func TestNextEdgeSyncRevisionRefusesInvalidAndOverflow(t *testing.T) {
	_, err := NextEdgeSyncRevision(-1)
	require.Error(t, err, "negative revision must be rejected")

	_, err = NextEdgeSyncRevision(math.MaxInt64)
	require.Error(t, err, "overflow must be rejected, not wrapped")
}

func TestIsValidEdgeSyncRevision(t *testing.T) {
	require.True(t, IsValidEdgeSyncRevision(1))
	require.True(t, IsValidEdgeSyncRevision(100))
	require.False(t, IsValidEdgeSyncRevision(0), "0 means never synced, not a valid version")
	require.False(t, IsValidEdgeSyncRevision(-1))
}

func TestEdgeSyncPayloadRevisionParsing(t *testing.T) {
	// 正常解析
	require.Equal(t, int64(7), *edgeSyncPayloadRevision(`{"revision":7}`))

	// 读不出来一律返回 nil（不是 0），让调用方判 unknown，
	// 不能把"解析失败"当成"第 0 版已同步"。
	require.Nil(t, edgeSyncPayloadRevision(""))
	require.Nil(t, edgeSyncPayloadRevision("not-json"))
	require.Nil(t, edgeSyncPayloadRevision(`{"other":1}`))
	require.Nil(t, edgeSyncPayloadRevision(`{"revision":0}`), "0 is not a valid revision")
	require.Nil(t, edgeSyncPayloadRevision(`{"revision":-3}`))
}

// 修订号由历史最大值推导：只增不减，且只认同一 resource_id 的任务。
func TestEdgeSyncRevisionFromHistory(t *testing.T) {
	mk := func(resourceID, payload string) *model.EdgeSyncTask {
		return &model.EdgeSyncTask{ResourceID: resourceID, Payload: payload}
	}

	// 无历史 -> 从 1 起步
	require.Equal(t, int64(1), EdgeSyncRevisionFromHistory(nil, "r1"))
	require.Equal(t, int64(1), EdgeSyncRevisionFromHistory(
		[]*model.EdgeSyncTask{mk("r1", "not-json")}, "r1"))

	// 有历史 -> 最大值 + 1
	require.Equal(t, int64(4), EdgeSyncRevisionFromHistory([]*model.EdgeSyncTask{
		mk("r1", `{"revision":1}`),
		mk("r1", `{"revision":3}`),
		mk("r1", `{"revision":2}`),
	}, "r1"))

	// 只认同一 resource_id，别的资源再高也不影响
	require.Equal(t, int64(2), EdgeSyncRevisionFromHistory([]*model.EdgeSyncTask{
		mk("r1", `{"revision":1}`),
		mk("r2", `{"revision":99}`),
	}, "r1"))

	// nil 任务不导致 panic
	require.Equal(t, int64(2), EdgeSyncRevisionFromHistory(
		[]*model.EdgeSyncTask{nil, mk("r1", `{"revision":1}`)}, "r1"))
}
