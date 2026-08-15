package service

import (
	"fmt"

	"aetherlink-iot/backend/internal/model"
)

func buildFleetCommandJobExecutionSummary(
	job *model.CommandJob,
	health *model.FleetCommandJobProgressHealth,
	retryCounts commandJobRetryPolicyCounts,
	logMissingCount int,
	audit *model.FleetCommandJobAuditSummary,
) *model.FleetCommandJobExecutionSummary {
	if job == nil {
		return nil
	}

	summary := &model.FleetCommandJobExecutionSummary{
		PathType:   commandJobExecutionPathType(job),
		PathLabel:  commandJobExecutionPathLabel(job),
		Decision:   "monitor",
		CanClose:   false,
		NextAction: "请刷新任务，直到每台设备都进入终态。",
		Evidence: []string{
			fmt.Sprintf("目标 %d 台，可执行 %d 台，阻断 %d 台", job.RequestedCount, job.EligibleCount, job.BlockedCount),
			fmt.Sprintf("已提交 %d 台，失败 %d 台", job.SubmittedCount, job.FailedCount),
		},
	}

	if health != nil {
		summary.Evidence = append(summary.Evidence, fmt.Sprintf("待完成 %d 台，已终态 %d 台", health.PendingCount, health.TerminalCount))
	}
	if retryCounts.Ready > 0 || retryCounts.Waiting > 0 || retryCounts.Exhausted > 0 {
		summary.Evidence = append(summary.Evidence, fmt.Sprintf("可重试 %d 台，等待 %d 台，耗尽 %d 台", retryCounts.Ready, retryCounts.Waiting, retryCounts.Exhausted))
	}
	if logMissingCount > 0 {
		summary.Evidence = append(summary.Evidence, fmt.Sprintf("%d 台设备缺少平台日志证据", logMissingCount))
	}
	if audit != nil && audit.LatestEventType != "" {
		summary.Evidence = append(summary.Evidence, "最新审计事件："+audit.LatestEventType)
	}
	summary.CloseBlockers = commandJobCloseBlockers(job, health, retryCounts, logMissingCount, audit)
	summary.CanClose = len(summary.CloseBlockers) == 0

	switch {
	case summary.CanClose:
		summary.Decision = "close"
		summary.NextAction = "请把任务链接保留到客户记录中；当前不需要操作员继续处理。"
	case commandJobAuditHasAmbiguousAck(audit):
		summary.Decision = "collect_evidence"
		summary.NextAction = "关闭或按模糊设备响应重试前，请先复核重复 message-id 和命令日志证据。"
	case job.Status == commandJobStatusCanceled:
		summary.Decision = "canceled"
		summary.NextAction = "请保留取消证据；如果操作仍需执行，请重新创建预览。"
	case job.Status == commandJobStatusScheduled:
		summary.Decision = "wait_schedule"
		summary.NextAction = "任务尚未到计划时间；请在 scheduled_at 后确认恢复扫描已将其激活。"
	case retryCounts.Ready > 0:
		summary.Decision = "retry"
		summary.NextAction = "请复核失败行，然后只重试已就绪设备。"
	case retryCounts.Waiting > 0:
		summary.Decision = "wait"
		summary.NextAction = "请等待重试窗口，到期后先刷新，再发起下一次重试。"
	case retryCounts.Exhausted > 0 || job.BlockedCount > 0:
		summary.Decision = "support"
		summary.NextAction = "请打开支持包，先处理阻断或重试耗尽设备，再重新发起。"
	case logMissingCount > 0:
		summary.Decision = "collect_evidence"
		summary.NextAction = "关闭客户交接前，请刷新命令日志和 broker 证据。"
	case health != nil && health.State == "timeout_risk":
		summary.Decision = "watch_timeout"
		summary.NextAction = "请重点观察待完成行；若超时到期，请收集支持证据。"
	case health != nil && health.State == "timed_out":
		summary.Decision = "support"
		summary.NextAction = "发送新批量命令前，请先复核超时行和支持证据。"
	}

	summary.Checklist = commandJobExecutionChecklist(job, health, retryCounts, logMissingCount, audit)
	return summary
}

func commandJobCloseBlockers(
	job *model.CommandJob,
	health *model.FleetCommandJobProgressHealth,
	retryCounts commandJobRetryPolicyCounts,
	logMissingCount int,
	audit *model.FleetCommandJobAuditSummary,
) []string {
	if job == nil {
		return []string{"任务记录缺失。"}
	}
	blockers := []string{}
	if job.Status != commandJobStatusCompleted {
		blockers = append(blockers, "任务尚未完成。")
	}
	if health != nil && health.PendingCount > 0 {
		blockers = append(blockers, fmt.Sprintf("%d 台设备仍在等待终态。", health.PendingCount))
	}
	if job.FailedCount > 0 || job.BlockedCount > 0 {
		blockers = append(blockers, fmt.Sprintf("%d 台失败、%d 台阻断设备仍需复核。", job.FailedCount, job.BlockedCount))
	}
	if retryCounts.Ready > 0 || retryCounts.Waiting > 0 || retryCounts.Exhausted > 0 {
		blockers = append(blockers, fmt.Sprintf("%d 台可重试、%d 台等待、%d 台重试耗尽设备需要决策。", retryCounts.Ready, retryCounts.Waiting, retryCounts.Exhausted))
	}
	if logMissingCount > 0 {
		blockers = append(blockers, fmt.Sprintf("%d 台设备缺少平台日志证据。", logMissingCount))
	}
	if audit == nil || audit.LatestEventType == "" {
		blockers = append(blockers, "审计回执尚未加载。")
	}
	if commandJobAuditHasAmbiguousAck(audit) {
		blockers = append(blockers, "最新设备响应未被应用，因为匹配到多条命令任务行；请复核重复 message-id 证据。")
	}
	return blockers
}

func commandJobExecutionChecklist(
	job *model.CommandJob,
	health *model.FleetCommandJobProgressHealth,
	retryCounts commandJobRetryPolicyCounts,
	logMissingCount int,
	audit *model.FleetCommandJobAuditSummary,
) []model.FleetCommandJobExecutionChecklistItem {
	if job == nil {
		return nil
	}

	items := []model.FleetCommandJobExecutionChecklistItem{
		{
			Key:    "scope",
			Label:  "确认目标范围",
			State:  "done",
			Detail: fmt.Sprintf("目标 %d 台，可执行 %d 台，阻断 %d 台", job.RequestedCount, job.EligibleCount, job.BlockedCount),
		},
	}

	if health != nil && health.PendingCount > 0 {
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "progress",
			Label:  "观察待完成设备",
			State:  "watch",
			Detail: fmt.Sprintf("%d 台设备仍需终态结果", health.PendingCount),
		})
	} else {
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "progress",
			Label:  "确认终态进度",
			State:  "done",
			Detail: fmt.Sprintf("已提交 %d 台，失败 %d 台", job.SubmittedCount, job.FailedCount),
		})
	}

	switch {
	case retryCounts.Ready > 0:
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "retry",
			Label:  "复核可重试设备",
			State:  "todo",
			Detail: fmt.Sprintf("%d 台设备现在可以重试", retryCounts.Ready),
		})
	case retryCounts.Waiting > 0:
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "retry",
			Label:  "等待重试窗口",
			State:  "watch",
			Detail: fmt.Sprintf("%d 台设备重试前仍在冷却", retryCounts.Waiting),
		})
	case retryCounts.Exhausted > 0:
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "retry",
			Label:  "升级处理重试耗尽设备",
			State:  "blocked",
			Detail: fmt.Sprintf("%d 台设备已达到重试上限", retryCounts.Exhausted),
		})
	}

	if logMissingCount > 0 {
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "logs",
			Label:  "收集命令日志证据",
			State:  "todo",
			Detail: fmt.Sprintf("%d 台设备缺少平台日志证据", logMissingCount),
		})
	}

	if audit != nil && audit.LatestEventType != "" {
		state := "done"
		label := "保留审计回执"
		if commandJobAuditHasAmbiguousAck(audit) {
			state = "blocked"
			label = "处理模糊设备响应"
		}
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "audit",
			Label:  label,
			State:  state,
			Detail: "最新事件：" + audit.LatestEventType,
		})
	} else {
		items = append(items, model.FleetCommandJobExecutionChecklistItem{
			Key:    "audit",
			Label:  "刷新审计回执",
			State:  "watch",
			Detail: "尚未加载近期审计事件",
		})
	}

	return items
}

func commandJobAuditHasAmbiguousAck(audit *model.FleetCommandJobAuditSummary) bool {
	return audit != nil && audit.LatestEventType == commandJobEventDeviceAckAmbiguous
}

func commandJobExecutionPathType(job *model.CommandJob) string {
	if job.ScopeType == fleetCommandScopeDeviceFilter || job.RequestedCount > 1 {
		return "fleet_job"
	}
	return "single_device_command"
}

func commandJobExecutionPathLabel(job *model.CommandJob) string {
	if job.ScopeType == fleetCommandScopeDeviceFilter {
		return "设备筛选批量任务"
	}
	if job.RequestedCount > 1 {
		return "已选设备批量任务"
	}
	return "单设备命令"
}
