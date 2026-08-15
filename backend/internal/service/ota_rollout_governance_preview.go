// 文件用途：把纯 OTA rollout 治理规划器接到真实 task 状态上（只读预览）。
// 核心逻辑：按 task id 读取一行 rollout task 的调度/限速/中止配置，再聚合其 detail 的
//
//	分状态计数，映射成 OTARolloutGovernanceInput 喂给纯规划器 PlanOTARolloutGovernance，
//	回报"下一步应做什么"。它是只读预览：不下发、不改任何 detail 行、不连 broker。
//
// 关键注意事项：真正的批量下发、DB 行状态推进、限速窗口滚动清零与设备回调对账仍属需运行时
//
//	（PG+broker+设备）验证的部分，不在此实现；这里只把"真实状态 → 治理决策"的读路径接通，
//	决策逻辑仍由纯函数 PlanOTARolloutGovernance 单一来源保证。
package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	utils "aetherlink-iot/backend/pkg/utils"
)

// PreviewRolloutGovernance 读取一个 OTA rollout task 的当前状态，聚合其 detail 分状态计数，
// 并用纯规划器推演下一步治理动作。它是只读的：不下发、不改行、不连 broker。
func (o *OTA) PreviewRolloutGovernance(taskID string, claims *utils.UserClaims) (*model.OTARolloutGovernanceDecision, error) {
	taskID = strings.TrimSpace(taskID)
	task, err := ensureOTATaskAccess(taskID, claims)
	if err != nil {
		return nil, err
	}

	counts, err := dal.CountOTAUpgradeTaskDetailStatuses(taskID)
	if err != nil {
		return nil, err
	}

	input := buildOTARolloutGovernanceInput(task, counts, time.Now().UTC())
	decision := PlanOTARolloutGovernance(input)
	return &decision, nil
}

// buildOTARolloutGovernanceInput 把持久化 task 行 + detail 分状态计数映射为规划器入参。
// upgrading = pushed(2) + upgrading(3)；canceled(6) 不计入总量，视为已从 rollout 移除。
func buildOTARolloutGovernanceInput(task *model.OtaUpgradeTask, counts map[int16]int, now time.Time) model.OTARolloutGovernanceInput {
	pending := counts[model.OtaUpgradeTaskDetailStatusPending]
	upgrading := counts[model.OtaUpgradeTaskDetailStatusPushed] + counts[model.OtaUpgradeTaskDetailStatusUpgrading]
	succeeded := counts[model.OtaUpgradeTaskDetailStatusSucceeded]
	failed := counts[model.OtaUpgradeTaskDetailStatusFailed]

	return model.OTARolloutGovernanceInput{
		Status:                  task.Status,
		Now:                     now,
		ScheduledAt:             task.ScheduledAt,
		TimeoutAt:               task.TimeoutAt,
		RolloutRatePerMinute:    task.RolloutRatePerMinute,
		AbortFailureRatePercent: task.AbortFailureRatePercent,
		RateWindowStartedAt:     task.RateWindowStartedAt,
		RateWindowDispatched:    task.RateWindowDispatched,
		PendingCount:            pending,
		UpgradingCount:          upgrading,
		SucceededCount:          succeeded,
		FailedCount:             failed,
	}
}
