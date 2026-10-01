// 文件用途：批次命令作业的聚合统计（状态计数、重试指标）与可重试明细的重新入队。
// 关键注意事项：重试指标在单条聚合 SQL 中计算，避免逐状态多次 COUNT。
package dal

import (
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

func CountCommandJobDetailsByStatus(jobID, tenantID string) (map[string]int, error) {
	type row struct {
		Status string
		Count  int
	}
	var rows []row
	err := global.DB.Model(&model.CommandJobDetail{}).
		Select("status, count(*) as count").
		Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := newCommandJobDetailStatusCounts()
	for _, item := range rows {
		counts[item.Status] = item.Count
	}
	return counts, nil
}

// newCommandJobDetailStatusCounts keeps the status-count response shape
// stable when a job has no rows in one or more lifecycle states. Consumers use
// these keys to render the same summary fields for every command job.
func newCommandJobDetailStatusCounts() map[string]int {
	return map[string]int{
		"ready":       0,
		"dispatching": 0,
		"submitted":   0,
		"failed":      0,
		"blocked":     0,
		"canceled":    0,
	}
}

type CommandJobSummaryMetrics struct {
	StatusCounts        map[string]int
	RetryableCount      int
	RetryReadyCount     int
	RetryWaitingCount   int
	RetryExhaustedCount int
	LogMissingCount     int
}

func normalizeCommandJobMaxAttempts(maxAttempts int) int {
	if maxAttempts <= 0 {
		return 1
	}
	return maxAttempts
}

func commandJobRetryMetricsSelect() string {
	return strings.Join([]string{
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? THEN 1 ELSE 0 END), 0) AS retryable_count",
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? AND dispatch_attempts < ? AND (next_retry_after IS NULL OR next_retry_after <= ?) THEN 1 ELSE 0 END), 0) AS retry_ready_count",
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? AND dispatch_attempts < ? AND next_retry_after IS NOT NULL AND next_retry_after > ? THEN 1 ELSE 0 END), 0) AS retry_waiting_count",
		"COALESCE(SUM(CASE WHEN status = ? AND can_retry = ? AND dispatch_attempts >= ? THEN 1 ELSE 0 END), 0) AS retry_exhausted_count",
		"COALESCE(SUM(CASE WHEN status = ? AND log_recorded = ? THEN 1 ELSE 0 END), 0) AS log_missing_count",
	}, ",\n")
}

func commandJobRetryMetricsArgs(maxAttempts int, now time.Time) []interface{} {
	maxAttempts = normalizeCommandJobMaxAttempts(maxAttempts)
	return []interface{}{
		"failed",
		true,
		"failed",
		true,
		maxAttempts,
		now,
		"failed",
		true,
		maxAttempts,
		now,
		"failed",
		false,
		maxAttempts,
		"submitted",
		false,
	}
}

func GetCommandJobSummaryMetrics(jobID, tenantID string, maxAttempts int, now time.Time) (CommandJobSummaryMetrics, error) {
	type row struct {
		ReadyCount          int
		DispatchingCount    int
		SubmittedCount      int
		FailedCount         int
		BlockedCount        int
		CanceledCount       int
		RetryableCount      int
		RetryReadyCount     int
		RetryWaitingCount   int
		RetryExhaustedCount int
		LogMissingCount     int
	}
	var item row
	selectSQL := `COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS ready_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS dispatching_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS submitted_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS failed_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS blocked_count,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS canceled_count,
			` + commandJobRetryMetricsSelect()
	selectArgs := append(
		[]interface{}{"ready", "dispatching", "submitted", "failed", "blocked", "canceled"},
		commandJobRetryMetricsArgs(maxAttempts, now)...,
	)
	err := global.DB.Model(&model.CommandJobDetail{}).
		Select(selectSQL, selectArgs...).
		Where("command_job_id = ? AND tenant_id = ?", jobID, tenantID).
		Scan(&item).Error
	if err != nil {
		return CommandJobSummaryMetrics{}, err
	}
	return CommandJobSummaryMetrics{
		StatusCounts: map[string]int{
			"ready":       item.ReadyCount,
			"dispatching": item.DispatchingCount,
			"submitted":   item.SubmittedCount,
			"failed":      item.FailedCount,
			"blocked":     item.BlockedCount,
			"canceled":    item.CanceledCount,
		},
		RetryableCount:      item.RetryableCount,
		RetryReadyCount:     item.RetryReadyCount,
		RetryWaitingCount:   item.RetryWaitingCount,
		RetryExhaustedCount: item.RetryExhaustedCount,
		LogMissingCount:     item.LogMissingCount,
	}, nil
}

type CommandJobRequeueResult struct {
	Requeued    int64
	CoolingDown int64
	Exhausted   int64
}

func RequeueRetryableCommandJobDetails(jobID, tenantID string, maxAttempts int, now time.Time) (CommandJobRequeueResult, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	result := CommandJobRequeueResult{}
	err := global.DB.Transaction(func(tx *gorm.DB) error {
		exhausted := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND can_retry = ? AND dispatch_attempts >= ?", jobID, tenantID, "failed", true, maxAttempts).
			Updates(map[string]interface{}{
				"can_retry":        false,
				"next_retry_after": nil,
				"advice":           "Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.",
				"updated_at":       now,
			})
		if exhausted.Error != nil {
			return exhausted.Error
		}
		result.Exhausted = exhausted.RowsAffected

		if err := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND can_retry = ? AND dispatch_attempts < ? AND next_retry_after IS NOT NULL AND next_retry_after > ?", jobID, tenantID, "failed", true, maxAttempts, now).
			Count(&result.CoolingDown).Error; err != nil {
			return err
		}

		requeued := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND can_retry = ? AND dispatch_attempts < ? AND (next_retry_after IS NULL OR next_retry_after <= ?)", jobID, tenantID, "failed", true, maxAttempts, now).
			Updates(map[string]interface{}{
				"status":               "ready",
				"message_id":           nil,
				"log_recorded":         false,
				"reason":               "queued for retry",
				"can_retry":            false,
				"dispatch_lease_token": nil,
				"dispatch_lease_until": nil,
				"next_retry_after":     nil,
				"updated_at":           now,
				"submitted_at":         nil,
				"completed_at":         nil,
			})
		if requeued.Error != nil {
			return requeued.Error
		}
		result.Requeued = requeued.RowsAffected
		return nil
	})
	return result, err
}
