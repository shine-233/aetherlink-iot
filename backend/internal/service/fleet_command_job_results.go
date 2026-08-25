package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
)

func commandJobResultFromPersistenceWithFreshDetails(job *model.CommandJob, claims *utils.UserClaims) (*model.FleetCommandJobSubmitResult, error) {
	details, err := dal.GetCommandJobDetails(job.ID, claims.TenantID, commandJobInlineRowLimit)
	if err != nil {
		return nil, err
	}
	counts, err := dal.CountCommandJobDetailsByStatus(job.ID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	events, err := loadRecentCommandJobEvents(job.ID, claims.TenantID)
	if err != nil {
		events = nil
	}
	return commandJobResultFromPersistence(job, details, counts, events), nil
}

func commandJobSupportBundleFromPersistence(
	job *model.CommandJob,
	supportDetails []*model.CommandJobDetail,
	counts map[string]int,
	retryableCount int,
	retryReadyCount int,
	retryWaitingCount int,
	retryExhaustedCount int,
	logMissingCount int,
	events []*model.CommandJobEvent,
) *model.FleetCommandJobSupportBundle {
	commandLogs := commandSetLogsByDeviceMessageIDForJobDetails(supportDetails)
	progressHealth := buildFleetCommandJobProgressHealth(job, time.Now().UTC())
	auditSummary := buildFleetCommandJobAuditSummary(events)
	// 支持包证据行同样受内联上限约束；达到上限且状态计数显示仍有更多行时
	// 才判定截断并显式提示，避免把部分证据当成完整证据集交给支持人员复核。
	supportRowsTruncated := len(supportDetails) >= commandJobInlineRowLimit &&
		commandJobStatusCountsTotal(counts) > len(supportDetails)
	retryCounts := commandJobRetryPolicyCounts{
		Retryable: retryableCount,
		Ready:     retryReadyCount,
		Waiting:   retryWaitingCount,
		Exhausted: retryExhaustedCount,
	}
	bundle := &model.FleetCommandJobSupportBundle{
		JobID:               job.ID,
		JobType:             job.JobType,
		ScopeType:           job.ScopeType,
		Identify:            job.Identify,
		Status:              job.Status,
		ScheduledAt:         job.ScheduledAt,
		NextDispatchAt:      job.NextDispatchAt,
		AuditRemark:         job.Remark,
		RequestedCount:      job.RequestedCount,
		EligibleCount:       job.EligibleCount,
		BlockedCount:        job.BlockedCount,
		SubmittedCount:      job.SubmittedCount,
		FailedCount:         job.FailedCount,
		RetryableCount:      retryableCount,
		RetryReadyCount:     retryReadyCount,
		RetryWaitingCount:   retryWaitingCount,
		RetryExhaustedCount: retryExhaustedCount,
		LogMissingCount:     logMissingCount,
		StatusCounts:        counts,
		RowsTruncated:       supportRowsTruncated,
		Events:              fleetCommandJobEventsFromPersistence(events),
		ExecutionSummary:    buildFleetCommandJobExecutionSummary(job, progressHealth, retryCounts, logMissingCount, auditSummary),
		Governance:          buildFleetCommandJobGovernanceSummary(job, progressHealth, retryCounts, logMissingCount, auditSummary),
		GeneratedAt:         time.Now().UTC(),
		ShareHint:           "Share this bundle with support when a command job needs troubleshooting. It contains job outcome evidence, retryable devices, missing log evidence, and per-device failure reasons.",
	}

	for _, detail := range supportDetails {
		row := commandJobSubmitRowFromDetail(detail, commandLogs)
		if detail.CanRetry {
			bundle.RetryableDeviceIDs = append(bundle.RetryableDeviceIDs, detail.DeviceID)
		}
		if detail.Status == commandJobDetailStatusSubmitted && !detail.LogRecorded {
			bundle.MissingLogDeviceIDs = append(bundle.MissingLogDeviceIDs, detail.DeviceID)
		}
		if commandJobSupportBundleIncludesDeviceEvidence(job, detail, row) {
			bundle.FailedDevices = append(bundle.FailedDevices, commandJobSupportDeviceFromDetail(job, detail, row))
		}
	}
	bundle.NextActions = commandJobSupportNextActions(bundle)
	if supportRowsTruncated {
		bundle.NextActions = append(bundle.NextActions, "支持包证据行数已达到单次内联上限，可能缺少部分设备行；请结合分页 rows 接口复核完整结果。")
	}
	return bundle
}

func commandJobSupportBundleIncludesDeviceEvidence(job *model.CommandJob, detail *model.CommandJobDetail, row model.FleetCommandJobSubmitRow) bool {
	if detail == nil {
		return false
	}
	if detail.Status == commandJobDetailStatusFailed || detail.Status == commandJobDetailStatusBlocked || detail.Status == commandJobDetailStatusCanceled || row.ResponseStatusLabel == "device_ack_failed" {
		return true
	}
	return job != nil && job.Status == commandJobStatusCanceled && detail.Status == commandJobDetailStatusDispatching
}

func commandJobSupportDeviceFromDetail(job *model.CommandJob, detail *model.CommandJobDetail, row model.FleetCommandJobSubmitRow) model.FleetCommandJobSupportDevice {
	return model.FleetCommandJobSupportDevice{
		DetailID:            detail.ID,
		DeviceID:            detail.DeviceID,
		DeviceNumber:        detail.DeviceNumber,
		Name:                detail.Name,
		Status:              detail.Status,
		Readiness:           row.Readiness,
		MessageID:           SafeDeref(detail.MessageID),
		DispatchAttempts:    detail.DispatchAttempts,
		MaxDispatchAttempts: commandJobMaxDispatchAttempts,
		RetryState:          commandJobDetailRetryState(detail, time.Now().UTC()),
		NextRetryAfter:      detail.NextRetryAfter,
		ResponseStatus:      row.ResponseStatus,
		ResponseStatusLabel: row.ResponseStatusLabel,
		ResponseData:        row.ResponseData,
		ResponseError:       row.ResponseError,
		ResponseAt:          row.CommandLogCreatedAt,
		Reason:              SafeDeref(detail.Reason),
		Advice:              SafeDeref(detail.Advice),
		ReadyCheckURL:       commandJobReadyCheckURLFromDetail(job, detail),
		JobDetailURL:        commandJobDetailURLFromDetail(job, detail),
		DiagnosticSummary:   commandJobSupportDiagnosticFromDetail(job, detail, row),
	}
}

func commandJobReadyCheckURLFromDetail(job *model.CommandJob, detail *model.CommandJobDetail) string {
	if detail == nil || detail.DeviceID == "" {
		return ""
	}
	query := url.Values{}
	query.Set("d_id", detail.DeviceID)
	query.Set("tab", "ready-check")
	query.Set("source", "command_job_diagnosis")
	if job != nil && job.ID != "" {
		query.Set("command_job_id", job.ID)
	}
	if detail.ID != "" {
		query.Set("command_detail_id", detail.ID)
	}
	if messageID := SafeDeref(detail.MessageID); messageID != "" {
		query.Set("message_id", messageID)
	}
	return "/device/details?" + query.Encode()
}

func commandJobDetailURLFromDetail(job *model.CommandJob, detail *model.CommandJobDetail) string {
	if job == nil || job.ID == "" {
		return ""
	}
	query := url.Values{}
	query.Set("command_job_id", job.ID)
	if detail != nil {
		if detail.ID != "" {
			query.Set("detail_id", detail.ID)
		}
		if detail.DeviceID != "" {
			query.Set("device_id", detail.DeviceID)
		}
	}
	return "/device/command-center?" + query.Encode()
}

func commandJobDetailReadiness(detail *model.CommandJobDetail) []string {
	if detail == nil || detail.Readiness == nil || *detail.Readiness == "" {
		return nil
	}
	var readiness []string
	if err := json.Unmarshal([]byte(*detail.Readiness), &readiness); err != nil {
		return nil
	}
	return readiness
}

func commandJobSupportDiagnosticFromDetail(job *model.CommandJob, detail *model.CommandJobDetail, row model.FleetCommandJobSubmitRow) *model.FleetCommandJobSupportDiagnostic {
	if detail == nil {
		return nil
	}
	evidence := []string{
		"status=" + emptyAsUnknown(detail.Status),
		"message_id=" + emptyAsUnknown(SafeDeref(detail.MessageID)),
		fmt.Sprintf("dispatch_attempts=%d", detail.DispatchAttempts),
		fmt.Sprintf("max_dispatch_attempts=%d", commandJobMaxDispatchAttempts),
		"retry_state=" + commandJobDetailRetryState(detail, time.Now().UTC()),
	}
	if detail.NextRetryAfter != nil {
		evidence = append(evidence, "next_retry_after="+detail.NextRetryAfter.Format(time.RFC3339))
	}
	if row.ResponseStatusLabel != "" {
		evidence = append(evidence, "response_status="+row.ResponseStatusLabel)
	}
	if reason := SafeDeref(detail.Reason); reason != "" {
		evidence = append(evidence, "reason="+reason)
	}
	if responseError := row.ResponseError; responseError != "" {
		evidence = append(evidence, "response_error="+responseError)
	}

	switch {
	case job != nil && job.Status == commandJobStatusCanceled && detail.Status == commandJobDetailStatusDispatching:
		return &model.FleetCommandJobSupportDiagnostic{
			Level:       "warning",
			Code:        "cancel_in_flight",
			Summary:     "任务取消时，这一行已经在下发途中。",
			Evidence:    evidence,
			NextActions: []string{"等待可能延迟到达的设备确认或命令日志证据。", "请把这一行保留在支持包中；如果操作仍需执行，请重新创建预览。"},
		}
	case row.ResponseStatusLabel == "device_ack_failed":
		return &model.FleetCommandJobSupportDiagnostic{
			Level:       "error",
			Code:        "device_ack_failed",
			Summary:     "设备对这一条命令返回了失败响应。",
			Evidence:    evidence,
			NextActions: []string{"打开这台设备的 Ready Check，并检查响应载荷。", "确认命令载荷和设备状态后再重试。"},
		}
	case detail.Status == commandJobDetailStatusFailed && detail.CanRetry:
		return &model.FleetCommandJobSupportDiagnostic{
			Level:       "warning",
			Code:        "retryable_dispatch_failure",
			Summary:     "平台在确认设备完成前，将这一行标记为失败且可重试。",
			Evidence:    evidence,
			NextActions: []string{"复核失败原因和命令发布路径。", "理解阻断原因后，再只重试这台设备。"},
		}
	case detail.Status == commandJobDetailStatusBlocked:
		return &model.FleetCommandJobSupportDiagnostic{
			Level:       "error",
			Code:        "blocked_before_dispatch",
			Summary:     "这一行在命令下发前已被阻断。",
			Evidence:    evidence,
			NextActions: []string{"处理预览或支持证据中显示的可执行性阻断项。", "提交新任务前请重新运行预览。"},
		}
	case detail.Status == commandJobDetailStatusCanceled:
		return &model.FleetCommandJobSupportDiagnostic{
			Level:       "info",
			Code:        "canceled_before_terminal_result",
			Summary:     "这一行在记录设备终态结果前已被取消。",
			Evidence:    evidence,
			NextActions: []string{"保留取消证据用于审计。", "如果操作仍需执行，请重新创建预览。"},
		}
	case detail.Status == commandJobDetailStatusSubmitted && !detail.LogRecorded:
		return &model.FleetCommandJobSupportDiagnostic{
			Level:       "warning",
			Code:        "missing_platform_log",
			Summary:     "命令行已提交，但缺少平台日志证据。",
			Evidence:    evidence,
			NextActions: []string{"刷新任务和命令日志。", "重试前请检查 broker 投递证据。"},
		}
	default:
		return &model.FleetCommandJobSupportDiagnostic{
			Level:       "info",
			Code:        "needs_row_review",
			Summary:     "重试或升级处理这台设备前，请先复核这一行证据。",
			Evidence:    evidence,
			NextActions: []string{"打开这台设备的 Ready Check，对比命令响应、遥测和设备影子证据。"},
		}
	}
}

func commandJobSupportNextActions(bundle *model.FleetCommandJobSupportBundle) []string {
	actions := []string{"打开命令任务详情链接，并在继续处理前刷新一次。"}
	if bundle.Status == commandJobStatusCanceled {
		actions = append(actions, "请把已取消任务链接和支持包作为审计回执保留。")
		if bundle.StatusCounts[commandJobDetailStatusDispatching] > 0 {
			actions = append(actions, "关闭客户交接前，请复核下发中的行是否有延迟 ACK 或命令日志证据。")
		}
		if bundle.BlockedCount > 0 {
			actions = append(actions, "请处理被阻断设备的可执行性；如果操作仍需执行，请重新运行预览。")
		}
		if bundle.FailedCount == 0 && bundle.LogMissingCount == 0 {
			actions = append(actions, "当前没有记录失败或缺日志行；请保留已取消任务 ID 供审计跟进。")
		}
		return actions
	}
	if bundle.RetryReadyCount > 0 {
		actions = append(actions, "复核每个失败原因后，再重试已就绪设备。")
	}
	if bundle.RetryWaitingCount > 0 {
		actions = append(actions, "仍在冷却的设备需要等到重试窗口后再重新入队。")
	}
	if bundle.RetryExhaustedCount > 0 {
		actions = append(actions, "达到重试上限的设备需要支持复核后，再创建新的尝试。")
	}
	if bundle.LogMissingCount > 0 {
		actions = append(actions, "请检查缺少平台日志设备的命令发布日志和 broker 投递证据。")
	}
	if bundle.BlockedCount > 0 {
		actions = append(actions, "请先处理被阻断设备的可执行性，再重新预览并提交。")
	}
	if bundle.FailedCount == 0 && bundle.LogMissingCount == 0 {
		actions = append(actions, "当前没有记录失败或缺日志行；请保留任务 ID 供审计跟进。")
	}
	return actions
}

func commandJobResultFromPersistence(job *model.CommandJob, details []*model.CommandJobDetail, counts map[string]int, events []*model.CommandJobEvent) *model.FleetCommandJobSubmitResult {
	retryCounts := commandJobRetryPolicyCountsFromDetails(details, time.Now().UTC())
	logMissingCount := 0
	for _, detail := range details {
		if detail.Status == commandJobDetailStatusSubmitted && !detail.LogRecorded {
			logMissingCount++
		}
	}
	rows := commandJobRowsFromDetails(details)
	// 内联行集受 commandJobInlineRowLimit 约束；真实总数以状态计数为准，
	// 截断时必须显式标记，避免把“前 N 行”当成全量结果。
	// counts 为 nil/空时退化为内联行数本身，RowsTotal 始终不小于已返回行数。
	rowsTotal := len(rows)
	if countsTotal := commandJobStatusCountsTotal(counts); countsTotal > rowsTotal {
		rowsTotal = countsTotal
	}
	rowsTruncated := len(rows) < rowsTotal
	createdAt := job.CreatedAt
	updatedAt := job.UpdatedAt
	scopeLimits := fleetCommandJobScopeLimits
	if job.ScopeType == fleetCommandScopeDeviceFilter {
		scopeLimits = fleetCommandDeviceFilterPreviewScopeLimits
	}
	progressHealth := buildFleetCommandJobProgressHealth(job, time.Now().UTC())
	auditSummary := buildFleetCommandJobAuditSummary(events)
	executionSummary := buildFleetCommandJobExecutionSummary(job, progressHealth, retryCounts, logMissingCount, auditSummary)
	return &model.FleetCommandJobSubmitResult{
		JobID:               job.ID,
		JobType:             job.JobType,
		ScopeType:           job.ScopeType,
		Identify:            job.Identify,
		Status:              job.Status,
		AuditRemark:         job.Remark,
		RequestedCount:      job.RequestedCount,
		EligibleCount:       job.EligibleCount,
		BlockedCount:        job.BlockedCount,
		SubmittedCount:      job.SubmittedCount,
		FailedCount:         job.FailedCount,
		RetryableCount:      retryCounts.Retryable,
		RetryReadyCount:     retryCounts.Ready,
		RetryWaitingCount:   retryCounts.Waiting,
		RetryExhaustedCount: retryCounts.Exhausted,
		LogMissingCount:     logMissingCount,
		TimeoutSeconds:      job.TimeoutSeconds,
		CanCancel:           job.CanCancel,
		CanRetryFailed:      commandJobCanRetryFailed(job, retryCounts.Ready),
		CreatedAt:           &createdAt,
		UpdatedAt:           &updatedAt,
		ScheduledAt:         job.ScheduledAt,
		NextDispatchAt:      job.NextDispatchAt,
		TimeoutAt:           job.TimeoutAt,
		Rows:                rows,
		RowsTotal:           rowsTotal,
		RowsTruncated:       rowsTruncated,
		Events:              fleetCommandJobEventsFromPersistence(events),
		StatusCounts:        counts,
		ProgressHealth:      progressHealth,
		HandoffSummary:      buildFleetCommandJobHandoffSummary(job, progressHealth, executionSummary),
		AuditSummary:        auditSummary,
		ExecutionSummary:    executionSummary,
		Governance:          buildFleetCommandJobGovernanceSummary(job, progressHealth, retryCounts, logMissingCount, auditSummary),
		ScopeLimits:         scopeLimits,
		Warnings:            fleetCommandJobSubmitResultWarnings(rowsTruncated),
	}
}

// fleetCommandJobSubmitResultWarnings 生成提交结果响应的基础提示；
// 内联行集被截断时追加显式截断说明，引导调用方改用分页 rows 接口。
func fleetCommandJobSubmitResultWarnings(rowsTruncated bool) []string {
	warnings := []string{
		"任务和逐设备提交结果已持久化，可用于刷新、重试、取消和审计复查。",
		"设备侧业务完成仍需结合命令响应、遥测或设备影子证据判断。",
	}
	if rowsTruncated {
		warnings = append(warnings, "逐设备行数超过单次内联上限，当前仅返回部分行；完整结果请使用分页 rows 接口查询。")
	}
	return warnings
}

// commandJobStatusCountsTotal 汇总各状态计数，得到 detail 行真实总数。
func commandJobStatusCountsTotal(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

// commandJobCanRetryFailed is derived from the same fresh retry policy counts
// exposed in the response. The persisted job summary can briefly lag a detail
// ACK update, so forwarding job.CanRetryFailed would make one response claim
// that a row is retry-ready while the job-level action is disabled.
func commandJobCanRetryFailed(job *model.CommandJob, retryReadyCount int) bool {
	return job != nil && job.Status != commandJobStatusCanceled && retryReadyCount > 0
}

func commandJobRowsFromDetails(details []*model.CommandJobDetail) []model.FleetCommandJobSubmitRow {
	commandLogs := commandSetLogsByDeviceMessageIDForJobDetails(details)
	return commandJobRowsFromDetailsAndLogs(details, commandLogs)
}

func commandJobRowsFromDetailsAndLogs(
	details []*model.CommandJobDetail,
	commandLogs map[string]*model.CommandSetLog,
) []model.FleetCommandJobSubmitRow {
	rows := make([]model.FleetCommandJobSubmitRow, 0, len(details))
	for _, detail := range details {
		rows = append(rows, commandJobSubmitRowFromDetail(detail, commandLogs))
	}
	return rows
}

func commandJobResultSummaryFromPersistence(
	job *model.CommandJob,
	counts map[string]int,
	retryableCount int,
	retryReadyCount int,
	retryWaitingCount int,
	retryExhaustedCount int,
	logMissingCount int,
	events []*model.CommandJobEvent,
) *model.FleetCommandJobSubmitResult {
	createdAt := job.CreatedAt
	updatedAt := job.UpdatedAt
	scopeLimits := fleetCommandJobScopeLimits
	if job.ScopeType == fleetCommandScopeDeviceFilter {
		scopeLimits = fleetCommandDeviceFilterPreviewScopeLimits
	}
	progressHealth := buildFleetCommandJobProgressHealth(job, time.Now().UTC())
	auditSummary := buildFleetCommandJobAuditSummary(events)
	retryCounts := commandJobRetryPolicyCounts{
		Retryable: retryableCount,
		Ready:     retryReadyCount,
		Waiting:   retryWaitingCount,
		Exhausted: retryExhaustedCount,
	}
	executionSummary := buildFleetCommandJobExecutionSummary(job, progressHealth, retryCounts, logMissingCount, auditSummary)
	return &model.FleetCommandJobSubmitResult{
		JobID:               job.ID,
		JobType:             job.JobType,
		ScopeType:           job.ScopeType,
		Identify:            job.Identify,
		Status:              job.Status,
		AuditRemark:         job.Remark,
		RequestedCount:      job.RequestedCount,
		EligibleCount:       job.EligibleCount,
		BlockedCount:        job.BlockedCount,
		SubmittedCount:      job.SubmittedCount,
		FailedCount:         job.FailedCount,
		RetryableCount:      retryableCount,
		RetryReadyCount:     retryReadyCount,
		RetryWaitingCount:   retryWaitingCount,
		RetryExhaustedCount: retryExhaustedCount,
		LogMissingCount:     logMissingCount,
		TimeoutSeconds:      job.TimeoutSeconds,
		CanCancel:           job.CanCancel,
		CanRetryFailed:      commandJobCanRetryFailed(job, retryReadyCount),
		CreatedAt:           &createdAt,
		UpdatedAt:           &updatedAt,
		ScheduledAt:         job.ScheduledAt,
		NextDispatchAt:      job.NextDispatchAt,
		TimeoutAt:           job.TimeoutAt,
		Rows:                []model.FleetCommandJobSubmitRow{},
		RowsTotal:           job.RequestedCount,
		RowsTruncated:       true,
		Events:              fleetCommandJobEventsFromPersistence(events),
		StatusCounts:        counts,
		ProgressHealth:      progressHealth,
		HandoffSummary:      buildFleetCommandJobHandoffSummary(job, progressHealth, executionSummary),
		AuditSummary:        auditSummary,
		ExecutionSummary:    executionSummary,
		Governance:          buildFleetCommandJobGovernanceSummary(job, progressHealth, retryCounts, logMissingCount, auditSummary),
		ScopeLimits:         scopeLimits,
		Warnings: []string{
			"当前只加载任务摘要，没有逐设备行；如需检查全部设备结果，请请求 include_rows=true。",
			"设备侧业务完成仍需结合命令响应、遥测或设备影子证据判断。",
		},
	}
}

type commandJobRetryPolicyCounts struct {
	Retryable int
	Ready     int
	Waiting   int
	Exhausted int
}

func commandJobRetryPolicyCountsFromDetails(details []*model.CommandJobDetail, now time.Time) commandJobRetryPolicyCounts {
	counts := commandJobRetryPolicyCounts{}
	for _, detail := range details {
		state := commandJobDetailRetryState(detail, now)
		switch state {
		case "retryable":
			counts.Retryable++
			counts.Ready++
		case "waiting_backoff":
			counts.Retryable++
			counts.Waiting++
		case "max_attempts_reached":
			counts.Exhausted++
		}
	}
	return counts
}

func commandSetLogLookupKeyForDetail(detail *model.CommandJobDetail) string {
	if detail == nil || detail.MessageID == nil {
		return ""
	}
	return dal.CommandSetLogLookupKey(detail.DeviceID, *detail.MessageID)
}

func commandSetLogsByDeviceMessageIDForJobDetails(details []*model.CommandJobDetail) map[string]*model.CommandSetLog {
	lookups := make([]dal.CommandSetLogLookup, 0, len(details))
	seen := map[string]struct{}{}
	for _, detail := range details {
		if detail == nil || detail.DeviceID == "" || detail.MessageID == nil || *detail.MessageID == "" {
			continue
		}
		key := commandSetLogLookupKeyForDetail(detail)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		lookups = append(lookups, dal.CommandSetLogLookup{
			DeviceID:  detail.DeviceID,
			MessageID: *detail.MessageID,
		})
	}
	if len(lookups) == 0 || global.DB == nil {
		// 生产环境 global.DB 与 query.SetDefault 恒成对初始化（见 application.go）；
		// DB 未就绪（如纯内存单测）时跳过日志联查，仅返回 detail 自带响应字段。
		return map[string]*model.CommandSetLog{}
	}
	logs, err := dal.GetCommandSetLogsByDeviceMessageIDs(lookups)
	if err != nil {
		return map[string]*model.CommandSetLog{}
	}
	return logs
}

func commandJobSubmitRowFromDetail(detail *model.CommandJobDetail, commandLogs map[string]*model.CommandSetLog) model.FleetCommandJobSubmitRow {
	row := model.FleetCommandJobSubmitRow{
		DetailID:              detail.ID,
		DeviceID:              detail.DeviceID,
		DeviceNumber:          detail.DeviceNumber,
		Name:                  detail.Name,
		Eligible:              detail.Eligible,
		Status:                detail.Status,
		Readiness:             commandJobDetailReadiness(detail),
		MessageID:             SafeDeref(detail.MessageID),
		DispatchAttempts:      detail.DispatchAttempts,
		MaxDispatchAttempts:   commandJobMaxDispatchAttempts,
		RetryState:            commandJobDetailRetryState(detail, time.Now().UTC()),
		LastDispatchStartedAt: detail.LastDispatchStartedAt,
		NextRetryAfter:        detail.NextRetryAfter,
		LogRecorded:           detail.LogRecorded,
		Reason:                SafeDeref(detail.Reason),
		Advice:                SafeDeref(detail.Advice),
		CanRetry:              detail.CanRetry,
		RecommendedPath:       detail.RecommendedPath,
		TelemetryCurrentCount: detail.TelemetryCurrentCount,
		LatestTelemetryKey:    detail.LatestTelemetryKey,
		LatestTelemetryAt:     detail.LatestTelemetryAt,
		SubmittedAt:           detail.SubmittedAt,
		CompletedAt:           detail.CompletedAt,
	}
	if detail.ResponseStatus != nil || detail.ResponsePayload != nil || detail.ResponseError != nil || detail.ResponseAt != nil {
		row.ResponseRecorded = detail.ResponseStatus != nil || detail.ResponsePayload != nil || detail.ResponseError != nil
		row.ResponseStatus = SafeDeref(detail.ResponseStatus)
		if row.ResponseStatus != "" {
			row.ResponseStatusLabel = commandDeliveryStatusLabel(row.ResponseStatus)
		}
		row.ResponseData = SafeDeref(detail.ResponsePayload)
		row.ResponseError = SafeDeref(detail.ResponseError)
		row.CommandLogCreatedAt = detail.ResponseAt
		return row
	}
	if detail.MessageID == nil || commandLogs == nil {
		return row
	}
	if log := commandLogs[commandSetLogLookupKeyForDetail(detail)]; log != nil {
		row.ResponseStatus = SafeDeref(log.Status)
		if row.ResponseStatus != "" {
			row.ResponseStatusLabel = commandDeliveryStatusLabel(row.ResponseStatus)
		}
		row.ResponseRecorded = log.RspDatum != nil || log.ErrorMessage != nil || row.ResponseStatusLabel == "device_ack_success" || row.ResponseStatusLabel == "device_ack_failed"
		row.ResponseData = SafeDeref(log.RspDatum)
		row.ResponseError = SafeDeref(log.ErrorMessage)
		row.CommandLogCreatedAt = &log.CreatedAt
	}
	return row
}
