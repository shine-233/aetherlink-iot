package service

import (
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// FleetCommandJobQueryService owns Command Job read-side orchestration.
type FleetCommandJobQueryService struct{}

func newFleetCommandJobQueryService() FleetCommandJobQueryService {
	return FleetCommandJobQueryService{}
}

func (c *CommandData) GetFleetCommandJob(jobID string, claims *utils.UserClaims) (*model.FleetCommandJobSubmitResult, error) {
	return newFleetCommandJobQueryService().GetFleetCommandJob(jobID, claims)
}

func (c *CommandData) GetFleetCommandJobRows(jobID string, req *model.FleetCommandJobRowsReq, claims *utils.UserClaims) (*model.FleetCommandJobRowsResult, error) {
	return newFleetCommandJobQueryService().GetFleetCommandJobRows(jobID, req, claims)
}

func (c *CommandData) GetFleetCommandJobSummary(jobID string, claims *utils.UserClaims) (*model.FleetCommandJobSubmitResult, error) {
	return newFleetCommandJobQueryService().GetFleetCommandJobSummary(jobID, claims)
}

func (c *CommandData) GetFleetCommandJobSupportBundle(jobID string, claims *utils.UserClaims) (*model.FleetCommandJobSupportBundle, error) {
	return newFleetCommandJobQueryService().GetFleetCommandJobSupportBundle(jobID, claims)
}

func (c *CommandData) ListFleetCommandJobs(req *model.FleetCommandJobListReq, claims *utils.UserClaims) (*model.FleetCommandJobListResult, error) {
	return newFleetCommandJobQueryService().ListFleetCommandJobs(req, claims)
}

func (FleetCommandJobQueryService) GetFleetCommandJob(jobID string, claims *utils.UserClaims) (*model.FleetCommandJobSubmitResult, error) {
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
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

func (FleetCommandJobQueryService) GetFleetCommandJobRows(jobID string, req *model.FleetCommandJobRowsReq, claims *utils.UserClaims) (*model.FleetCommandJobRowsResult, error) {
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	req = normalizeFleetCommandJobRowsReq(req)
	now := time.Now().UTC()

	total, err := dal.CountCommandJobDetailsByFilter(
		job.ID,
		claims.TenantID,
		req.StatusFilter,
		req.Search,
		commandJobMaxDispatchAttempts,
		now,
	)
	if err != nil {
		return nil, err
	}
	details, err := dal.GetCommandJobDetailsByPageAndFilter(
		job.ID,
		claims.TenantID,
		req.Page,
		req.PageSize,
		req.StatusFilter,
		req.Search,
		commandJobMaxDispatchAttempts,
		now,
	)
	if err != nil {
		return nil, err
	}
	return commandJobRowsResultFromPersistence(total, req, details), nil
}

func (FleetCommandJobQueryService) GetFleetCommandJobSummary(jobID string, claims *utils.UserClaims) (*model.FleetCommandJobSubmitResult, error) {
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	metrics, err := dal.GetCommandJobSummaryMetrics(job.ID, claims.TenantID, commandJobMaxDispatchAttempts, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	events, err := loadRecentCommandJobEvents(job.ID, claims.TenantID)
	if err != nil {
		events = nil
	}
	return commandJobResultSummaryFromPersistence(
		job,
		metrics.StatusCounts,
		metrics.RetryableCount,
		metrics.RetryReadyCount,
		metrics.RetryWaitingCount,
		metrics.RetryExhaustedCount,
		metrics.LogMissingCount,
		events,
	), nil
}

func loadFleetCommandJobWithFreshTimeout(jobID, tenantID string) (*model.CommandJob, error) {
	job, err := dal.GetCommandJobByID(jobID, tenantID)
	if err != nil {
		return nil, err
	}
	if err := expireCommandJobIfTimedOut(job); err != nil {
		return nil, err
	}
	return job, nil
}

func expireCommandJobIfTimedOut(job *model.CommandJob) error {
	if job == nil ||
		(job.Status != commandJobStatusRunning && job.Status != commandJobStatusScheduled) ||
		job.TimeoutAt == nil ||
		time.Now().UTC().Before(*job.TimeoutAt) {
		return nil
	}
	now := time.Now().UTC()
	affected, err := dal.FailTimedOutCommandJobDetailsWithRetryPolicy(
		job.ID,
		job.TenantID,
		commandJobMaxDispatchAttempts,
		commandJobNextRetryAfter(1, now),
		now,
	)
	if err != nil {
		return err
	}
	if affected > 0 {
		recordFleetCommandJobEvent(job.ID, job.TenantID, nil, nil, commandJobEventTimeout, "ready or dispatching command rows timed out before delivery completed")
	}
	return refreshCommandJobSummary(job)
}

func expireTimedOutFleetCommandJobsForTenant(tenantID string) error {
	jobs, err := dal.ListTimedOutRunningCommandJobsForTenant(tenantID, time.Now().UTC(), defaultFleetCommandJobRecoveryLimit)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := expireCommandJobIfTimedOut(job); err != nil {
			return err
		}
	}
	return nil
}

func fleetCommandJobTimeoutRecoverableDetailStatuses() []string {
	return []string{commandJobDetailStatusReady, commandJobDetailStatusDispatching}
}

func (FleetCommandJobQueryService) GetFleetCommandJobSupportBundle(jobID string, claims *utils.UserClaims) (*model.FleetCommandJobSupportBundle, error) {
	job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	metrics, err := dal.GetCommandJobSummaryMetrics(job.ID, claims.TenantID, commandJobMaxDispatchAttempts, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	details, err := dal.FindCommandJobSupportDetails(job.ID, claims.TenantID, job.Status == commandJobStatusCanceled, commandJobInlineRowLimit)
	if err != nil {
		return nil, err
	}
	events, err := loadRecentCommandJobEvents(job.ID, claims.TenantID)
	if err != nil {
		events = nil
	}

	return commandJobSupportBundleFromPersistence(
		job,
		details,
		metrics.StatusCounts,
		metrics.RetryableCount,
		metrics.RetryReadyCount,
		metrics.RetryWaitingCount,
		metrics.RetryExhaustedCount,
		metrics.LogMissingCount,
		events,
	), nil
}

func loadRecentCommandJobEvents(jobID, tenantID string) ([]*model.CommandJobEvent, error) {
	return dal.GetRecentCommandJobEvents(jobID, tenantID, defaultFleetCommandJobEventLimit)
}

// fleetCommandJobListScopes 解析命令任务列表读作用域（ROADMAP C2 自上而下）：
// TENANT_USER 保持 self-only；TENANT_ADMIN/SYS_ADMIN 非空租户 → expandTenantIDScope
// （self∪子孙，链接缺失回退 self-only）；空租户 → [""]（提交路径 requireDeviceTenantClaims
// 本就拒绝空租户，平台任务不存在，映射仅保持旧行为）；nil 声明 → nil fail-closed。
// 边界决策：设备筛选预览/提交仍锚定操作员本租户（C2 仅放列表读，不扩大向子树设备
// 下发命令的写路径）；超时恢复 expireTimedOutFleetCommandJobsForTenant 保持单租户
// （有状态写，全局 worker ListTimedOutRunningCommandJobs 兜底全部租户）。
func fleetCommandJobListScopes(claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	if claims.Authority == constant.TENANT_USER {
		if tenantID := strings.TrimSpace(claims.TenantID); tenantID != "" {
			return []string{tenantID}
		}
		return nil
	}
	if strings.TrimSpace(claims.TenantID) == "" {
		return []string{""}
	}
	return expandTenantIDScope(claims.TenantID)
}

func (FleetCommandJobQueryService) ListFleetCommandJobs(req *model.FleetCommandJobListReq, claims *utils.UserClaims) (*model.FleetCommandJobListResult, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to list fleet command jobs")
	}
	req = normalizeFleetCommandJobListReq(req)
	scopes := fleetCommandJobListScopes(claims)

	if err := expireTimedOutFleetCommandJobsForTenant(claims.TenantID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	total, jobs, err := dal.ListCommandJobs(
		scopes,
		req.Status,
		req.Search,
		req.AttentionFilter,
		req.Page,
		req.PageSize,
		commandJobMaxDispatchAttempts,
		now,
	)
	if err != nil {
		return nil, err
	}

	jobIDs := commandJobIDsFromList(jobs)
	attentionMetrics, err := dal.GetCommandJobListAttentionMetrics(jobIDs, scopes, commandJobMaxDispatchAttempts, now)
	if err != nil {
		return nil, err
	}
	attentionSummary, err := dal.GetCommandJobListAttentionSummary(
		scopes,
		req.Status,
		req.Search,
		req.AttentionFilter,
		commandJobMaxDispatchAttempts,
		now,
	)
	if err != nil {
		return nil, err
	}

	return commandJobListResultFromPersistence(total, req, jobs, attentionMetrics, attentionSummary), nil
}
