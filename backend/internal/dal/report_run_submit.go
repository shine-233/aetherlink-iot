package dal

// 文件用途：报表运行的幂等提交路径（手动触发 / 失败重试）。
// 核心逻辑：两条提交路径共用同一套幂等协议——对 (tenant_id, schedule_id, trigger, idempotency_key_hash)
//   先查重、查不到再 INSERT ... ON CONFLICT DO NOTHING，靠唯一索引兜底并发重复提交；
//   命中既有行时回读该行并投影其真实状态，绝不把已终止的重放当作新排队任务返回。
// 关键注意事项：
//   - 请求指纹不一致 = 同一幂等键换了参数，必须报 ErrReportIdempotencyConflict（不能静默复用旧运行）。
//   - 重试前必须用 FOR UPDATE 锁住父运行并确认其处于可重试终态，否则返回 ErrReportRunNotRetryable。

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func SubmitManualReportRun(ctx context.Context, input ManualReportRunInput) (*ReportRunSubmission, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.TenantID) == "" || strings.TrimSpace(input.ScheduleID) == "" {
		return nil, fmt.Errorf("manual report run identity is incomplete")
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, fmt.Errorf("manual report run idempotency key is required")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	keyHash := reportHexHash(HashReportIdempotencyKey(input.IdempotencyKey))
	fingerprint := reportHexHash(HashReportRequestFingerprint(input.RequestFingerprint))
	var submission ReportRunSubmission
	err = db.Transaction(func(tx *gorm.DB) error {
		replayed, replayedStatus, replayErr := findIdempotentReportRun(tx, input.TenantID, input.ScheduleID, model.ReportRunTriggerManual, keyHash, fingerprint)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			submission.Run, submission.IdempotentReplay = replayed, true
			submission.OverallStatus = replayedStatus
			return nil
		}
		var schedule model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL", input.ScheduleID, input.TenantID, true).
			Take(&schedule).Error; err != nil {
			return err
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		windowEnd := now
		windowStart := windowEnd.Add(-time.Duration(schedule.LookbackHours) * time.Hour)
		run := snapshotReportRun(&schedule, input.ID, model.ReportRunTriggerManual, nil, nil, windowStart, windowEnd, 0, input.MaxAttempts, now)
		run.IdempotencyKeyHash, run.RequestFingerprint = &keyHash, &fingerprint
		return createIdempotentReportRun(tx, run, &submission)
	})
	return &submission, err
}

func SubmitRetryReportRun(ctx context.Context, input RetryReportRunInput) (*ReportRunSubmission, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.TenantID) == "" || strings.TrimSpace(input.ScheduleID) == "" || strings.TrimSpace(input.ParentRunID) == "" {
		return nil, fmt.Errorf("retry report run identity is incomplete")
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, fmt.Errorf("retry report run idempotency key is required")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	keyHash := reportHexHash(HashReportIdempotencyKey(input.IdempotencyKey))
	fingerprint := reportHexHash(HashReportRequestFingerprint(input.RequestFingerprint))
	var submission ReportRunSubmission
	err = db.Transaction(func(tx *gorm.DB) error {
		replayed, replayedStatus, replayErr := findIdempotentReportRun(tx, input.TenantID, input.ScheduleID, model.ReportRunTriggerRetry, keyHash, fingerprint)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			submission.Run, submission.IdempotentReplay = replayed, true
			submission.OverallStatus = replayedStatus
			submission.DuplicateDeliveryRisk, replayErr = reportRunDuplicateDeliveryRisk(tx, replayed)
			return replayErr
		}
		var schedule model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL", input.ScheduleID, input.TenantID, true).
			Take(&schedule).Error; err != nil {
			return err
		}
		var parent model.ReportScheduleRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND schedule_id = ? AND tenant_id = ?", input.ParentRunID, input.ScheduleID, input.TenantID).
			Take(&parent).Error; err != nil {
			return err
		}
		var delivery model.ReportScheduleDelivery
		deliveryErr := tx.Where("run_id = ? AND tenant_id = ?", parent.ID, input.TenantID).Take(&delivery).Error
		retryable := parent.GenerationStatus == model.ReportGenerationStatusFailed && errors.Is(deliveryErr, gorm.ErrRecordNotFound)
		if deliveryErr == nil {
			retryable = parent.GenerationStatus == model.ReportGenerationStatusSucceeded &&
				(delivery.Status == model.ReportDeliveryStatusFailed || delivery.Status == model.ReportDeliveryStatusAmbiguous)
		} else if !errors.Is(deliveryErr, gorm.ErrRecordNotFound) {
			return deliveryErr
		}
		if !retryable {
			return ErrReportRunNotRetryable
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		parentID := parent.ID
		run := &model.ReportScheduleRun{
			ID: input.ID, TenantID: parent.TenantID, ScheduleID: parent.ScheduleID, RetryParentRunID: &parentID,
			Trigger: model.ReportRunTriggerRetry, WindowStartAt: parent.WindowStartAt, WindowEndAt: parent.WindowEndAt,
			ConfigSnapshot: cloneReportConfigSnapshot(parent.ConfigSnapshot), GenerationStatus: model.ReportGenerationStatusPending,
			MaxAttempts: input.MaxAttempts, NextAttemptAt: &now,
			IdempotencyKeyHash: &keyHash, RequestFingerprint: &fingerprint, CreatedAt: now, UpdatedAt: now,
		}
		if run.MaxAttempts < 1 {
			run.MaxAttempts = parent.MaxAttempts
		}
		if run.MaxAttempts < 1 {
			run.MaxAttempts = defaultReportMaxAttempts
		}
		submission.DuplicateDeliveryRisk = deliveryErr == nil && delivery.Status == model.ReportDeliveryStatusAmbiguous
		return createIdempotentReportRun(tx, run, &submission)
	})
	return &submission, err
}

func cloneReportConfigSnapshot(snapshot model.ReportRunConfigSnapshot) model.ReportRunConfigSnapshot {
	snapshot.DeviceIDs = append([]string(nil), snapshot.DeviceIDs...)
	snapshot.Keys = append([]string(nil), snapshot.Keys...)
	return snapshot
}

// findIdempotentReportRun returns the replayed run together with its real
// projected status, so a replay of an already-terminal run cannot be reported as
// freshly queued work.
func findIdempotentReportRun(tx *gorm.DB, tenantID, scheduleID, trigger, keyHash, fingerprint string) (*model.ReportScheduleRun, string, error) {
	var existing model.ReportScheduleRun
	err := tx.Where("tenant_id = ? AND schedule_id = ? AND trigger = ? AND idempotency_key_hash = ?", tenantID, scheduleID, trigger, keyHash).
		Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if existing.RequestFingerprint == nil || *existing.RequestFingerprint != fingerprint {
		return nil, "", ErrReportIdempotencyConflict
	}
	status, statusErr := projectReportRunStatus(tx, &existing)
	if statusErr != nil {
		return nil, "", statusErr
	}
	return &existing, status, nil
}

// projectReportRunStatus resolves the delivery row for a run and projects the
// public overall status, defaulting delivery to pending when no row exists yet.
func projectReportRunStatus(tx *gorm.DB, run *model.ReportScheduleRun) (string, error) {
	deliveryStatus := model.ReportDeliveryStatusPending
	var delivery model.ReportScheduleDelivery
	err := tx.Select("status").Where("run_id = ? AND tenant_id = ?", run.ID, run.TenantID).Take(&delivery).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if err == nil {
		deliveryStatus = delivery.Status
	}
	return model.ProjectReportStatus(run.GenerationStatus, deliveryStatus), nil
}

func reportRunDuplicateDeliveryRisk(tx *gorm.DB, run *model.ReportScheduleRun) (bool, error) {
	if run == nil || run.RetryParentRunID == nil {
		return false, nil
	}
	var parentDelivery model.ReportScheduleDelivery
	err := tx.Select("status").Where("run_id = ? AND tenant_id = ?", *run.RetryParentRunID, run.TenantID).Take(&parentDelivery).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return parentDelivery.Status == model.ReportDeliveryStatusAmbiguous, err
}

func createIdempotentReportRun(tx *gorm.DB, run *model.ReportScheduleRun, submission *ReportRunSubmission) error {
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(run)
	if created.Error != nil {
		return created.Error
	}
	if created.RowsAffected == 1 {
		submission.Run = run
		// A freshly inserted run has no delivery row yet, so the projection is
		// derived from the pending generation status alone.
		submission.OverallStatus = model.ProjectReportStatus(run.GenerationStatus, model.ReportDeliveryStatusPending)
		return nil
	}
	var existing model.ReportScheduleRun
	if err := tx.Where("tenant_id = ? AND schedule_id = ? AND trigger = ? AND idempotency_key_hash = ?", run.TenantID, run.ScheduleID, run.Trigger, run.IdempotencyKeyHash).Take(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReportIdempotencyConflict
		}
		return err
	}
	if existing.RequestFingerprint == nil || run.RequestFingerprint == nil ||
		!bytes.Equal([]byte(*existing.RequestFingerprint), []byte(*run.RequestFingerprint)) {
		return ErrReportIdempotencyConflict
	}
	status, statusErr := projectReportRunStatus(tx, &existing)
	if statusErr != nil {
		return statusErr
	}
	submission.Run = &existing
	submission.IdempotentReplay = true
	submission.OverallStatus = status
	return nil
}
