package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const defaultFleetCommandTimeoutSeconds = 60
const defaultFleetCommandJobListPageSize = 10
const maxFleetCommandJobListPageSize = 50
const defaultFleetCommandJobRowsPageSize = 200
const maxFleetCommandJobRowsPageSize = 500
const defaultFleetCommandJobEventLimit = 100
const maxFleetCommandScheduleAhead = 365 * 24 * time.Hour
const fleetCommandScopeSelectedDevices = "selected_devices"
const fleetCommandScopeDeviceFilter = "device_filter"

const (
	fleetCommandPathImmediate = "immediate"
	fleetCommandPathJobs      = "jobs"
	fleetCommandPathBlocked   = "blocked"
)

const (
	commandJobStatusScheduled       = "scheduled"
	commandJobStatusRunning         = "running"
	commandJobStatusCompleted       = "completed"
	commandJobStatusPartiallyFailed = "partially_failed"
	commandJobStatusFailed          = "failed"
	commandJobStatusCanceled        = "canceled"

	commandJobDetailStatusBlocked     = "blocked"
	commandJobDetailStatusReady       = "ready"
	commandJobDetailStatusDispatching = "dispatching"
	commandJobDetailStatusSubmitted   = "submitted"
	commandJobDetailStatusFailed      = "failed"
	commandJobDetailStatusCanceled    = "canceled"
)

const (
	commandJobEventCreated            = "created"
	commandJobEventScheduled          = "scheduled"
	commandJobEventStarted            = "started"
	commandJobEventQueued             = "queued"
	commandJobEventDispatchStarted    = "dispatch_started"
	commandJobEventDispatchSubmitted  = "dispatch_submitted"
	commandJobEventDispatchFailed     = "dispatch_failed"
	commandJobEventCanceled           = "canceled"
	commandJobEventRetried            = "retried"
	commandJobEventCompleted          = "completed"
	commandJobEventTimeout            = "timeout"
	commandJobEventResumed            = "resumed"
	commandJobEventWorkerFailed       = "worker_failed"
	commandJobEventDeviceAckSuccess   = "device_ack_success"
	commandJobEventDeviceAckFailed    = "device_ack_failed"
	commandJobEventDeviceAckAmbiguous = "device_ack_ambiguous"
)

var fleetCommandJobScopeLimits = []string{
	"当前接口支持已选设备批量任务，以及受数量上限保护的 device_filter 批量任务。",
	"任务可立即执行或持久化 scheduled_at 后由数据库恢复扫描到点激活；行领取由数据库锁定的全局/租户并发与速率配额约束，进程内 worker 只负责执行已获配额的下发。",
	"当前仍没有外部分布式队列、跨租户公平调度或动态租户配额管理；这些能力需要后续 Jobs 引擎继续增强。",
}

func normalizeFleetCommandJobReq(req *model.FleetCommandJobReq) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "批量命令任务请求不能为空")
	}
	req.ScopeType = strings.TrimSpace(req.ScopeType)
	if req.ScopeType == "" {
		req.ScopeType = fleetCommandScopeSelectedDevices
	}
	switch req.ScopeType {
	case fleetCommandScopeSelectedDevices:
	case fleetCommandScopeDeviceFilter:
	default:
		return errcode.NewWithMessage(errcode.CodeParamError, "不支持的批量命令 scope_type")
	}
	req.Identify = strings.TrimSpace(req.Identify)
	if req.Identify == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "命令标识 identify 不能为空")
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = defaultFleetCommandTimeoutSeconds
	}
	if req.ScheduledAt != nil {
		now := time.Now().UTC()
		scheduledAt := req.ScheduledAt.UTC()
		if scheduledAt.IsZero() {
			return errcode.NewWithMessage(errcode.CodeParamError, "scheduled_at 不能为空时间")
		}
		if !scheduledAt.After(now) {
			return errcode.NewWithMessage(errcode.CodeParamError, "scheduled_at 必须晚于当前时间；如需立即执行请省略该字段")
		}
		if scheduledAt.After(now.Add(maxFleetCommandScheduleAhead)) {
			return errcode.NewWithMessage(errcode.CodeParamError, "scheduled_at 不能超过一年")
		}
		req.ScheduledAt = &scheduledAt
	}
	req.PreviewToken = strings.TrimSpace(req.PreviewToken)
	if req.ScopeSource != nil {
		source := strings.TrimSpace(*req.ScopeSource)
		req.ScopeSource = optionalString(source)
	}
	return nil
}

func uniqueFleetDeviceIDs(deviceIDs []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(deviceIDs))
	for _, rawID := range deviceIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func (c *CommandData) PreviewFleetCommandJob(req *model.FleetCommandJobReq, claims *utils.UserClaims) (*model.FleetCommandJobPreviewResult, error) {
	if err := normalizeFleetCommandJobReq(req); err != nil {
		return nil, err
	}
	if req.ScopeType == fleetCommandScopeDeviceFilter {
		return c.previewFleetCommandDeviceFilter(req, claims)
	}

	return c.previewFleetCommandSelectedDevices(req, claims)
}

func (c *CommandData) SubmitFleetCommandJob(ctx context.Context, operatorID string, req *model.FleetCommandJobReq, claims *utils.UserClaims, includeRows ...bool) (*model.FleetCommandJobSubmitResult, error) {
	_ = ctx
	if err := normalizeFleetCommandJobReq(req); err != nil {
		return nil, err
	}
	preview, err := c.PreviewFleetCommandJob(req, claims)
	if err != nil {
		return nil, err
	}
	if req.PreviewToken == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "提交前必须携带命令任务预览令牌；请先预览再提交")
	}
	if req.PreviewToken != preview.PreviewToken {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "命令任务预览已过期；请重新预览后再提交")
	}
	if req.ScopeType == fleetCommandScopeDeviceFilter {
		maxDevices := normalizeFleetCommandDeviceFilterMaxDevices(req.MaxDevices)
		if preview.TotalMatched > int64(maxDevices) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("device_filter 匹配到 %d 台设备，max_devices 为 %d", preview.TotalMatched, maxDevices))
		}
		if len(preview.Rows) != preview.RequestedCount {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_filter 当前预览只是子集；请缩小筛选范围并重新预览后再提交")
		}
	}

	job, details, err := buildPersistedFleetCommandJob(req, preview, operatorID, claims)
	if err != nil {
		return nil, err
	}
	if err := dal.CreateCommandJobWithDetails(job, details); err != nil {
		return nil, err
	}

	recordFleetCommandJobEvent(job.ID, claims.TenantID, nil, nil, commandJobEventCreated, "命令任务已从预览创建")
	if job.Status == commandJobStatusScheduled && job.ScheduledAt != nil {
		recordFleetCommandJobEvent(
			job.ID,
			claims.TenantID,
			nil,
			nil,
			commandJobEventScheduled,
			"命令任务将在 "+job.ScheduledAt.UTC().Format(time.RFC3339)+" 进入下发队列",
		)
	} else {
		recordFleetCommandJobEvent(job.ID, claims.TenantID, nil, nil, commandJobEventQueued, "可执行命令行已持久化，等待 worker 下发")
		c.dispatchFleetCommandJob(job.ID, operatorID, claims)
	}
	if !fleetCommandJobShouldIncludeRows(includeRows) {
		return c.GetFleetCommandJobSummary(job.ID, claims)
	}
	return c.GetFleetCommandJob(job.ID, claims)
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return StringPtr(value)
}

func applyFleetCommandJobCancelRequest(job *model.CommandJob, canceledRows int64) {
	if job == nil {
		return
	}
	job.Status = commandJobStatusCanceled
	job.CanCancel = false
	job.CanRetryFailed = false
	job.NextDispatchAt = nil
	job.Remark = StringPtr(commandJobCancelRemark(canceledRows))
	job.UpdatedAt = time.Now().UTC()
}

func commandJobCancelRemark(canceledRows int64) string {
	if canceledRows <= 0 {
		return "收到取消请求时，所有待处理行都已离开队列；下发中的行仍可能产生设备确认"
	}
	return fmt.Sprintf("收到取消请求；%d 行待处理命令已在下发前取消，下发中的行仍可能产生设备确认", canceledRows)
}

func commandJobCancelEventMessage(canceledRows int64) string {
	if canceledRows <= 0 {
		return "收到取消请求时，待处理行都已离开队列；下发中的行保留用于 ACK 或支持证据"
	}
	return fmt.Sprintf("%d 行待处理命令已在下发前取消；下发中的行保留用于 ACK 或支持证据", canceledRows)
}

func (c *CommandData) CancelFleetCommandJob(jobID string, claims *utils.UserClaims, includeRows ...bool) (*model.FleetCommandJobSubmitResult, error) {
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	if job.Status != commandJobStatusRunning && job.Status != commandJobStatusScheduled {
		if !fleetCommandJobShouldIncludeRows(includeRows) {
			return c.GetFleetCommandJobSummary(job.ID, claims)
		}
		return commandJobResultFromPersistenceWithFreshDetails(job, claims)
	}
	canceledRows, err := dal.UpdateCommandJobDetailsStatus(job.ID, claims.TenantID, []string{commandJobDetailStatusReady}, commandJobDetailStatusCanceled, "任务已在下发前取消", false)
	if err != nil {
		return nil, err
	}
	applyFleetCommandJobCancelRequest(job, canceledRows)
	if err := dal.UpdateCommandJob(job); err != nil {
		return nil, err
	}
	recordFleetCommandJobEvent(job.ID, claims.TenantID, nil, nil, commandJobEventCanceled, commandJobCancelEventMessage(canceledRows))
	if err := refreshCommandJobSummary(job); err != nil {
		return nil, err
	}
	if !fleetCommandJobShouldIncludeRows(includeRows) {
		return c.GetFleetCommandJobSummary(job.ID, claims)
	}
	return c.GetFleetCommandJob(job.ID, claims)
}

func (c *CommandData) RetryFleetCommandJob(ctx context.Context, jobID, operatorID string, claims *utils.UserClaims, includeRows ...bool) (*model.FleetCommandJobSubmitResult, error) {
	_ = ctx
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	if job.Status == commandJobStatusCanceled {
		if !fleetCommandJobShouldIncludeRows(includeRows) {
			return c.GetFleetCommandJobSummary(job.ID, claims)
		}
		return c.GetFleetCommandJob(job.ID, claims)
	}
	requeueResult, err := dal.RequeueRetryableCommandJobDetails(job.ID, claims.TenantID, commandJobMaxDispatchAttempts, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if requeueResult.Requeued == 0 {
		if requeueResult.CoolingDown > 0 {
			recordFleetCommandJobEvent(job.ID, claims.TenantID, nil, nil, commandJobEventRetried, fmt.Sprintf("%d 行失败命令正在等待重试退避", requeueResult.CoolingDown))
		}
		if requeueResult.Exhausted > 0 {
			recordFleetCommandJobEvent(job.ID, claims.TenantID, nil, nil, commandJobEventRetried, fmt.Sprintf("%d 行失败命令已达到最大下发次数", requeueResult.Exhausted))
			if err := refreshCommandJobSummary(job); err != nil {
				return nil, err
			}
		}
		if !fleetCommandJobShouldIncludeRows(includeRows) {
			return c.GetFleetCommandJobSummary(job.ID, claims)
		}
		return c.GetFleetCommandJob(job.ID, claims)
	}
	recordFleetCommandJobEvent(job.ID, claims.TenantID, nil, nil, commandJobEventRetried, fmt.Sprintf("%d 行失败命令已加入重试队列", requeueResult.Requeued))
	if err := refreshCommandJobSummary(job); err != nil {
		return nil, err
	}
	c.dispatchFleetCommandJob(job.ID, operatorID, claims)
	if !fleetCommandJobShouldIncludeRows(includeRows) {
		return c.GetFleetCommandJobSummary(job.ID, claims)
	}
	return c.GetFleetCommandJob(job.ID, claims)
}

func fleetCommandJobShouldIncludeRows(includeRows []bool) bool {
	return len(includeRows) == 0 || includeRows[0]
}
