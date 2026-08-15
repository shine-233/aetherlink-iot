package service

import (
	"fmt"

	"aetherlink-iot/backend/internal/model"
)

func buildFleetCommandJobPreviewGovernanceSummary(result *model.FleetCommandJobPreviewResult) *model.FleetCommandJobGovernanceSummary {
	if result == nil {
		return nil
	}

	level := "success"
	switch {
	case result.EligibleCount <= 0 || result.BlockedCount >= result.RequestedCount:
		level = "error"
	case result.BlockedCount > 0 || result.RequestedCount > len(result.Rows) || result.TimeoutSeconds < 60:
		level = "warning"
	}

	summary := &model.FleetCommandJobGovernanceSummary{
		Level:      level,
		Title:      "提交前治理检查",
		Summary:    fmt.Sprintf("目标 %d 台，可执行 %d 台，下发前阻断 %d 台。", result.RequestedCount, result.EligibleCount, result.BlockedCount),
		NextAction: result.NextAction,
		Items: []model.FleetCommandJobGovernanceItem{
			{
				Key:    "scope",
				Label:  "目标范围",
				Value:  fmt.Sprintf("目标 %d 台 / 可执行 %d 台", result.RequestedCount, result.EligibleCount),
				State:  commandJobGovernanceState(result.EligibleCount > 0 && result.BlockedCount == 0, result.EligibleCount <= 0),
				Detail: fmt.Sprintf("提交前需要复核 %d 台被阻断设备。", result.BlockedCount),
			},
			{
				Key:    "preview_coverage",
				Label:  "预览覆盖",
				Value:  fmt.Sprintf("已展示 %d 行", len(result.Rows)),
				State:  commandJobGovernanceState(result.RequestedCount <= len(result.Rows), false),
				Detail: "筛选批量任务应在预览覆盖已确认范围后再提交。",
			},
			{
				Key:    "timeout",
				Label:  "超时窗口",
				Value:  fmt.Sprintf("%d 秒", result.TimeoutSeconds),
				State:  commandJobGovernanceState(result.TimeoutSeconds >= 60, result.TimeoutSeconds <= 0),
				Detail: "短超时适合即时检查，但对大批量或离线设备风险较高。",
			},
			{
				Key:    "retry_policy",
				Label:  "重试策略",
				Value:  fmt.Sprintf("最多下发 %d 次", commandJobMaxDispatchAttempts),
				State:  "done",
				Detail: "提交后会跟踪可重试、等待重试和重试耗尽状态。",
			},
		},
	}

	if summary.NextAction == "" {
		summary.NextAction = "请先复核治理项，再提交可执行设备并观察进度。"
	}
	summary.Items = append(summary.Items, fleetCommandJobDispatchGovernanceItem())
	return summary
}

func buildFleetCommandJobGovernanceSummary(
	job *model.CommandJob,
	health *model.FleetCommandJobProgressHealth,
	retryCounts commandJobRetryPolicyCounts,
	logMissingCount int,
	audit *model.FleetCommandJobAuditSummary,
) *model.FleetCommandJobGovernanceSummary {
	if job == nil {
		return nil
	}

	pendingCount := 0
	if health != nil {
		pendingCount = health.PendingCount
	}
	level := "success"
	switch {
	case job.BlockedCount > 0 || retryCounts.Exhausted > 0:
		level = "error"
	case job.FailedCount > 0 || retryCounts.Ready > 0 || retryCounts.Waiting > 0 || logMissingCount > 0:
		level = "warning"
	case pendingCount > 0 || job.Status == commandJobStatusRunning:
		level = "info"
	}

	summary := &model.FleetCommandJobGovernanceSummary{
		Level:      level,
		Title:      "任务治理检查",
		Summary:    fmt.Sprintf("目标 %d 台，已提交 %d 台，失败 %d 台，阻断 %d 台。", job.RequestedCount, job.SubmittedCount, job.FailedCount, job.BlockedCount),
		NextAction: commandJobGovernanceNextAction(job, health, retryCounts, logMissingCount),
		Items: []model.FleetCommandJobGovernanceItem{
			{
				Key:    "scope",
				Label:  "目标范围",
				Value:  fmt.Sprintf("目标 %d 台 / 可执行 %d 台", job.RequestedCount, job.EligibleCount),
				State:  commandJobGovernanceState(job.EligibleCount > 0 && job.BlockedCount == 0, job.EligibleCount <= 0 || job.BlockedCount > 0),
				Detail: fmt.Sprintf("任务证据中仍有 %d 台被阻断设备。", job.BlockedCount),
			},
			{
				Key:    "progress",
				Label:  "进度控制",
				Value:  fmt.Sprintf("待完成 %d 台 / 已终态 %d 台", pendingCount, commandJobGovernanceTerminalCount(job, health)),
				State:  commandJobGovernanceState(pendingCount == 0, false),
				Detail: "请持续刷新，直到每一行进入终态或记录超时证据。",
			},
			{
				Key:    "timeout",
				Label:  "超时窗口",
				Value:  commandJobGovernanceTimeoutValue(job, health),
				State:  commandJobGovernanceTimeoutState(health),
				Detail: "超时证据用于区分正常等待和需要升级支持的情况。",
			},
			{
				Key:    "retry_policy",
				Label:  "重试策略",
				Value:  fmt.Sprintf("可重试 %d 台 / 等待 %d 台 / 耗尽 %d 台", retryCounts.Ready, retryCounts.Waiting, retryCounts.Exhausted),
				State:  commandJobGovernanceRetryState(retryCounts),
				Detail: fmt.Sprintf("每行最多可下发 %d 次，超过后需要支持复核。", commandJobMaxDispatchAttempts),
			},
			{
				Key:    "evidence",
				Label:  "证据交接",
				Value:  fmt.Sprintf("缺少日志 %d 条", logMissingCount),
				State:  commandJobGovernanceState(logMissingCount == 0 && audit != nil && audit.LatestEventType != "", logMissingCount > 0),
				Detail: commandJobGovernanceEvidenceDetail(audit),
			},
		},
	}
	summary.Items = append(summary.Items, fleetCommandJobDispatchGovernanceItem())
	return summary
}

func commandJobGovernanceState(ok bool, blocked bool) string {
	if blocked {
		return "blocked"
	}
	if ok {
		return "done"
	}
	return "watch"
}

func commandJobGovernanceTerminalCount(job *model.CommandJob, health *model.FleetCommandJobProgressHealth) int {
	if health != nil {
		return health.TerminalCount
	}
	return job.SubmittedCount + job.FailedCount + job.BlockedCount
}

func commandJobGovernanceTimeoutValue(job *model.CommandJob, health *model.FleetCommandJobProgressHealth) string {
	if health != nil {
		return fmt.Sprintf("剩余 %d 秒", health.TimeoutRemainingSeconds)
	}
	return fmt.Sprintf("%d 秒", job.TimeoutSeconds)
}

func commandJobGovernanceTimeoutState(health *model.FleetCommandJobProgressHealth) string {
	if health == nil {
		return "watch"
	}
	switch health.State {
	case "timed_out", "needs_attention":
		return "blocked"
	case "timeout_risk":
		return "watch"
	case "complete":
		return "done"
	default:
		return "watch"
	}
}

func commandJobGovernanceRetryState(retryCounts commandJobRetryPolicyCounts) string {
	switch {
	case retryCounts.Exhausted > 0:
		return "blocked"
	case retryCounts.Ready > 0 || retryCounts.Waiting > 0:
		return "watch"
	default:
		return "done"
	}
}

func commandJobGovernanceEvidenceDetail(audit *model.FleetCommandJobAuditSummary) string {
	if audit != nil && audit.LatestEventType != "" {
		return "最新审计事件：" + audit.LatestEventType
	}
	return "关闭客户交接前，请刷新任务或支持包。"
}

func commandJobGovernanceNextAction(
	job *model.CommandJob,
	health *model.FleetCommandJobProgressHealth,
	retryCounts commandJobRetryPolicyCounts,
	logMissingCount int,
) string {
	switch {
	case retryCounts.Ready > 0:
		return "请复核受影响行，然后只重试已就绪设备。"
	case retryCounts.Waiting > 0:
		return "请等待重试窗口，并在重试前刷新任务。"
	case retryCounts.Exhausted > 0 || job.BlockedCount > 0:
		return "请打开支持包，先处理阻断或重试耗尽设备，再重新发起。"
	case logMissingCount > 0:
		return "关闭前请收集命令日志和 broker 投递证据。"
	case health != nil && health.PendingCount > 0:
		return "请持续观察待完成设备，直到出现终态或超时证据。"
	default:
		return "请在客户交接中保留任务链接和治理摘要。"
	}
}
