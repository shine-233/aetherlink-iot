// 文件用途：P1.5 Reconcile 编排的决策层——回答"这次该给这个节点下发什么"。
// 核心逻辑：先过节点级闸门（健康 / 版本兼容），再逐资源用修订号判定要不要下发。
// 关键注意事项：
//   - **节点级闸门优先**：节点不在线或版本不兼容时，逐个资源判定毫无意义——
//     发不出去或会把节点刷成砖。闸门不通过一律整体跳过，并给出原因。
//   - **unknown 一律 fail closed**：健康状态未知、云端修订号非法时不做乐观假设。
//     宁可少发一次，也不能在状态不明时覆盖边缘的当前内容。
//   - **边缘超前是人工事件**：不下发，也不静默当一致，必须暴露出来。
//   - **边缘从未上报（nil）不等于"已同步"**：它意味着不知道边缘有什么，
//     应当下发一次让它收敛，而不是跳过。
//   - 本文件只做决策、不落库不发消息，便于无环境验证。
package service

// EdgeReconcileAction 单个资源的处置动作。
type EdgeReconcileAction string

const (
	// EdgeReconcileSync 需要下发。
	EdgeReconcileSync EdgeReconcileAction = "sync"
	// EdgeReconcileSkip 无需下发（已一致，或被节点级闸门拦下）。
	EdgeReconcileSkip EdgeReconcileAction = "skip"
	// EdgeReconcileNeedsAttention 需要人工确认，绝不能自动处理。
	EdgeReconcileNeedsAttention EdgeReconcileAction = "needs_attention"
)

// EdgeReconcileResource 一个待比对的资源：边缘上报的修订号 vs 云端当前修订号。
// 两者都为指针，nil 表示"这一侧没有该信息"。
type EdgeReconcileResource struct {
	ResourceType  string
	ResourceID    string
	EdgeRevision  *int64
	CloudRevision *int64
}

// EdgeReconcileItem 单个资源的决策结果，附带原因便于运维归因。
type EdgeReconcileItem struct {
	ResourceType string
	ResourceID   string
	Action       EdgeReconcileAction
	Reason       string
}

// PlanEdgeReconcile 编排一次边缘同步：先过节点级闸门，再逐资源判定。
// health / versionOK 由 ClassifyEdgeNodeHealth、CheckEdgeVersionCompatibility 得出，
// 这里只消费结论，保持可测。
func PlanEdgeReconcile(health EdgeNodeHealth, versionOK bool, versionReason string, resources []EdgeReconcileResource) []EdgeReconcileItem {
	items := make([]EdgeReconcileItem, 0, len(resources))

	// 节点级闸门一：健康状态。unknown 与 offline 都不得下发——
	// 状态不明时下发可能覆盖边缘当前内容，而事后无法确认它到底收没收到。
	if health != EdgeNodeHealthOnline && health != EdgeNodeHealthDegraded {
		for _, res := range resources {
			items = append(items, EdgeReconcileItem{
				ResourceType: res.ResourceType,
				ResourceID:   res.ResourceID,
				Action:       EdgeReconcileSkip,
				Reason:       "node not reachable: health=" + string(health),
			})
		}
		return items
	}

	// 节点级闸门二：版本兼容。不兼容就整体拒绝，避免在边缘把节点刷成砖。
	if !versionOK {
		reason := versionReason
		if reason == "" {
			reason = "node version incompatible"
		}
		for _, res := range resources {
			items = append(items, EdgeReconcileItem{
				ResourceType: res.ResourceType,
				ResourceID:   res.ResourceID,
				Action:       EdgeReconcileSkip,
				Reason:       "version gate: " + reason,
			})
		}
		return items
	}

	for _, res := range resources {
		item := EdgeReconcileItem{ResourceType: res.ResourceType, ResourceID: res.ResourceID}
		switch {
		// 云端修订号缺失或非法：数据本身有问题，交给人工而不是猜一个值下发。
		case res.CloudRevision == nil || !IsValidEdgeSyncRevision(*res.CloudRevision):
			item.Action = EdgeReconcileNeedsAttention
			item.Reason = "cloud revision missing or invalid; refusing to guess"

		// 云端没问题、边缘没上报过：说明不知道边缘有什么，下发一次让它收敛。
		// 绝不能当作"已同步"跳过——那会让边缘永远停在旧版本。
		case res.EdgeRevision == nil:
			item.Action = EdgeReconcileSync
			item.Reason = "edge has never reported a revision; send to converge"

		default:
			switch ClassifyEdgeSyncState(res.EdgeRevision, res.CloudRevision) {
			case EdgeSyncStateBehind:
				item.Action = EdgeReconcileSync
				item.Reason = "edge is behind cloud"
			case EdgeSyncStateInSync:
				item.Action = EdgeReconcileSkip
				item.Reason = "already in sync"
			case EdgeSyncStateAhead:
				// 边缘比云端还新：串号、时钟错乱或数据被手工改过，人工确认。
				item.Action = EdgeReconcileNeedsAttention
				item.Reason = "edge revision is ahead of cloud; manual review required"
			default:
				item.Action = EdgeReconcileNeedsAttention
				item.Reason = "revision state unknown; refusing to act"
			}
		}
		items = append(items, item)
	}
	return items
}

// EdgeReconcileSyncCount 计划中真正需要下发的资源数，便于调用方快速判断有无动作。
func EdgeReconcileSyncCount(items []EdgeReconcileItem) int {
	count := 0
	for _, item := range items {
		if item.Action == EdgeReconcileSync {
			count++
		}
	}
	return count
}

// EdgeReconcileBlocked 是否存在需要人工介入的项。
// 调用方应当在有阻断项时停止自动下发——自动处理人工事件等于掩盖问题。
func EdgeReconcileBlocked(items []EdgeReconcileItem) bool {
	for _, item := range items {
		if item.Action == EdgeReconcileNeedsAttention {
			return true
		}
	}
	return false
}
