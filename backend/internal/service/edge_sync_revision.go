// 文件用途：P1.5 边缘同步修订号（revision）——回答"边缘到底拿到了哪一版"。
// 核心逻辑：为每个 (租户, 网关, 资源类型, 资源 ID) 维护单调递增的修订号，
//
//	并据此判定边缘与云端的同步状态（落后/一致/超前/未知）。
//
// 关键注意事项：
//   - **修订号与快照格式版本是两回事**：`edgeSyncPayload.Version` 表示"快照 JSON 的结构版本"
//     （当前恒为 1），而 `Revision` 表示"这个资源的第几版内容"。此前只有 Version，
//     恒为 1 意味着无从判断边缘已拿到哪一版，重连后无法按版本同步。
//   - **修订号从 1 开始**：0 与负数表示"从未同步"，不得当作有效版本号参与比较。
//   - **缺失或非法一律判 unknown（fail closed）**：无法判断时不能乐观地当成"已同步"，
//     否则断云重连后边缘会拿着旧内容继续跑，而云端以为它是最新的。
//   - **边缘超前于云端是异常**：必须暴露出来告警，不得静默当作一致——
//     那通常意味着串号、时钟错乱或数据被手工改过。
package service

import (
	"encoding/json"
	"errors"
	"math"

	"aetherlink-iot/backend/internal/model"
)

// errEdgeSyncRevisionOverflow 修订号无法继续递增（非法负值或已达上限）。
// 此时必须报错而不是回绕——回绕会让边缘误判自己拿到的是旧版，从而错过更新。
var errEdgeSyncRevisionOverflow = errors.New("edge sync revision cannot advance")

// EdgeSyncState 边缘与云端的同步状态判定结果。
type EdgeSyncState string

const (
	// EdgeSyncStateUnknown 无从判断（任一侧缺失或非法）。此时应触发全量同步，而非跳过。
	EdgeSyncStateUnknown EdgeSyncState = "unknown"
	// EdgeSyncStateBehind 边缘落后于云端，需要下发。
	EdgeSyncStateBehind EdgeSyncState = "behind"
	// EdgeSyncStateInSync 版本一致，无需下发。
	EdgeSyncStateInSync EdgeSyncState = "in_sync"
	// EdgeSyncStateAhead 边缘比云端还新，属异常，需人工确认。
	EdgeSyncStateAhead EdgeSyncState = "ahead"
)

// edgeSyncRevisionMinimum 有效修订号的下界。0 表示"从未同步"，不参与比较。
const edgeSyncRevisionMinimum int64 = 1

// edgeSyncRevisionScanLimit 推导修订号时回溯的历史任务上限。
// 有界是为了不把一次下发变成全表扫描；修订号由历史最大值推导，
// 因此回溯窗口内的最大值即当前版本，超出窗口的旧任务不影响正确性。
const edgeSyncRevisionScanLimit = 200

// IsValidEdgeSyncRevision 修订号是否有效：必须 >= 1。
// 0 与负数表示"从未同步"，是合法状态但不是合法版本号。
func IsValidEdgeSyncRevision(revision int64) bool {
	return revision >= edgeSyncRevisionMinimum
}

// ClassifyEdgeSyncState 比较边缘上报的修订号与云端当前修订号。
// 任一侧缺失（nil）或非法（< 1）一律返回 unknown——
// 宁可触发一次全量同步，也不能在无法判断时假装一致。
func ClassifyEdgeSyncState(edgeRevision, cloudRevision *int64) EdgeSyncState {
	if edgeRevision == nil || cloudRevision == nil {
		return EdgeSyncStateUnknown
	}
	if !IsValidEdgeSyncRevision(*edgeRevision) || !IsValidEdgeSyncRevision(*cloudRevision) {
		return EdgeSyncStateUnknown
	}
	switch {
	case *edgeRevision < *cloudRevision:
		return EdgeSyncStateBehind
	case *edgeRevision > *cloudRevision:
		return EdgeSyncStateAhead
	default:
		return EdgeSyncStateInSync
	}
}

// NextEdgeSyncRevision 由当前修订号推导下一个。单调递增，绝不回退。
// current 为 0 表示"从未同步"，返回 1；非法（负数）或已到上限则报错——
// 溢出后静默回绕会让边缘误判自己拿到的是旧版，属于必须拦下的情况。
func NextEdgeSyncRevision(current int64) (int64, error) {
	if current < 0 {
		return 0, errEdgeSyncRevisionOverflow
	}
	if current == math.MaxInt64 {
		return 0, errEdgeSyncRevisionOverflow
	}
	next := current + 1
	if next < edgeSyncRevisionMinimum {
		next = edgeSyncRevisionMinimum
	}
	return next, nil
}

// edgeSyncPayloadRevision 解析任务快照里的修订号。
// 解析失败或缺失返回 nil（不是 0）——调用方据此判 unknown，
// 而不是把"读不出来"当成"第 0 版已同步"。
func edgeSyncPayloadRevision(payload string) *int64 {
	if payload == "" {
		return nil
	}
	var parsed struct {
		Revision *int64 `json:"revision"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil
	}
	if parsed.Revision == nil || !IsValidEdgeSyncRevision(*parsed.Revision) {
		return nil
	}
	return parsed.Revision
}

// EdgeSyncRevisionFromHistory 从该资源的历史任务中推导下一个修订号：
// 取历史 payload 中的最大修订号 + 1；没有任何有效历史则从 1 开始。
// 历史任务只增不删，因此修订号天然单调；万一历史被清理导致取不到，
// 从 1 重新开始会让边缘收到一个"更旧"的版本号，
// 此时 ClassifyEdgeSyncState 会判为 ahead 而暴露出来，不会静默丢更新。
func EdgeSyncRevisionFromHistory(tasks []*model.EdgeSyncTask, resourceID string) int64 {
	var maxRevision int64
	for _, task := range tasks {
		if task == nil || task.ResourceID != resourceID {
			continue
		}
		revision := edgeSyncPayloadRevision(task.Payload)
		if revision == nil {
			continue
		}
		if *revision > maxRevision {
			maxRevision = *revision
		}
	}
	// 历史里没有有效修订号时从 1 起步。
	if maxRevision < edgeSyncRevisionMinimum {
		return edgeSyncRevisionMinimum
	}
	next, err := NextEdgeSyncRevision(maxRevision)
	if err != nil {
		// 到顶后无处可进：停在最大值，宁可让冲突闸门拦下也不能回绕。
		return maxRevision
	}
	return next
}
