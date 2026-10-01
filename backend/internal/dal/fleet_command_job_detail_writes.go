// 文件用途：批次命令作业明细的写路径：下发租约回写、响应匹配、批量状态迁移与重试策略失败处理。
// 关键注意事项：条件更新依赖 WHERE 中的状态/租约守卫实现乐观并发，调用方以 RowsAffected 判断是否命中。
package dal

import (
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func UpdateCommandJob(job *model.CommandJob) error {
	return global.DB.Save(job).Error
}

func UpdateCommandJobDetail(detail *model.CommandJobDetail) error {
	return global.DB.Save(detail).Error
}

func UpdateClaimedCommandJobDetailAfterDispatch(detail *model.CommandJobDetail, leaseToken string) (int64, error) {
	if detail == nil || detail.ID == "" || detail.TenantID == "" || leaseToken == "" {
		return 0, nil
	}
	updates := map[string]interface{}{
		"status":               detail.Status,
		"message_id":           detail.MessageID,
		"log_recorded":         detail.LogRecorded,
		"reason":               detail.Reason,
		"can_retry":            detail.CanRetry,
		"dispatch_lease_token": nil,
		"dispatch_lease_until": nil,
		"next_retry_after":     detail.NextRetryAfter,
		"updated_at":           detail.UpdatedAt,
		"submitted_at":         detail.SubmittedAt,
		"completed_at":         detail.CompletedAt,
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("id = ? AND tenant_id = ? AND status = ? AND dispatch_lease_token = ?", detail.ID, detail.TenantID, "dispatching", leaseToken).
		Updates(updates)
	return result.RowsAffected, result.Error
}

func UpdateCommandJobDetailResponseByMessageID(deviceID, messageID, status, payload, responseError string, responseAt time.Time) (*model.CommandJobDetail, int64, error) {
	if deviceID == "" || messageID == "" {
		return nil, 0, nil
	}
	matches, err := FindCommandJobDetailResponseCandidates(deviceID, messageID, 2)
	if err != nil {
		return nil, 0, err
	}
	if len(matches) == 0 {
		return nil, 0, nil
	}
	if commandJobDetailResponseMatchIsAmbiguous(len(matches)) {
		return nil, 0, ErrAmbiguousCommandJobDetailResponse
	}
	detail := *matches[0]
	if commandJobDetailResponseIsStale(detail.ResponseAt, responseAt) {
		return nil, 0, nil
	}
	updates := map[string]interface{}{
		"response_status":  status,
		"response_payload": payload,
		"response_at":      responseAt,
		"updated_at":       responseAt,
	}
	if responseError != "" {
		updates["response_error"] = responseError
	} else {
		updates["response_error"] = nil
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("id = ? AND tenant_id = ? AND (response_at IS NULL OR response_at <= ?)", detail.ID, detail.TenantID, responseAt).
		Updates(updates)
	if result.Error != nil {
		return nil, result.RowsAffected, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, 0, nil
	}
	detail.ResponseStatus = &status
	detail.ResponsePayload = &payload
	detail.ResponseAt = &responseAt
	if responseError != "" {
		detail.ResponseError = &responseError
	} else {
		detail.ResponseError = nil
	}
	return &detail, result.RowsAffected, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func FindCommandJobDetailResponseCandidates(deviceID, messageID string, limit int) ([]*model.CommandJobDetail, error) {
	if deviceID == "" || messageID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 2
	}
	var matches []*model.CommandJobDetail
	err := global.DB.
		Where("device_id = ? AND message_id = ?", deviceID, messageID).
		Order("created_at DESC").
		Limit(limit).
		Find(&matches).Error
	return matches, err
}

func commandJobDetailResponseMatchIsAmbiguous(matchCount int) bool {
	return matchCount > 1
}

func commandJobDetailResponseIsStale(current *time.Time, incoming time.Time) bool {
	return current != nil && current.After(incoming)
}

func UpdateCommandJobDetailsStatus(jobID, tenantID string, fromStatuses []string, toStatus, reason string, canRetry bool) (int64, error) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":               toStatus,
		"reason":               reason,
		"can_retry":            canRetry,
		"dispatch_lease_token": nil,
		"dispatch_lease_until": nil,
		"next_retry_after":     nil,
		"updated_at":           now,
		"completed_at":         now,
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status IN ?", jobID, tenantID, fromStatuses).
		Updates(updates)
	return result.RowsAffected, result.Error
}

func commandJobNextRetryAfterExpression(maxAttempts int, nextRetryAfter time.Time) clause.Expr {
	return gorm.Expr(
		"CASE WHEN dispatch_attempts < ? THEN CAST(? AS timestamptz) ELSE NULL END",
		maxAttempts,
		nextRetryAfter,
	)
}

func FailTimedOutCommandJobDetailsWithRetryPolicy(jobID, tenantID string, maxAttempts int, nextRetryAfter time.Time, now time.Time) (int64, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var affected int64
	err := global.DB.Transaction(func(tx *gorm.DB) error {
		preserved := tx.Model(&model.CommandJobDetail{}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND eligible = ? AND message_id IS NOT NULL AND message_id <> ''", jobID, tenantID, "dispatching", true).
			Updates(map[string]interface{}{
				"status":               "failed",
				"reason":               "job timed out after this command row received a message id; confirm command logs and device state before creating a new attempt",
				"can_retry":            false,
				"dispatch_lease_token": nil,
				"dispatch_lease_until": nil,
				"next_retry_after":     nil,
				"advice":               "A message id was already allocated before timeout; confirm command logs and device state before creating a fresh attempt.",
				"updated_at":           now,
				"completed_at":         now,
			})
		if preserved.Error != nil {
			return preserved.Error
		}
		affected += preserved.RowsAffected

		recoverable := tx.Model(&model.CommandJobDetail{}).
			Where(
				"command_job_id = ? AND tenant_id = ? AND eligible = ? AND (status = ? OR (status = ? AND (message_id IS NULL OR message_id = '')))",
				jobID,
				tenantID,
				true,
				"ready",
				"dispatching",
			).
			Updates(map[string]interface{}{
				"status":               "failed",
				"message_id":           nil,
				"log_recorded":         false,
				"reason":               "job timed out before command delivery completed",
				"can_retry":            gorm.Expr("dispatch_attempts < ?", maxAttempts),
				"dispatch_lease_token": nil,
				"dispatch_lease_until": nil,
				"next_retry_after":     commandJobNextRetryAfterExpression(maxAttempts, nextRetryAfter),
				"advice": gorm.Expr(
					"CASE WHEN dispatch_attempts < ? THEN ? ELSE ? END",
					maxAttempts,
					"Retry becomes available after a short backoff; review the timeout evidence before retrying.",
					"Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.",
				),
				"updated_at":   now,
				"submitted_at": nil,
				"completed_at": now,
			})
		if recoverable.Error != nil {
			return recoverable.Error
		}
		affected += recoverable.RowsAffected
		return nil
	})
	return affected, err
}

func FailInterruptedCommandJobDetails(jobID, tenantID string, maxAttempts int, nextRetryAfter time.Time, now time.Time) (int64, error) {
	preserved := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status = ? AND eligible = ? AND message_id IS NOT NULL AND message_id <> '' AND (dispatch_lease_until IS NULL OR dispatch_lease_until <= ?)", jobID, tenantID, "dispatching", true, now).
		Updates(map[string]interface{}{
			"status":               "failed",
			"reason":               "backend restarted after this command row received a message id; confirm device state and command logs before creating a new attempt",
			"can_retry":            false,
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
			"next_retry_after":     nil,
			"advice":               "A message id was already allocated before restart; confirm command logs and device state before creating a fresh attempt.",
			"updated_at":           now,
			"completed_at":         now,
		})
	if preserved.Error != nil {
		return 0, preserved.Error
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND status = ? AND eligible = ? AND (message_id IS NULL OR message_id = '') AND (dispatch_lease_until IS NULL OR dispatch_lease_until <= ?)", jobID, tenantID, "dispatching", true, now).
		Updates(map[string]interface{}{
			"status":               "failed",
			"message_id":           nil,
			"log_recorded":         false,
			"reason":               "backend restarted while this command row was dispatching; confirm device state before retry",
			"can_retry":            gorm.Expr("dispatch_attempts < ?", maxAttempts),
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
			"next_retry_after":     commandJobNextRetryAfterExpression(maxAttempts, nextRetryAfter),
			"advice": gorm.Expr(
				"CASE WHEN dispatch_attempts < ? THEN ? ELSE ? END",
				maxAttempts,
				"Retry becomes available after a short backoff; review the restart evidence before retrying.",
				"Maximum dispatch attempts reached; inspect device state, command logs, and support bundle evidence before creating a fresh attempt.",
			),
			"updated_at":   now,
			"submitted_at": nil,
			"completed_at": now,
		})
	return preserved.RowsAffected + result.RowsAffected, result.Error
}
