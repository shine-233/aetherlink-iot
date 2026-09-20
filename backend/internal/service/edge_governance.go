// 文件用途：边缘运维治理决策层（ROADMAP P1.5）。
//
// 覆盖三件事：节点健康判定、边缘版本兼容性、同步冲突检测。
// 三者都是纯决策函数——不碰数据库、不发命令，便于定向验证，也便于
// 后续接进 Reconcile 时行为可预期。
//
// 关键注意事项：
//   - 一切无法判定的状态一律归入 unknown / 不兼容，**绝不乐观地当成健康或兼容**。
//     把"不知道"当成"没问题"，是边缘运维里最容易把节点刷成砖的一类错误。
//   - 冲突只做检测与上报，**绝不自动合并或自动覆盖**：边缘最终状态必须可预测。
package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
)

// 节点健康判定阈值。
const (
	edgeNodeDegradedAfter = 60 * time.Second
	edgeNodeOfflineAfter  = 5 * time.Minute
)

const (
	// edgeSyncStatusPending 在途状态（与 CreateEdgeSync 落库时使用的字面量一致）。
	edgeSyncStatusPending = "pending"
	// edgeSyncConflictScanLimit 冲突扫描窗口：只查最近的在途任务，避免全表扫描。
	edgeSyncConflictScanLimit = 50
)

// EdgeNodeHealth 边缘节点健康状态。
type EdgeNodeHealth string

const (
	EdgeNodeHealthOnline   EdgeNodeHealth = "online"
	EdgeNodeHealthDegraded EdgeNodeHealth = "degraded"
	EdgeNodeHealthOffline  EdgeNodeHealth = "offline"
	// EdgeNodeHealthUnknown 无法判定：从未上报过心跳，或心跳时间在未来（时钟异常）。
	EdgeNodeHealthUnknown EdgeNodeHealth = "unknown"
)

// ClassifyEdgeNodeHealth 按最后心跳时间判定节点健康。
// lastSeenAt 为 nil / 零值 → unknown（从未上报，不得当成在线）；
// 心跳时间晚于 now → unknown（时钟异常，不得当成刚上报过）。
func ClassifyEdgeNodeHealth(lastSeenAt *time.Time, now time.Time) EdgeNodeHealth {
	if lastSeenAt == nil || lastSeenAt.IsZero() {
		return EdgeNodeHealthUnknown
	}
	elapsed := now.Sub(*lastSeenAt)
	if elapsed < 0 {
		return EdgeNodeHealthUnknown
	}
	switch {
	case elapsed < edgeNodeDegradedAfter:
		return EdgeNodeHealthOnline
	case elapsed < edgeNodeOfflineAfter:
		return EdgeNodeHealthDegraded
	default:
		return EdgeNodeHealthOffline
	}
}

// CheckEdgeVersionCompatibility 判断边缘节点版本是否满足最低要求。
// 版本串为空或含非数值段一律判定不兼容并给出原因——无法判断兼容就不允许下发。
func CheckEdgeVersionCompatibility(nodeVersion, minVersion string) (bool, string) {
	node := strings.TrimSpace(nodeVersion)
	min := strings.TrimSpace(minVersion)
	if node == "" {
		return false, "edge node version is empty"
	}
	if min == "" {
		return false, "required minimum version is empty"
	}
	nodeParts, ok := parseVersionSegments(node)
	if !ok {
		return false, fmt.Sprintf("edge node version %q is not numeric dotted", node)
	}
	minParts, ok := parseVersionSegments(min)
	if !ok {
		return false, fmt.Sprintf("required minimum version %q is not numeric dotted", min)
	}
	if compareVersionSegments(nodeParts, minParts) < 0 {
		return false, fmt.Sprintf("edge node version %s is older than required %s", node, min)
	}
	return true, ""
}

// parseVersionSegments 解析点分数字版本，返回各段数值。
func parseVersionSegments(version string) ([]int, bool) {
	raw := strings.Split(strings.TrimSuffix(version, "+"), ".")
	segments := make([]int, 0, len(raw))
	for _, part := range raw {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, false
		}
		value, err := strconv.Atoi(trimmed)
		if err != nil || value < 0 {
			return nil, false
		}
		segments = append(segments, value)
	}
	if len(segments) == 0 {
		return nil, false
	}
	return segments, true
}

// compareVersionSegments 逐段比较，长度不同时按 0 补齐。
func compareVersionSegments(left, right []int) int {
	size := len(left)
	if len(right) > size {
		size = len(right)
	}
	for i := 0; i < size; i++ {
		var l, r int
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	}
	return 0
}

// EdgeSyncConflict 边缘同步冲突：同一资源已有一份在途快照，且内容与新快照不同。
// 命中后必须由人工决定，不允许自动覆盖——否则边缘最终状态取决于消息到达顺序。
type EdgeSyncConflict struct {
	ResourceType  string
	ResourceID    string
	PendingTaskID string
	Reason        string
}

// DetectEdgeSyncConflict 在既有在途任务中查找同一资源但内容不同的快照。
// newContent 为空表示新快照无内容（如纯 OTA 指令），此时只按资源 ID 判重。
func DetectEdgeSyncConflict(pending []*model.EdgeSyncTask, resourceType, resourceID, newContent string) *EdgeSyncConflict {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return nil
	}
	for _, task := range pending {
		if task == nil || task.ResourceID != resourceID {
			continue
		}
		if resourceType != "" && task.ResourceType != resourceType {
			continue
		}
		existing := edgeSyncPayloadContent(task.Payload)
		if existing == newContent {
			// 内容一致：幂等重发，不算冲突。
			continue
		}
		return &EdgeSyncConflict{
			ResourceType:  task.ResourceType,
			ResourceID:    resourceID,
			PendingTaskID: task.ID,
			Reason:        "another snapshot for this resource is still in flight with different content",
		}
	}
	return nil
}

// edgeSyncPayloadContent 取出任务快照里的业务内容用于比对；解析失败返回空串，
// 此时与新内容不一致会被判为冲突——宁可升级为人工确认，也不静默放行。
func edgeSyncPayloadContent(payload string) string {
	if strings.TrimSpace(payload) == "" {
		return ""
	}
	var parsed edgeSyncPayload
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return ""
	}
	if parsed.Content == nil {
		return ""
	}
	return string(parsed.Content)
}
