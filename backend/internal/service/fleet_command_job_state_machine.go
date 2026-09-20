// 文件用途：批次命令作业（OTA 等）状态机与暂停/恢复（ROADMAP P0.3）。
// 核心逻辑：集中声明合法状态转移，任何非法转移一律拒绝并留下审计事件，绝不静默成功。
// 关键注意事项：
//  1. 终态（completed/partially_failed/failed/canceled）不可再转移到任何状态。
//  2. paused 不会被 worker 领取（派发只取 running/scheduled），因此暂停即停止下发。
//  3. 暂停/恢复的用户语义事件是 paused/unpaused；commandJobEventResumed 属于
//     worker 故障恢复，两者不可混用，避免把"故障自愈"粉饰成"人工恢复"。
package service

import (
	"context"
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// commandJobTransitions 合法状态转移表（from -> 允许的 to 集合）。
var commandJobTransitions = map[string]map[string]bool{
	commandJobStatusScheduled: {
		commandJobStatusRunning:  true,
		commandJobStatusPaused:   true,
		commandJobStatusCanceled: true,
	},
	commandJobStatusRunning: {
		commandJobStatusPaused:          true,
		commandJobStatusCanceled:        true,
		commandJobStatusCompleted:       true,
		commandJobStatusPartiallyFailed: true,
		commandJobStatusFailed:          true,
	},
	commandJobStatusPaused: {
		commandJobStatusRunning:  true,
		commandJobStatusCanceled: true,
	},
	// 终态：空集合表示不可再转移。
	commandJobStatusCompleted:       {},
	commandJobStatusPartiallyFailed: {},
	commandJobStatusFailed:          {},
	commandJobStatusCanceled:        {},
}

// isLegalCommandJobTransition 判断状态转移是否合法。
func isLegalCommandJobTransition(from, to string) bool {
	allowed, ok := commandJobTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// isTerminalCommandJobStatus 判断是否为终态。
func isTerminalCommandJobStatus(status string) bool {
	allowed, ok := commandJobTransitions[status]
	return ok && len(allowed) == 0
}

// commandJobTransitionError 非法转移的统一错误。
func commandJobTransitionError(from, to string) error {
	return errcode.NewWithMessage(
		errcode.CodeOpDenied,
		fmt.Sprintf("command job transition %s -> %s is not allowed", from, to),
	)
}

// PauseFleetCommandJob 暂停批次：scheduled/running -> paused。
// 暂停会清除 next_dispatch_at，使 worker 不再领取该批次；已在途的下发不受影响。
func (c *CommandData) PauseFleetCommandJob(
	jobID string,
	claims *utils.UserClaims,
	includeRows ...bool,
) (*model.FleetCommandJobSubmitResult, error) {
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	if job.Status == commandJobStatusPaused {
		// 已是暂停态：幂等返回，不产生第二次暂停事件。
		if !fleetCommandJobShouldIncludeRows(includeRows) {
			return c.GetFleetCommandJobSummary(job.ID, claims)
		}
		return c.GetFleetCommandJob(job.ID, claims)
	}
	if !isLegalCommandJobTransition(job.Status, commandJobStatusPaused) {
		return nil, commandJobTransitionError(job.Status, commandJobStatusPaused)
	}

	job.Status = commandJobStatusPaused
	job.NextDispatchAt = nil
	if err := dal.UpdateCommandJob(job); err != nil {
		return nil, err
	}
	recordFleetCommandJobEvent(
		job.ID, claims.TenantID, nil, nil,
		commandJobEventPaused,
		"批次已暂停，worker 将停止下发",
	)
	if err := refreshCommandJobSummary(job); err != nil {
		return nil, err
	}
	if !fleetCommandJobShouldIncludeRows(includeRows) {
		return c.GetFleetCommandJobSummary(job.ID, claims)
	}
	return c.GetFleetCommandJob(job.ID, claims)
}

// ResumeFleetCommandJob 恢复批次：paused -> running，并立即触发一次派发。
// 只有 paused 可恢复；终态批次必须先重试或新建，不允许"复活"。
func (c *CommandData) ResumeFleetCommandJob(
	ctx context.Context,
	jobID, operatorID string,
	claims *utils.UserClaims,
	includeRows ...bool,
) (*model.FleetCommandJobSubmitResult, error) {
	_ = ctx
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	if job.Status != commandJobStatusPaused {
		if !fleetCommandJobShouldIncludeRows(includeRows) {
			return c.GetFleetCommandJobSummary(job.ID, claims)
		}
		return c.GetFleetCommandJob(job.ID, claims)
	}
	if !isLegalCommandJobTransition(job.Status, commandJobStatusRunning) {
		return nil, commandJobTransitionError(job.Status, commandJobStatusRunning)
	}

	now := time.Now().UTC()
	job.Status = commandJobStatusRunning
	job.NextDispatchAt = &now
	if err := dal.UpdateCommandJob(job); err != nil {
		return nil, err
	}
	recordFleetCommandJobEvent(
		job.ID, claims.TenantID, nil, nil,
		commandJobEventUnpaused,
		"批次已恢复，继续下发",
	)
	if err := refreshCommandJobSummary(job); err != nil {
		return nil, err
	}
	c.dispatchFleetCommandJob(job.ID, operatorID, claims)
	if !fleetCommandJobShouldIncludeRows(includeRows) {
		return c.GetFleetCommandJobSummary(job.ID, claims)
	}
	return c.GetFleetCommandJob(job.ID, claims)
}
