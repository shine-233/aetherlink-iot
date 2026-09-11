package dal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReportDeliveryClaim struct {
	Delivery   *model.ReportScheduleDelivery
	Run        *model.ReportScheduleRun
	Token      string
	LeaseUntil time.Time
}

func ClaimReportDeliveries(ctx context.Context, limit int, leaseDuration time.Duration) ([]ReportDeliveryClaim, error) {
	if leaseDuration <= 0 {
		return nil, fmt.Errorf("report delivery lease duration must be positive")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	limit = clampReportWorkerLimit(limit)
	claims := make([]ReportDeliveryClaim, 0, limit)
	err = db.Transaction(func(tx *gorm.DB) error {
		var deliveries []*model.ReportScheduleDelivery
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= clock_timestamp())", []string{model.ReportDeliveryStatusPending, model.ReportDeliveryStatusRetrying}).
			Order("next_attempt_at ASC NULLS FIRST, created_at ASC, run_id ASC").Limit(limit).Find(&deliveries).Error; err != nil {
			return err
		}
		if len(deliveries) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, delivery := range deliveries {
			token := uuid.NewString()
			leaseUntil := now.Add(leaseDuration)
			result := tx.Model(&model.ReportScheduleDelivery{}).
				Where("run_id = ? AND status IN ? AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", delivery.RunID, []string{model.ReportDeliveryStatusPending, model.ReportDeliveryStatusRetrying}, now).
				Updates(map[string]interface{}{
					"status": model.ReportDeliveryStatusProcessing, "attempt_count": gorm.Expr("attempt_count + 1"),
					"claim_token": token, "lease_until": leaseUntil, "next_attempt_at": nil,
					"started_at": gorm.Expr("COALESCE(started_at, ?)", now), "last_error": nil, "updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				// Lost to a concurrent worker before the claim update landed. Nothing
				// was written for this row, so skip it rather than discarding the
				// whole batch.
				continue
			}
			var run model.ReportScheduleRun
			// Abort here on purpose: the row has already moved to processing, so
			// skipping would orphan a leased delivery that no worker will settle.
			// Rolling back leaves it pending/retrying for the next cycle, which is
			// better than letting the reaper mark it ambiguous.
			if err := tx.Where("id = ? AND tenant_id = ? AND generation_status = ?", delivery.RunID, delivery.TenantID, model.ReportGenerationStatusSucceeded).Take(&run).Error; err != nil {
				return err
			}
			delivery.Status, delivery.AttemptCount = model.ReportDeliveryStatusProcessing, delivery.AttemptCount+1
			delivery.ClaimToken, delivery.LeaseUntil = &token, &leaseUntil
			if delivery.StartedAt == nil {
				delivery.StartedAt = &now
			}
			claims = append(claims, ReportDeliveryClaim{Delivery: delivery, Run: &run, Token: token, LeaseUntil: leaseUntil})
		}
		return nil
	})
	return claims, err
}

func RenewReportDeliveryLease(ctx context.Context, runID, token string, leaseDuration time.Duration) (time.Time, error) {
	return renewReportLease(ctx, model.TableNameReportScheduleDelivery, "run_id", runID, "status", model.ReportDeliveryStatusProcessing, "claim_token", token, "lease_until", leaseDuration)
}

func SettleReportDeliveryAccepted(ctx context.Context, runID, token string) error {
	return settleReportDelivery(ctx, runID, token, model.ReportDeliveryStatusAccepted, "", false)
}

func SettleReportDeliveryFailed(ctx context.Context, runID, token, errorCode string) error {
	return settleReportDelivery(ctx, runID, token, model.ReportDeliveryStatusFailed, errorCode, false)
}

func RetryClaimedReportDelivery(ctx context.Context, runID, token, errorCode string) error {
	return settleReportDelivery(ctx, runID, token, model.ReportDeliveryStatusRetrying, errorCode, true)
}

// SettleReportDeliveryAmbiguous is terminal because SMTP acceptance may have occurred.
func SettleReportDeliveryAmbiguous(ctx context.Context, runID, token, errorCode string) error {
	return settleReportDelivery(ctx, runID, token, model.ReportDeliveryStatusAmbiguous, errorCode, false)
}

func publicReportDeliveryStatus(status string) string {
	switch status {
	case model.ReportDeliveryStatusAccepted:
		return model.ReportProjectedStatusSucceeded
	case model.ReportDeliveryStatusFailed:
		return model.ReportProjectedStatusFailed
	case model.ReportDeliveryStatusAmbiguous:
		return model.ReportProjectedStatusAmbiguous
	default:
		return model.ReportProjectedStatusRunning
	}
}

func settleReportDelivery(ctx context.Context, runID, token, status, errorCode string, retryable bool) error {
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("report delivery settlement identity is required")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var delivery model.ReportScheduleDelivery
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("run_id = ? AND status = ? AND claim_token = ?", runID, model.ReportDeliveryStatusProcessing, token).
			Take(&delivery).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrReportClaimLost
			}
			return err
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		if delivery.LeaseUntil == nil || !delivery.LeaseUntil.After(now) {
			return ErrReportClaimLost
		}
		if retryable && delivery.AttemptCount >= delivery.MaxAttempts {
			status = model.ReportDeliveryStatusFailed
		}
		updates := map[string]interface{}{
			"status": status, "claim_token": nil, "lease_until": nil, "next_attempt_at": nil,
			"last_error": nullableReportText(errorCode), "updated_at": now,
		}
		terminal := status != model.ReportDeliveryStatusRetrying
		if status == model.ReportDeliveryStatusRetrying {
			updates["next_attempt_at"] = now.Add(reportRetryDelay(delivery.AttemptCount))
		} else {
			updates["payload"] = nil
			updates["completed_at"] = now
			switch status {
			case model.ReportDeliveryStatusAccepted:
				updates["accepted_at"] = now
			case model.ReportDeliveryStatusFailed:
				updates["failed_at"] = now
			case model.ReportDeliveryStatusAmbiguous:
				updates["ambiguous_at"] = now
			}
		}
		result := tx.Model(&model.ReportScheduleDelivery{}).
			Where("run_id = ? AND status = ? AND claim_token = ? AND lease_until > ?", runID, model.ReportDeliveryStatusProcessing, token, now).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReportClaimLost
		}
		if !terminal {
			return nil
		}
		return tx.Model(&model.ReportSchedule{}).
			Where("id = (SELECT schedule_id FROM report_schedule_runs WHERE id = ?) AND tenant_id = ? AND last_run_id = ?", delivery.RunID, delivery.TenantID, delivery.RunID).
			Updates(map[string]interface{}{"last_status": publicReportDeliveryStatus(status), "updated_at": now}).Error
	})
}

// MarkExpiredReportDeliveriesAmbiguous never silently retries an expired SMTP claim.
func MarkExpiredReportDeliveriesAmbiguous(ctx context.Context, errorCode string) (int64, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return 0, err
	}
	var marked int64
	err = db.Transaction(func(tx *gorm.DB) error {
		var deliveries []*model.ReportScheduleDelivery
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND lease_until <= clock_timestamp()", model.ReportDeliveryStatusProcessing).
			Order("lease_until ASC, run_id ASC").Limit(maxReportWorkerBatchSize).Find(&deliveries).Error; err != nil {
			return err
		}
		if len(deliveries) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, delivery := range deliveries {
			result := tx.Model(&model.ReportScheduleDelivery{}).
				Where("run_id = ? AND status = ? AND lease_until <= ?", delivery.RunID, model.ReportDeliveryStatusProcessing, now).
				Updates(map[string]interface{}{
					"status": model.ReportDeliveryStatusAmbiguous, "last_error": nullableReportText(errorCode),
					"claim_token": nil, "lease_until": nil, "payload": nil,
					"ambiguous_at": now, "completed_at": now, "updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrReportClaimLost
			}
			if err := tx.Model(&model.ReportSchedule{}).
				Where("id = (SELECT schedule_id FROM report_schedule_runs WHERE id = ?) AND tenant_id = ? AND last_run_id = ?", delivery.RunID, delivery.TenantID, delivery.RunID).
				Updates(map[string]interface{}{"last_status": model.ReportProjectedStatusAmbiguous, "updated_at": now}).Error; err != nil {
				return err
			}
			marked++
		}
		return nil
	})
	return marked, err
}
