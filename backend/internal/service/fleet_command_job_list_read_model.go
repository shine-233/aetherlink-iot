package service

import (
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

func normalizeFleetCommandJobListReq(req *model.FleetCommandJobListReq) *model.FleetCommandJobListReq {
	if req == nil {
		req = &model.FleetCommandJobListReq{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultFleetCommandJobListPageSize
	}
	if req.PageSize > maxFleetCommandJobListPageSize {
		req.PageSize = maxFleetCommandJobListPageSize
	}
	req.Status = strings.TrimSpace(req.Status)
	req.AttentionFilter = normalizeFleetCommandJobListAttentionFilter(req.AttentionFilter)
	req.Search = normalizeFleetCommandJobListSearch(req.Search)
	return req
}

func commandJobIDsFromList(jobs []*model.CommandJob) []string {
	jobIDs := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		jobIDs = append(jobIDs, job.ID)
	}
	return jobIDs
}

func commandJobListResultFromPersistence(
	total int64,
	req *model.FleetCommandJobListReq,
	jobs []*model.CommandJob,
	attentionMetrics map[string]dal.CommandJobListAttentionMetrics,
	attentionSummary dal.CommandJobListAttentionMetrics,
) *model.FleetCommandJobListResult {
	return &model.FleetCommandJobListResult{
		Total:           total,
		Page:            req.Page,
		PageSize:        req.PageSize,
		Search:          req.Search,
		AttentionFilter: req.AttentionFilter,
		AttentionCounts: commandJobListAttentionCountsFromMetrics(attentionSummary),
		List:            commandJobListItemsFromPersistence(jobs, attentionMetrics),
	}
}

func commandJobListItemsFromPersistence(
	jobs []*model.CommandJob,
	attentionMetrics map[string]dal.CommandJobListAttentionMetrics,
) []model.FleetCommandJobListItem {
	items := make([]model.FleetCommandJobListItem, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		createdAt := job.CreatedAt
		updatedAt := job.UpdatedAt
		attention := attentionMetrics[job.ID]
		items = append(items, model.FleetCommandJobListItem{
			JobID:                    job.ID,
			JobType:                  job.JobType,
			ScopeType:                job.ScopeType,
			Identify:                 job.Identify,
			CommandValue:             job.CommandValue,
			TimeoutSeconds:           job.TimeoutSeconds,
			Status:                   job.Status,
			AuditRemark:              job.Remark,
			RequestedCount:           job.RequestedCount,
			EligibleCount:            job.EligibleCount,
			BlockedCount:             job.BlockedCount,
			SubmittedCount:           job.SubmittedCount,
			FailedCount:              job.FailedCount,
			RetryableCount:           attention.RetryableCount,
			RetryReadyCount:          attention.RetryReadyCount,
			RetryWaitingCount:        attention.RetryWaitingCount,
			RetryExhaustedCount:      attention.RetryExhaustedCount,
			LogMissingCount:          attention.LogMissingCount,
			DeviceAckFailedCount:     attention.DeviceAckFailedCount,
			NeedsOperatorAction:      attention.NeedsOperatorActionCount > 0,
			NeedsOperatorActionCount: attention.NeedsOperatorActionCount,
			CanCancel:                job.CanCancel,
			CanRetryFailed:           attention.RetryReadyCount > 0,
			CreatedAt:                &createdAt,
			UpdatedAt:                &updatedAt,
			ScheduledAt:              job.ScheduledAt,
			NextDispatchAt:           job.NextDispatchAt,
			TimeoutAt:                job.TimeoutAt,
		})
	}
	return items
}

func commandJobListAttentionCountsFromMetrics(metrics dal.CommandJobListAttentionMetrics) model.FleetCommandJobListAttentionCounts {
	return model.FleetCommandJobListAttentionCounts{
		RetryableCount:           metrics.RetryableCount,
		RetryReadyCount:          metrics.RetryReadyCount,
		RetryWaitingCount:        metrics.RetryWaitingCount,
		RetryExhaustedCount:      metrics.RetryExhaustedCount,
		LogMissingCount:          metrics.LogMissingCount,
		DeviceAckFailedCount:     metrics.DeviceAckFailedCount,
		BlockedCount:             metrics.BlockedCount,
		NeedsOperatorActionCount: metrics.NeedsOperatorActionCount,
	}
}

func normalizeFleetCommandJobListSearch(search string) string {
	search = strings.TrimSpace(search)
	// Keep search truncation valid for multi-byte UTF-8 input.
	runes := []rune(search)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return search
}

func normalizeFleetCommandJobListAttentionFilter(attentionFilter string) string {
	switch strings.TrimSpace(attentionFilter) {
	case "needs_operator_action", "needs_attention":
		return "needs_operator_action"
	case "retryable", "retry_ready", "retry_waiting", "retry_exhausted", "failed", "missing_log", "device_failed", "blocked", "in_progress", "canceled":
		return strings.TrimSpace(attentionFilter)
	default:
		return ""
	}
}
