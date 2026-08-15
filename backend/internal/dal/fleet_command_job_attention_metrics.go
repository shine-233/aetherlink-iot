package dal

import (
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"
)

type CommandJobListAttentionMetrics struct {
	RetryableCount           int
	RetryReadyCount          int
	RetryWaitingCount        int
	RetryExhaustedCount      int
	LogMissingCount          int
	DeviceAckFailedCount     int
	BlockedCount             int
	NeedsOperatorActionCount int
}

type commandJobAttentionMetricsRow struct {
	CommandJobID             string
	RetryableCount           int
	RetryReadyCount          int
	RetryWaitingCount        int
	RetryExhaustedCount      int
	LogMissingCount          int
	DeviceAckFailedCount     int
	BlockedCount             int
	NeedsOperatorActionCount int
}

func commandJobAttentionMetricsSelect(includeJobID bool) string {
	fields := []string{}
	if includeJobID {
		fields = append(fields, "command_job_id")
	}
	fields = append(fields,
		commandJobRetryMetricsSelect(),
		"COALESCE(SUM(CASE WHEN response_status = ? THEN 1 ELSE 0 END), 0) AS device_ack_failed_count",
		"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS blocked_count",
		"COALESCE(SUM(CASE WHEN (can_retry = ? OR status = ? OR (status = ? AND log_recorded = ?) OR status IN ? OR response_status = ?) THEN 1 ELSE 0 END), 0) AS needs_operator_action_count",
	)
	return strings.Join(fields, ",\n")
}

func commandJobAttentionMetricsArgs(maxAttempts int, now time.Time) []interface{} {
	args := commandJobRetryMetricsArgs(maxAttempts, now)
	return append(args,
		strconv.Itoa(constant.ResponseSStatusFailed),
		"blocked",
		true,
		"failed",
		"submitted",
		false,
		[]string{"blocked", "canceled"},
		strconv.Itoa(constant.ResponseSStatusFailed),
	)
}

func commandJobListAttentionMetricsFromRow(row commandJobAttentionMetricsRow) CommandJobListAttentionMetrics {
	return CommandJobListAttentionMetrics{
		RetryableCount:           row.RetryableCount,
		RetryReadyCount:          row.RetryReadyCount,
		RetryWaitingCount:        row.RetryWaitingCount,
		RetryExhaustedCount:      row.RetryExhaustedCount,
		LogMissingCount:          row.LogMissingCount,
		DeviceAckFailedCount:     row.DeviceAckFailedCount,
		BlockedCount:             row.BlockedCount,
		NeedsOperatorActionCount: row.NeedsOperatorActionCount,
	}
}

func GetCommandJobListAttentionMetrics(jobIDs []string, tenantID string, maxAttempts int, now time.Time) (map[string]CommandJobListAttentionMetrics, error) {
	result := map[string]CommandJobListAttentionMetrics{}
	if len(jobIDs) == 0 {
		return result, nil
	}
	var rows []commandJobAttentionMetricsRow
	err := global.DB.Model(&model.CommandJobDetail{}).
		Select(commandJobAttentionMetricsSelect(true), commandJobAttentionMetricsArgs(maxAttempts, now)...).
		Where("tenant_id = ? AND command_job_id IN ?", tenantID, jobIDs).
		Group("command_job_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.CommandJobID] = commandJobListAttentionMetricsFromRow(item)
	}
	return result, nil
}

func GetCommandJobListAttentionSummary(tenantID, status, search, attentionFilter string, maxAttempts int, now time.Time) (CommandJobListAttentionMetrics, error) {
	var item commandJobAttentionMetricsRow
	matchingJobs := commandJobListBaseQuery(tenantID, status, search, attentionFilter, maxAttempts, now).Select("id")
	err := global.DB.Model(&model.CommandJobDetail{}).
		Select(commandJobAttentionMetricsSelect(false), commandJobAttentionMetricsArgs(maxAttempts, now)...).
		Where("tenant_id = ? AND command_job_id IN (?)", tenantID, matchingJobs).
		Scan(&item).Error
	if err != nil {
		return CommandJobListAttentionMetrics{}, err
	}
	return commandJobListAttentionMetricsFromRow(item), nil
}
