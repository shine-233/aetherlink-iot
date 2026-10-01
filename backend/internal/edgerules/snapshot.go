// 文件用途：边缘侧解析 edge_sync 下发的快照信封（TB-21 scoped v1）——修订号提取与比对。
// 核心逻辑：信封形状对照 service/edge_sync.go 的 edgeSyncPayload（type/resource_id/name/
//
//	content/revision/version/generated_at），content 即 chain.Graph；
//	修订号比对给出 unchanged/regression/newer/invalid 四态，驱动执行器的重连去重与升级重装载。
//
// 关键注意事项：
//   - 语义对照 service/edge_sync_revision.go（云端权威实现）；刻意不 import service（依赖隔离）。
//   - 修订号 < 1 视为非法（0 表示"从未同步"）：边缘 fail-closed 拒收，宁可继续跑旧版并保持
//     behind 状态由云端重推，也不能拿着无法去重的快照反复执行。
//   - 只认 type=rule_chain 的快照；dashboard/ota 由边缘其他消费方处理，这里拒收不误吞。
package edgerules

import (
	"encoding/json"
	"fmt"
)

// EdgeResourceRuleChain 规则链快照类型值（对照 model.EdgeResourceRuleChain）。
const EdgeResourceRuleChain = "rule_chain"

// Snapshot edge_sync 下发的快照信封（只取边缘规则执行器关心的字段）。
type Snapshot struct {
	Type        string          `json:"type"` // 仅接受 rule_chain
	ResourceID  string          `json:"resource_id"`
	Name        string          `json:"name"`
	Content     json.RawMessage `json:"content,omitempty"` // chain.Graph 快照
	GeneratedAt string          `json:"generated_at"`
	Version     int             `json:"version"` // 快照结构版本（当前恒为 1）
	Revision    int64           `json:"revision"`
}

// MinimumRevision 有效修订号下界（对照 edge_sync_revision.go：0/负数=从未同步，不是合法版本号）。
const MinimumRevision int64 = 1

// ParseSnapshot 解析并校验快照信封：type 必须是 rule_chain、修订号必须 >=1、
// resource_id 非空、content 必须能解析为合法子集图。任一不满足即拒收（fail-closed）。
func ParseSnapshot(raw []byte) (*Snapshot, *Graph, []string, error) {
	if len(raw) == 0 {
		return nil, nil, nil, fmt.Errorf("snapshot payload is empty")
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, nil, nil, fmt.Errorf("snapshot is not valid json: %w", err)
	}
	if snap.Type != EdgeResourceRuleChain {
		return nil, nil, nil, fmt.Errorf("unsupported snapshot type %q (only %q)", snap.Type, EdgeResourceRuleChain)
	}
	if snap.ResourceID == "" {
		return nil, nil, nil, fmt.Errorf("snapshot resource_id is required")
	}
	if !IsValidRevision(snap.Revision) {
		return nil, nil, nil, fmt.Errorf("snapshot revision %d is not a valid version (must be >= %d)", snap.Revision, MinimumRevision)
	}
	graph, unsupported, err := ParseGraph(snap.Content)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("snapshot graph invalid: %w", err)
	}
	return &snap, graph, unsupported, nil
}

// IsValidRevision 修订号是否有效（对照 IsValidEdgeSyncRevision）。
func IsValidRevision(revision int64) bool {
	return revision >= MinimumRevision
}

// RevisionState 收到的快照修订号与边缘已装载修订号的比对结果。
type RevisionState string

const (
	// RevisionStateNewer 来了更高版本，应装载（升级重装载）。
	RevisionStateNewer RevisionState = "newer"
	// RevisionStateUnchanged 版本未变：重连/重放同一任务时去重跳过，不重复执行（本缺口核心语义）。
	RevisionStateUnchanged RevisionState = "unchanged"
	// RevisionStateRegression 版本回退（边缘超前于云端）：异常，跳过并暴露，不静默当一致
	//（对照云端 ahead 语义——串号/手工改数的征兆）。
	RevisionStateRegression RevisionState = "regression"
	// RevisionStateInvalid 非法修订号（<1）：fail-closed 拒收。
	RevisionStateInvalid RevisionState = "invalid"
)

// ClassifyRevision 比对入站修订号与当前装载修订号（current<=0 表示边缘尚未装载）。
func ClassifyRevision(current, incoming int64) RevisionState {
	if !IsValidRevision(incoming) {
		return RevisionStateInvalid
	}
	switch {
	case current <= 0:
		return RevisionStateNewer // 从未装载：任何合法版本都视为新内容
	case incoming > current:
		return RevisionStateNewer
	case incoming == current:
		return RevisionStateUnchanged
	default:
		return RevisionStateRegression
	}
}
