// 文件用途：实现 OTA rollout 治理的纯规划逻辑（创建期调度 / 限速 / 中止 / 批量重试的决策）。
// 核心逻辑：给定一个 rollout task 的调度/限速/中止配置与聚合的 detail 状态计数,
//
//	推演"下一步应该做什么"（等待计划时间 / 按限速下发一批 / 保持在限速窗口 /
//	因失败率中止 / 超时 / 完成 / 其它保持）。它是纯函数,不落库、不连 broker、不下发。
//
// 关键注意事项：真正的批量下发、DB 行状态推进与设备回调对账属于需运行时(PG+broker+设备)
//
//	验证的部分,不在此实现;这里只保证"给定状态,治理决策正确且可离线单测"。
//
// 重构建议：接入 worker/dispatch 循环时,让循环复用本规划器,保持"rollout 治理决策单一来源"。
package service

import (
	"fmt"

	model "aetherlink-iot/backend/internal/model"
)

// OTA rollout 治理动作常量。
const (
	OTARolloutActionWaitSchedule   = "wait_schedule"
	OTARolloutActionDispatchBatch  = "dispatch_batch"
	OTARolloutActionHoldRateWindow = "hold_rate_window"
	OTARolloutActionAbort          = "abort"
	OTARolloutActionTimeout        = "timeout"
	OTARolloutActionComplete       = "complete"
	OTARolloutActionHold           = "hold"
)

// PlanOTARolloutGovernance 根据 rollout task 的当前状态规划下一步治理动作。
// 它是纯函数:不查询数据库、不连接 broker、不下发命令,只对传入的状态做决策。
// 决策优先级:终态/取消 > 超时 > 失败率中止 > 未到计划时间 > 无待下发(完成或等待升级)
//
//	> 限速窗口已满 > 按限速下发一批。
func PlanOTARolloutGovernance(in model.OTARolloutGovernanceInput) model.OTARolloutGovernanceDecision {
	decision := model.OTARolloutGovernanceDecision{
		Warnings:     []string{},
		NextSteps:    []string{},
		IsSimulation: true,
	}

	finished := in.SucceededCount + in.FailedCount
	decision.FailureRate = otaRolloutFailureRate(in.FailedCount, finished)

	// 已取消或已完成:不再规划任何下发。
	switch in.Status {
	case "canceled":
		decision.Action = OTARolloutActionHold
		decision.Reason = "rollout 已取消,不再调度下发"
		decision.NextSteps = append(decision.NextSteps, "如需继续,请新建 rollout 任务")
		return decision
	case "completed":
		decision.Action = OTARolloutActionComplete
		decision.Reason = "rollout 已标记完成"
		return decision
	}

	// 超时优先于其它下发决策:一旦越过绝对截止时间,应停止下发并收尾。
	if in.TimeoutAt != nil && !in.Now.Before(*in.TimeoutAt) {
		decision.Action = OTARolloutActionTimeout
		decision.RemainingInvalid = in.PendingCount + in.UpgradingCount
		decision.Reason = "已越过 rollout 绝对截止时间(timeout_at)"
		decision.NextSteps = append(decision.NextSteps,
			"将剩余 pending/upgrading 设备标记为超时并生成支持包",
			"确认设备连通性与包兼容性后,再新建 rollout 重试")
		return decision
	}

	// 失败率中止:仅在配置了阈值且已有足够样本(至少一台进入终态)时判断。
	if in.AbortFailureRatePercent != nil && finished > 0 && decision.FailureRate >= *in.AbortFailureRatePercent {
		decision.Action = OTARolloutActionAbort
		decision.RemainingInvalid = in.PendingCount
		decision.Reason = fmt.Sprintf("失败率 %.2f%% 已达到中止阈值 %.2f%%", decision.FailureRate, *in.AbortFailureRatePercent)
		decision.NextSteps = append(decision.NextSteps,
			"停止对剩余 pending 设备的下发",
			"按失败原因分组排查代表性设备,确认包兼容性后再决定是否重试")
		return decision
	}

	// 未到计划开始时间:等待调度。
	if in.ScheduledAt != nil && in.Now.Before(*in.ScheduledAt) {
		decision.Action = OTARolloutActionWaitSchedule
		decision.Reason = fmt.Sprintf("尚未到计划开始时间 %s", in.ScheduledAt.UTC().Format("2006-01-02T15:04:05Z07:00"))
		decision.NextSteps = append(decision.NextSteps, "到计划时间后再评估下发")
		return decision
	}

	// 没有待下发设备:要么全部收尾完成,要么仍在等待升级中的设备回调。
	if in.PendingCount == 0 {
		if in.UpgradingCount == 0 {
			decision.Action = OTARolloutActionComplete
			decision.Reason = "没有待下发设备,且没有升级中的设备"
			if in.FailedCount > 0 {
				decision.Warnings = append(decision.Warnings, fmt.Sprintf("%d 台设备升级失败,建议生成支持包复盘", in.FailedCount))
			}
			return decision
		}
		decision.Action = OTARolloutActionHold
		decision.Reason = fmt.Sprintf("没有待下发设备,等待 %d 台升级中的设备回调", in.UpgradingCount)
		decision.NextSteps = append(decision.NextSteps, "等待设备上报升级结果后再评估")
		return decision
	}

	// 限速:每分钟最多 RolloutRatePerMinute 台。若本窗口已下发达到上限,则保持等待下一窗口。
	rate := in.RolloutRatePerMinute
	if rate <= 0 {
		decision.Warnings = append(decision.Warnings, "rollout_rate_per_minute 非正数,已回退为每分钟 1 台的保守下发")
		rate = 1
	}
	remainingInWindow := rate - in.RateWindowDispatched
	if remainingInWindow <= 0 {
		decision.Action = OTARolloutActionHoldRateWindow
		decision.Reason = fmt.Sprintf("当前限速窗口已下发 %d/%d 台,等待下一窗口", in.RateWindowDispatched, rate)
		decision.NextSteps = append(decision.NextSteps, "限速窗口滚动后再下发下一批")
		return decision
	}

	batch := remainingInWindow
	if batch > in.PendingCount {
		batch = in.PendingCount
	}
	decision.Action = OTARolloutActionDispatchBatch
	decision.BatchSize = batch
	decision.Reason = fmt.Sprintf("在限速窗口内下发 %d 台(窗口剩余 %d,待下发 %d)", batch, remainingInWindow, in.PendingCount)
	decision.NextSteps = append(decision.NextSteps,
		"下发后累加限速窗口计数,并在窗口滚动时清零",
		"设备回调更新 detail 状态后重新评估下一批")
	return decision
}

// otaRolloutFailureRate 计算失败率(百分比),分母为已进入终态的设备数(成功+失败)。
func otaRolloutFailureRate(failed int, finished int) float64 {
	if finished <= 0 {
		return 0
	}
	return float64(failed) / float64(finished) * 100
}
