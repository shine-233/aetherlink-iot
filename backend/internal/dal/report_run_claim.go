package dal

// 文件用途：报表生成的租约生命周期（claim → renew → settle / complete → reap）。
// 核心逻辑：所有状态迁移都是"带 claim_token + lease_until 谓词的条件 UPDATE"，
//   RowsAffected != 1 即视为租约已失（ErrReportClaimLost），保证同一 run 不会被两个 worker 同时推进。
// 关键注意事项：
//   - claim 扫描用 SKIP LOCKED，被别人抢走的行要 continue 而不是回滚整批（否则整轮停滞）。
//   - 终态（失败）也必须推进 schedule 摘要：last_run_id 只在生成成功时写入，
//     若不加 IfCurrent 守卫回滚会让计划继续对外宣称上一次运行的结果。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ClaimReportGenerations(ctx context.Context, limit int, leaseDuration time.Duration) ([]ReportGenerationClaim, error) {
	if leaseDuration <= 0 {
		return nil, fmt.Errorf("report generation claim policy is invalid")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	limit = clampReportWorkerLimit(limit)
	claims := make([]ReportGenerationClaim, 0, limit)
	err = db.Transaction(func(tx *gorm.DB) error {
		var runs []*model.ReportScheduleRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("generation_status IN ? AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= clock_timestamp())", []string{model.ReportGenerationStatusPending, model.ReportGenerationStatusRetrying}).
			Order("next_attempt_at ASC NULLS FIRST, created_at ASC, id ASC").Limit(limit).Find(&runs).Error; err != nil {
			return err
		}
		if len(runs) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, run := range runs {
			token := uuid.NewString()
			leaseUntil := now.Add(leaseDuration)
			result := tx.Model(&model.ReportScheduleRun{}).
				Where("id = ? AND generation_status IN ? AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", run.ID, []string{model.ReportGenerationStatusPending, model.ReportGenerationStatusRetrying}, now).
				Updates(map[string]interface{}{
					"generation_status": model.ReportGenerationStatusProcessing, "attempt_count": gorm.Expr("attempt_count + 1"),
					"claim_token": token, "lease_until": leaseUntil, "next_attempt_at": nil,
					"started_at": gorm.Expr("COALESCE(started_at, ?)", now), "error_code": nil, "error_message": nil, "updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				// Another worker took this row between the SKIP LOCKED scan and the
				// claim update. Nothing was modified, so skip it instead of aborting:
				// aborting here used to roll back every claim already made in the
				// batch and stall the whole cycle. The row keeps its next_attempt_at,
				// so it is picked up again on a later pass.
				continue
			}
			run.GenerationStatus, run.AttemptCount = model.ReportGenerationStatusProcessing, run.AttemptCount+1
			run.ClaimToken, run.LeaseUntil = &token, &leaseUntil
			if run.StartedAt == nil {
				run.StartedAt = &now
			}
			claims = append(claims, ReportGenerationClaim{Run: run, Token: token, LeaseUntil: leaseUntil})
		}
		return nil
	})
	return claims, err
}

func RenewReportGenerationLease(ctx context.Context, runID, token string, leaseDuration time.Duration) (time.Time, error) {
	return renewReportLease(ctx, model.TableNameReportScheduleRun, "id", runID, "generation_status", model.ReportGenerationStatusProcessing, "claim_token", token, "lease_until", leaseDuration)
}

func renewReportLease(ctx context.Context, table, idColumn, id, statusColumn, status, tokenColumn, token, leaseColumn string, leaseDuration time.Duration) (time.Time, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(token) == "" || leaseDuration <= 0 {
		return time.Time{}, fmt.Errorf("report lease renewal identity and duration are required")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return time.Time{}, err
	}
	var renewedUntil time.Time
	err = db.Transaction(func(tx *gorm.DB) error {
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		renewedUntil = now.Add(leaseDuration)
		result := tx.Table(table).Where(idColumn+" = ? AND "+statusColumn+" = ? AND "+tokenColumn+" = ? AND "+leaseColumn+" > ?", id, status, token, now).
			Updates(map[string]interface{}{leaseColumn: renewedUntil, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReportClaimLost
		}
		return nil
	})
	return renewedUntil, err
}

func FailClaimedReportGeneration(ctx context.Context, runID, token, errorCode string) error {
	return settleClaimedReportGenerationError(ctx, runID, token, errorCode, "", false)
}

func RetryClaimedReportGeneration(ctx context.Context, runID, token, errorCode, errorMessage string) error {
	return settleClaimedReportGenerationError(ctx, runID, token, errorCode, errorMessage, true)
}

func settleClaimedReportGenerationError(ctx context.Context, runID, token, errorCode, errorMessage string, retryable bool) error {
	db, err := reportDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var run model.ReportScheduleRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND generation_status = ? AND claim_token = ?", runID, model.ReportGenerationStatusProcessing, token).
			Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReportClaimLost
			}
			return err
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		if run.LeaseUntil == nil || !run.LeaseUntil.After(now) {
			return ErrReportClaimLost
		}
		status := model.ReportGenerationStatusFailed
		updates := map[string]interface{}{
			"generation_status": status, "error_code": strings.TrimSpace(errorCode), "error_message": nullableReportText(errorMessage),
			"claim_token": nil, "lease_until": nil, "next_attempt_at": nil, "completed_at": now, "updated_at": now,
		}
		if retryable && run.AttemptCount < run.MaxAttempts {
			status = model.ReportGenerationStatusRetrying
			updates["generation_status"], updates["next_attempt_at"], updates["completed_at"] = status, now.Add(reportRetryDelay(run.AttemptCount)), nil
		}
		result := tx.Model(&model.ReportScheduleRun{}).
			Where("id = ? AND generation_status = ? AND claim_token = ? AND lease_until > ?", run.ID, model.ReportGenerationStatusProcessing, token, now).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReportClaimLost
		}
		if status == model.ReportGenerationStatusFailed {
			// Advance the summary even on failure: last_run_id is only written by
			// generation success, so the `IfCurrent` guard would silently no-op and
			// leave the schedule advertising the previous run's outcome.
			return updateReportScheduleSummaryTx(tx, &run, model.ReportProjectedStatusFailed, now)
		}
		return nil
	})
}

func nullableReportText(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func reportRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	baseDelay := reportRetryBaseDelay
	if configured := viper.GetDuration("reports.worker.retry_base_delay"); configured > 0 {
		baseDelay = configured
	}
	maxDelay := reportRetryMaxDelay
	if configured := viper.GetDuration("reports.worker.retry_max_delay"); configured > 0 {
		maxDelay = configured
	}
	delay := baseDelay
	for index := 1; index < attempt && delay < maxDelay; index++ {
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	return delay
}

// CompleteReportGeneration atomically fences generation success and inserts the full immutable outbox row.
func CompleteReportGeneration(ctx context.Context, runID, token, envelopeFrom string, recipients []string, messageID, subject string, payload []byte, rows int64) error {
	if strings.TrimSpace(envelopeFrom) == "" || len(recipients) == 0 || strings.TrimSpace(messageID) == "" || strings.TrimSpace(subject) == "" || len(payload) == 0 || rows < 0 {
		return fmt.Errorf("report generation artifact or delivery envelope is invalid")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	digestHex := hex.EncodeToString(digest[:])
	return db.Transaction(func(tx *gorm.DB) error {
		var run model.ReportScheduleRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND generation_status = ? AND claim_token = ?", runID, model.ReportGenerationStatusProcessing, token).
			Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReportClaimLost
			}
			return err
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		if run.LeaseUntil == nil || !run.LeaseUntil.After(now) {
			return ErrReportClaimLost
		}
		generationResult := &model.ReportGenerationResult{PayloadDigest: digestHex, PayloadSize: int64(len(payload)), RowCount: rows}
		settled := tx.Model(&model.ReportScheduleRun{}).
			Where("id = ? AND generation_status = ? AND claim_token = ? AND lease_until > ?", run.ID, model.ReportGenerationStatusProcessing, token, now).
			Updates(map[string]interface{}{
				"generation_status": model.ReportGenerationStatusSucceeded, "claim_token": nil, "lease_until": nil,
				"next_attempt_at": nil, "result": generationResult, "error_code": nil, "error_message": nil,
				"generated_at": now, "completed_at": now, "updated_at": now,
			})
		if settled.Error != nil {
			return settled.Error
		}
		if settled.RowsAffected != 1 {
			return ErrReportClaimLost
		}
		delivery := &model.ReportScheduleDelivery{
			RunID: run.ID, TenantID: run.TenantID, EnvelopeFrom: strings.TrimSpace(envelopeFrom),
			EnvelopeRecipients: append([]string(nil), recipients...), MessageID: strings.TrimSpace(messageID), Subject: strings.TrimSpace(subject),
			Payload: append([]byte(nil), payload...), PayloadDigest: digestHex, PayloadSize: int64(len(payload)), RowCount: rows,
			Status: model.ReportDeliveryStatusPending, MaxAttempts: run.MaxAttempts, NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
		}
		if delivery.MaxAttempts < 1 {
			delivery.MaxAttempts = defaultReportMaxAttempts
		}
		if err := tx.Create(delivery).Error; err != nil {
			return err
		}
		return updateReportScheduleSummaryTx(tx, &run, model.ReportProjectedStatusQueued, now)
	})
}

func ReapExpiredReportGenerations(ctx context.Context, errorCode string) (int64, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return 0, err
	}
	var reaped int64
	err = db.Transaction(func(tx *gorm.DB) error {
		var runs []*model.ReportScheduleRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("generation_status = ? AND lease_until <= clock_timestamp()", model.ReportGenerationStatusProcessing).
			Order("lease_until ASC, id ASC").Limit(maxReportWorkerBatchSize).Find(&runs).Error; err != nil {
			return err
		}
		if len(runs) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, run := range runs {
			terminal := run.AttemptCount >= run.MaxAttempts
			status := model.ReportGenerationStatusRetrying
			updates := map[string]interface{}{
				"generation_status": status, "error_code": strings.TrimSpace(errorCode), "claim_token": nil, "lease_until": nil,
				"next_attempt_at": now.Add(reportRetryDelay(run.AttemptCount)), "completed_at": nil, "updated_at": now,
			}
			if terminal {
				status = model.ReportGenerationStatusFailed
				updates["generation_status"], updates["next_attempt_at"], updates["completed_at"] = status, nil, now
			}
			result := tx.Model(&model.ReportScheduleRun{}).
				Where("id = ? AND generation_status = ? AND lease_until <= ?", run.ID, model.ReportGenerationStatusProcessing, now).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrReportClaimLost
			}
			if terminal {
				if err := updateReportScheduleSummaryTx(tx, run, model.ReportProjectedStatusFailed, now); err != nil {
					return err
				}
			}
			reaped++
		}
		return nil
	})
	return reaped, err
}

func updateReportScheduleSummaryTx(tx *gorm.DB, run *model.ReportScheduleRun, status string, now time.Time) error {
	return tx.Model(&model.ReportSchedule{}).
		Where(`id = ? AND tenant_id = ? AND deleted_at IS NULL AND (
			last_run_id IS NULL OR EXISTS (
				SELECT 1 FROM report_schedule_runs current_run
				WHERE current_run.id = report_schedules.last_run_id
				  AND current_run.tenant_id = report_schedules.tenant_id
				  AND current_run.schedule_id = report_schedules.id
				  AND (current_run.created_at < ? OR (current_run.created_at = ? AND current_run.id < ?))
			)
		)`, run.ScheduleID, run.TenantID, run.CreatedAt, run.CreatedAt, run.ID).
		Updates(map[string]interface{}{"last_run_id": run.ID, "last_run_at": run.WindowEndAt, "last_status": status, "updated_at": now}).Error
}
