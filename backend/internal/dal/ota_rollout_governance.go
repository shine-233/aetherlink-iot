// 文件用途: OTA rollout 治理执行面所需的持久化操作（P0.3 灰度/金丝雀）。
// 核心逻辑: 按 task_id 读取/推进明细行与 rollout task 的治理状态，供服务层的规划器执行决策。
// 关键注意事项: ota_upgrade_task_details 表没有 tenant 列（与既有 CountOTAUpgradeTaskDetailStatuses
//   同理），租户隔离在服务层通过 ensureOTATaskAccess 先校验 task 归属后再调用本文件；
//   本文件每个查询都带 task_id 过滤，不接受无 task 限定条件的读取。

package dal

import (
	"context"
	"time"

	"aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
)

// OTARolloutPendingClaimLimit 单次治理最多领取的待下发明细行。
// 即便规划器给出更大的批次也收敛到此值：一次治理把整批 pending 全捞出内存，
// 会把"限速下发"变成"一次性全推"，金丝雀就失去意义了。
const OTARolloutPendingClaimLimit = 200

// tenant-scope: parent-owned (2026-09-11) - ota_upgrade_task_details has no tenant_id
// column. Tenancy is enforced one level up: the service calls ensureOTATaskAccess first
// and only then queries by task_id. Every query here is task-scoped, never unscoped.
func ListOTAUpgradablePendingDetails(taskID string, limit int) ([]*model.OtaUpgradeTaskDetail, error) {
	if limit <= 0 || limit > OTARolloutPendingClaimLimit {
		limit = OTARolloutPendingClaimLimit
	}
	d := query.OtaUpgradeTaskDetail
	// 明细行没有 created_at 可排序字段，按主键排序：稳定且可复现，
	// 同一批金丝雀设备在多次治理间的选取顺序一致，便于比对。
	return d.WithContext(context.Background()).
		Where(d.OtaUpgradeTaskID.Eq(taskID)).
		Where(d.Status.Eq(model.OtaUpgradeTaskDetailStatusPending)).
		Order(d.ID).
		Limit(limit).
		Find()
}

// tenant-scope: parent-owned (2026-09-11) - task-scoped, see ListOTAUpgradablePendingDetails.
func CountOTAUpgradablePendingDetails(taskID string) (int64, error) {
	d := query.OtaUpgradeTaskDetail
	return d.WithContext(context.Background()).
		Where(d.OtaUpgradeTaskID.Eq(taskID)).
		Where(d.Status.Eq(model.OtaUpgradeTaskDetailStatusPending)).
		Count()
}

// OTARolloutTaskPatch 对 rollout task 的一次治理写入。零值字段表示不修改。
type OTARolloutTaskPatch struct {
	Status              string
	StatusDescription   string
	RateWindowDispatched *int
	RateWindowStartedAt *time.Time
}

// tenant-scope: keyed by task primary id only. The rollout task row carries tenant_id and
// callers must have validated access before reaching this write.
func UpdateOTAUpgradeTaskRolloutState(taskID string, patch OTARolloutTaskPatch) (int64, error) {
	d := query.OtaUpgradeTask
	updates := map[string]interface{}{"updated_at": time.Now().UTC()}
	if patch.Status != "" {
		updates["status"] = patch.Status
	}
	if patch.StatusDescription != "" {
		updates["status_description"] = patch.StatusDescription
	}
	if patch.RateWindowDispatched != nil {
		updates["rate_window_dispatched"] = *patch.RateWindowDispatched
	}
	if patch.RateWindowStartedAt != nil {
		updates["rate_window_started_at"] = *patch.RateWindowStartedAt
	}
	result, err := d.WithContext(context.Background()).
		Where(d.ID.Eq(taskID)).
		Updates(updates)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// CancelOTAUpgradeTaskPendingDetails 把尚未下发的 pending 明细行置为 canceled。
// 用于中止（失败率超阈值/超时）：剩余设备绝不能再被放量，否则"中止"只是个标记。
// 返回受影响行数。
func CancelOTAUpgradeTaskPendingDetails(taskID string, reason string, now time.Time) (int64, error) {
	d := query.OtaUpgradeTaskDetail
	result, err := d.WithContext(context.Background()).
		Where(d.OtaUpgradeTaskID.Eq(taskID)).
		Where(d.Status.Eq(model.OtaUpgradeTaskDetailStatusPending)).
		Updates(map[string]interface{}{
			"status":             model.OtaUpgradeTaskDetailStatusCanceled,
			"status_description": reason,
			"updated_at":         now,
		})
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}
