package service

import (
	"time"

	"aetherlink-iot/backend/internal/model"
)

const commandJobMaxDispatchAttempts = 3
const commandJobRetryBaseBackoffSeconds = 30

func applyFleetCommandJobDetailFailure(detail *model.CommandJobDetail, err error, now time.Time) {
	detail.Status = commandJobDetailStatusFailed
	detail.Reason = StringPtr(err.Error())
	detail.CanRetry = detail.DispatchAttempts < commandJobMaxDispatchAttempts
	if detail.CanRetry {
		retryAfter := commandJobNextRetryAfter(detail.DispatchAttempts, now)
		detail.NextRetryAfter = &retryAfter
		detail.Advice = StringPtr("Retry becomes available after a short backoff; review the failure reason before retrying.")
	} else {
		detail.NextRetryAfter = nil
		detail.Advice = StringPtr("Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.")
	}
	detail.DispatchLeaseToken = nil
	detail.DispatchLeaseUntil = nil
	detail.UpdatedAt = now
	detail.CompletedAt = &now
}

func applyFleetCommandJobDetailSuccess(detail *model.CommandJobDetail, tracking *CommandDeliveryTracking, now time.Time) {
	detail.Status = commandJobDetailStatusSubmitted
	detail.MessageID = StringPtr(tracking.MessageID)
	detail.LogRecorded = tracking.LogRecorded
	detail.Reason = nil
	detail.CanRetry = false
	detail.NextRetryAfter = nil
	detail.DispatchLeaseToken = nil
	detail.DispatchLeaseUntil = nil
	detail.UpdatedAt = now
	detail.SubmittedAt = &now
}

func commandJobNextRetryAfter(attempts int, now time.Time) time.Time {
	if attempts <= 0 {
		attempts = 1
	}
	backoffSeconds := commandJobRetryBaseBackoffSeconds * attempts
	return now.Add(time.Duration(backoffSeconds) * time.Second)
}

func commandJobDetailRetryState(detail *model.CommandJobDetail, now time.Time) string {
	if detail == nil || detail.Status != commandJobDetailStatusFailed {
		return "not_retryable"
	}
	if !detail.CanRetry {
		if detail.DispatchAttempts >= commandJobMaxDispatchAttempts {
			return "max_attempts_reached"
		}
		return "not_retryable"
	}
	if detail.NextRetryAfter != nil && detail.NextRetryAfter.After(now) {
		return "waiting_backoff"
	}
	return "retryable"
}

func buildFleetCommandJobProgressHealth(job *model.CommandJob, now time.Time) *model.FleetCommandJobProgressHealth {
	if job == nil {
		return nil
	}

	terminalCount := job.SubmittedCount + job.FailedCount + job.BlockedCount
	if terminalCount > job.RequestedCount {
		terminalCount = job.RequestedCount
	}
	pendingCount := job.RequestedCount - terminalCount
	if pendingCount < 0 {
		pendingCount = 0
	}

	elapsedSeconds := int64(0)
	if !job.CreatedAt.IsZero() {
		elapsedSeconds = int64(now.Sub(job.CreatedAt).Seconds())
		if elapsedSeconds < 0 {
			elapsedSeconds = 0
		}
	}
	remainingSeconds := int64(0)
	if job.TimeoutAt != nil {
		remainingSeconds = int64(job.TimeoutAt.Sub(now).Seconds())
	}

	state := commandJobProgressHealthState(job, pendingCount, remainingSeconds)
	return &model.FleetCommandJobProgressHealth{
		State:                   state,
		PendingCount:            pendingCount,
		TerminalCount:           terminalCount,
		ElapsedSeconds:          elapsedSeconds,
		TimeoutRemainingSeconds: remainingSeconds,
		NextAction:              commandJobProgressHealthNextAction(state),
	}
}

func commandJobProgressHealthState(job *model.CommandJob, pendingCount int, remainingSeconds int64) string {
	switch job.Status {
	case commandJobStatusScheduled:
		return "scheduled"
	case commandJobStatusCompleted:
		return "complete"
	case commandJobStatusFailed, commandJobStatusPartiallyFailed:
		return "needs_attention"
	case commandJobStatusCanceled:
		return "canceled"
	}
	if job.TimeoutAt != nil && remainingSeconds <= 0 {
		return "timed_out"
	}
	if pendingCount > 0 && job.TimeoutAt != nil && remainingSeconds <= 15 {
		return "timeout_risk"
	}
	if pendingCount > 0 {
		return "running"
	}
	if job.FailedCount > 0 || job.BlockedCount > 0 {
		return "needs_attention"
	}
	return "complete"
}

func commandJobProgressHealthNextAction(state string) string {
	switch state {
	case "scheduled":
		return "任务尚未到 scheduled_at；请保留自动恢复扫描，并在计划时间后确认任务进入运行态。"
	case "running":
		return "请保持自动刷新开启，并等待待完成设备进入终态。"
	case "timeout_risk":
		return "请重点观察；任务接近超时窗口。如果待完成行没有结束，请准备复核支持证据。"
	case "timed_out":
		return "请刷新任务、复核超时行，并结合重试或支持包证据后再重新发起。"
	case "needs_attention":
		return "重试或交给支持前，请先复核失败、阻断、缺日志或设备响应异常的行。"
	case "canceled":
		return "请保留已取消任务链接用于审计；如果操作仍需执行，请创建新的预览。"
	default:
		return "任务可以收口；请把链接和支持证据保留到客户记录中。"
	}
}
