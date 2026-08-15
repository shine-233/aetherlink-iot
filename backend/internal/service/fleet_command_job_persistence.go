package service

import (
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

func buildPersistedFleetCommandJob(req *model.FleetCommandJobReq, preview *model.FleetCommandJobPreviewResult, operatorID string, claims *utils.UserClaims) (*model.CommandJob, []*model.CommandJobDetail, error) {
	now := time.Now().UTC()
	startAt := now
	status := commandJobStatusRunning
	var scheduledAt *time.Time
	if req.ScheduledAt != nil {
		value := req.ScheduledAt.UTC()
		scheduledAt = &value
		if value.After(now) {
			startAt = value
			status = commandJobStatusScheduled
		}
	}
	timeoutAt := startAt.Add(time.Duration(preview.TimeoutSeconds) * time.Second)
	scopeSnapshot := fleetCommandScopeSnapshot(req, preview)
	job := &model.CommandJob{
		ID:             uuid.New(),
		TenantID:       claims.TenantID,
		OperatorID:     operatorID,
		JobType:        "command",
		ScopeType:      preview.ScopeType,
		Identify:       req.Identify,
		CommandValue:   req.Value,
		TimeoutSeconds: preview.TimeoutSeconds,
		Status:         status,
		RequestedCount: preview.RequestedCount,
		EligibleCount:  preview.EligibleCount,
		BlockedCount:   preview.BlockedCount,
		CanCancel:      true,
		CanRetryFailed: true,
		ScopeSnapshot:  scopeSnapshot,
		CreatedAt:      now,
		UpdatedAt:      now,
		ScheduledAt:    scheduledAt,
		TimeoutAt:      &timeoutAt,
	}
	details := make([]*model.CommandJobDetail, 0, len(preview.Rows))
	for _, row := range preview.Rows {
		status := commandJobDetailStatusBlocked
		canRetry := false
		if row.Eligible {
			status = commandJobDetailStatusReady
		}
		details = append(details, &model.CommandJobDetail{
			ID:                    uuid.New(),
			CommandJobID:          job.ID,
			TenantID:              claims.TenantID,
			DeviceID:              row.DeviceID,
			DeviceNumber:          row.DeviceNumber,
			Name:                  row.Name,
			Online:                row.Online,
			Eligible:              row.Eligible,
			Status:                status,
			RecommendedPath:       row.RecommendedPath,
			Reason:                optionalString(row.Reason),
			Advice:                optionalString(row.Advice),
			CanRetry:              canRetry,
			TelemetryCurrentCount: row.TelemetryCurrentCount,
			LatestTelemetryKey:    row.LatestTelemetryKey,
			LatestTelemetryAt:     row.LatestTelemetryAt,
			Readiness:             jsonStringPtr(row.Readiness),
			CreatedAt:             now,
			UpdatedAt:             now,
		})
	}
	return job, details, nil
}

func fleetCommandScopeSnapshot(req *model.FleetCommandJobReq, preview *model.FleetCommandJobPreviewResult) *string {
	raw, err := json.Marshal(map[string]interface{}{
		"scope_type":         preview.ScopeType,
		"scope_source":       req.ScopeSource,
		"device_ids":         uniqueFleetDeviceIDs(req.DeviceIDs),
		"device_filter":      req.DeviceFilter,
		"expected_total":     req.ExpectedTotal,
		"current_page_count": req.CurrentPageCount,
		"max_devices":        normalizeFleetCommandDeviceFilterMaxDevices(req.MaxDevices),
		"preview_token":      preview.PreviewToken,
		"total_matched":      preview.TotalMatched,
		"requested_count":    preview.RequestedCount,
		"eligible_count":     preview.EligibleCount,
		"blocked_count":      preview.BlockedCount,
		"timeout_seconds":    preview.TimeoutSeconds,
		"scheduled_at":       req.ScheduledAt,
		"selected_version":   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil
	}
	return StringPtr(string(raw))
}

func jsonStringPtr(value interface{}) *string {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return StringPtr(string(raw))
}

func refreshCommandJobSummary(job *model.CommandJob) error {
	// Delivery and cancellation can race: a worker may hold a stale running
	// snapshot while the cancel API has already persisted a terminal state.
	// Re-read cancellation before deriving counts so a late worker refresh
	// cannot overwrite a canceled job as failed/partially_failed.
	if job != nil && job.Status != commandJobStatusCanceled {
		latest, loadErr := dal.GetCommandJobByID(job.ID, job.TenantID)
		if loadErr == nil {
			job = adoptPersistedCanceledCommandJob(job, latest)
		}
	}
	metrics, err := dal.GetCommandJobSummaryMetrics(job.ID, job.TenantID, commandJobMaxDispatchAttempts, time.Now().UTC())
	if err != nil {
		return err
	}
	counts := metrics.StatusCounts
	cancelRequested := job.Status == commandJobStatusCanceled
	job.SubmittedCount = counts[commandJobDetailStatusSubmitted]
	job.FailedCount = counts[commandJobDetailStatusFailed] + counts[commandJobDetailStatusBlocked] + counts[commandJobDetailStatusCanceled]
	job.CanCancel = false
	job.CanRetryFailed = metrics.RetryReadyCount > 0
	if cancelRequested {
		job.Status = commandJobStatusCanceled
		job.CanRetryFailed = false
	} else if counts[commandJobDetailStatusReady] > 0 || counts[commandJobDetailStatusDispatching] > 0 {
		job.Status = commandJobStatusRunning
		job.CanCancel = counts[commandJobDetailStatusReady] > 0
	} else if counts[commandJobDetailStatusCanceled] > 0 && counts[commandJobDetailStatusSubmitted] == 0 && counts[commandJobDetailStatusFailed] == 0 {
		job.Status = commandJobStatusCanceled
	} else if job.SubmittedCount == job.RequestedCount && job.FailedCount == 0 {
		job.Status = commandJobStatusCompleted
	} else if job.SubmittedCount == 0 && job.FailedCount > 0 {
		job.Status = commandJobStatusFailed
	} else if job.FailedCount > 0 {
		job.Status = commandJobStatusPartiallyFailed
	} else {
		job.Status = commandJobStatusCompleted
	}
	now := time.Now().UTC()
	job.UpdatedAt = now
	if job.SubmittedCount > 0 && job.LastSubmittedAt == nil {
		job.LastSubmittedAt = &now
	}
	if commandJobStatusIsTerminal(job.Status) {
		job.NextDispatchAt = nil
	}
	return dal.UpdateCommandJob(job)
}

func adoptPersistedCanceledCommandJob(job, latest *model.CommandJob) *model.CommandJob {
	if job != nil && job.Status != commandJobStatusCanceled && latest != nil && latest.Status == commandJobStatusCanceled {
		return latest
	}
	return job
}
