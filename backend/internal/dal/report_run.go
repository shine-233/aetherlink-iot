package dal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	reportOccurrenceScanCap     = 10000
	reportOccurrenceScanHorizon = 10 * 365 * 24 * time.Hour
	reportRetryBaseDelay        = time.Minute
	reportRetryMaxDelay         = time.Hour
	defaultReportMaxAttempts    = 3
)

type ReportNextOccurrence func(schedule *model.ReportSchedule, after time.Time) (time.Time, error)

type ReportScheduleInitializationResult struct {
	Initialized int
	Invalid     int
}

type ReportMaterializationResult struct {
	Run               *model.ReportScheduleRun
	CoalescedMisfires int
}

type ReportGenerationClaim struct {
	Run        *model.ReportScheduleRun
	Token      string
	LeaseUntil time.Time
}

type ReportRunSubmission struct {
	Run                   *model.ReportScheduleRun
	IdempotentReplay      bool
	DuplicateDeliveryRisk bool
	// OverallStatus is the real projection of Run at submission time. A replay can
	// return a run that already reached a terminal state, so callers must not
	// assume a fresh submission.
	OverallStatus string
}

type ManualReportRunInput struct {
	ID                 string
	TenantID           string
	ScheduleID         string
	IdempotencyKey     string
	RequestFingerprint []byte
	MaxAttempts        int
	// Deprecated caller-clock fields are intentionally ignored; DB time owns the window.
	WindowStartAt time.Time
	WindowEndAt   time.Time
}

type RetryReportRunInput struct {
	ID                 string
	TenantID           string
	ScheduleID         string
	ParentRunID        string
	IdempotencyKey     string
	RequestFingerprint []byte
	MaxAttempts        int
}

func HashReportIdempotencyKey(value string) [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.TrimSpace(value)))
}

func HashReportRequestFingerprint(value []byte) [sha256.Size]byte {
	return sha256.Sum256(value)
}

func reportHexHash(value [sha256.Size]byte) string { return hex.EncodeToString(value[:]) }

func InitializeReportScheduleNextRuns(ctx context.Context, limit int, next ReportNextOccurrence) (ReportScheduleInitializationResult, error) {
	if next == nil {
		return ReportScheduleInitializationResult{}, fmt.Errorf("report next-occurrence callback is required")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return ReportScheduleInitializationResult{}, err
	}
	limit = clampReportWorkerLimit(limit)
	result := ReportScheduleInitializationResult{}
	err = db.Transaction(func(tx *gorm.DB) error {
		var schedules []*model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("enabled = ? AND deleted_at IS NULL AND next_run_at IS NULL", true).
			Order("created_at ASC, id ASC").Limit(limit).Find(&schedules).Error; err != nil {
			return err
		}
		if len(schedules) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, schedule := range schedules {
			occurrence, nextErr := next(schedule, now)
			if nextErr != nil || occurrence.IsZero() || !occurrence.After(now) {
				if updateErr := tx.Model(&model.ReportSchedule{}).
					Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at IS NULL", schedule.ID, schedule.TenantID, true).
					Updates(map[string]interface{}{"enabled": false, "schedule_error_code": "invalid_schedule", "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error; updateErr != nil {
					return updateErr
				}
				result.Invalid++
				continue
			}
			if updateErr := tx.Model(&model.ReportSchedule{}).
				Where("id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at IS NULL", schedule.ID, true).
				Updates(map[string]interface{}{"next_run_at": occurrence.UTC(), "updated_at": now}).Error; updateErr != nil {
				return updateErr
			}
			result.Initialized++
		}
		return nil
	})
	return result, err
}

func MaterializeDueReportScheduleRuns(ctx context.Context, limit, maxAttempts int, next ReportNextOccurrence) ([]ReportMaterializationResult, error) {
	if next == nil {
		return nil, fmt.Errorf("report next-occurrence callback is required")
	}
	if maxAttempts < 1 {
		return nil, fmt.Errorf("report generation max attempts must be positive")
	}
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	limit = clampReportWorkerLimit(limit)
	materialized := make([]ReportMaterializationResult, 0, limit)
	err = db.Transaction(func(tx *gorm.DB) error {
		var schedules []*model.ReportSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("enabled = ? AND deleted_at IS NULL AND next_run_at IS NOT NULL", true).
			Order("next_run_at ASC, id ASC").Limit(limit).Find(&schedules).Error; err != nil {
			return err
		}
		if len(schedules) == 0 {
			return nil
		}
		now, err := reportDatabaseNow(tx)
		if err != nil {
			return err
		}
		for _, schedule := range schedules {
			if schedule.NextRunAt == nil || schedule.NextRunAt.After(now) {
				continue
			}
			cursor := schedule.NextRunAt.UTC()
			firstFuture, missed, lastMissed, nextErr := firstFutureReportOccurrenceBounded(schedule, cursor, now, next)
			if nextErr != nil {
				if quarantineErr := quarantineInvalidReportScheduleTx(tx, schedule, cursor, now); quarantineErr != nil {
					return quarantineErr
				}
				continue
			}
			slot := lastMissed.UTC()
			windowStart := slot.Add(-time.Duration(schedule.LookbackHours) * time.Hour)
			run := snapshotReportRun(schedule, uuid.NewString(), model.ReportRunTriggerScheduled, nil, &slot, windowStart, slot, missed-1, maxAttempts, now)
			if missed > 1 {
				first, last := cursor, slot
				run.MisfireFirstSlot, run.MisfireLastSlot = &first, &last
			}
			create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(run)
			if create.Error != nil {
				return create.Error
			}
			// Keep the tenant predicate: the sibling statements in this function all
			// scope by tenant_id, and dropping it here is how a constraint drift
			// turns into a cross-tenant write if IDs ever stop being globally unique.
			if updateErr := tx.Model(&model.ReportSchedule{}).
				Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at = ?", schedule.ID, schedule.TenantID, true, cursor).
				Updates(map[string]interface{}{"next_run_at": firstFuture, "updated_at": now}).Error; updateErr != nil {
				return updateErr
			}
			if create.RowsAffected == 1 {
				materialized = append(materialized, ReportMaterializationResult{Run: run, CoalescedMisfires: missed - 1})
			}
		}
		return nil
	})
	return materialized, err
}

func quarantineInvalidReportScheduleTx(tx *gorm.DB, schedule *model.ReportSchedule, cursor, now time.Time) error {
	result := tx.Model(&model.ReportSchedule{}).
		Where("id = ? AND tenant_id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at = ?", schedule.ID, schedule.TenantID, true, cursor).
		Updates(map[string]interface{}{
			"enabled": false, "next_run_at": nil, "schedule_error_code": "invalid_schedule",
			"revision": gorm.Expr("revision + 1"), "updated_at": now,
		})
	return result.Error
}

func firstFutureReportOccurrence(schedule *model.ReportSchedule, cursor, now time.Time, next ReportNextOccurrence) (time.Time, int, error) {
	future, missed, _, err := firstFutureReportOccurrenceBounded(schedule, cursor, now, next)
	return future, missed, err
}

func firstFutureReportOccurrenceBounded(schedule *model.ReportSchedule, cursor, now time.Time, next ReportNextOccurrence) (time.Time, int, time.Time, error) {
	first := cursor.UTC()
	if now.Sub(first) > reportOccurrenceScanHorizon {
		return time.Time{}, 0, time.Time{}, fmt.Errorf("report schedule %s stale occurrence exceeds scan horizon", schedule.ID)
	}
	missed, lastMissed := 1, first
	for !cursor.After(now) {
		if missed > reportOccurrenceScanCap {
			return time.Time{}, 0, time.Time{}, fmt.Errorf("report schedule %s stale occurrence scan limit exceeded", schedule.ID)
		}
		following, err := next(schedule, cursor)
		if err != nil {
			return time.Time{}, 0, time.Time{}, err
		}
		if following.IsZero() || !following.After(cursor) {
			return time.Time{}, 0, time.Time{}, fmt.Errorf("report schedule %s next occurrence did not advance", schedule.ID)
		}
		cursor = following.UTC()
		if !cursor.After(now) {
			missed++
			lastMissed = cursor
		}
	}
	return cursor, missed, lastMissed, nil
}

func snapshotReportRun(schedule *model.ReportSchedule, id, trigger string, parentID *string, scheduledSlot *time.Time, windowStart, windowEnd time.Time, misfires, maxAttempts int, now time.Time) *model.ReportScheduleRun {
	if maxAttempts < 1 {
		maxAttempts = defaultReportMaxAttempts
	}
	snapshot := model.ReportRunConfigSnapshot{
		ScheduleName: schedule.Name, Recipients: schedule.Recipients,
		DeviceIDs: append([]string(nil), schedule.DeviceIDs...), Keys: append([]string(nil), schedule.Keys...),
		Format: schedule.Format,
	}
	return &model.ReportScheduleRun{
		ID: id, TenantID: schedule.TenantID, ScheduleID: schedule.ID, RetryParentRunID: parentID,
		Trigger: trigger, ScheduledSlot: scheduledSlot, MisfireCount: misfires,
		WindowStartAt: windowStart.UTC(), WindowEndAt: windowEnd.UTC(), ConfigSnapshot: snapshot,
		GenerationStatus: model.ReportGenerationStatusPending, MaxAttempts: maxAttempts,
		NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
	}
}

func reportDatabaseNow(tx *gorm.DB) (time.Time, error) {
	var now time.Time
	err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error
	return now.UTC(), err
}

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
	delay := reportRetryBaseDelay
	for index := 1; index < attempt && delay < reportRetryMaxDelay; index++ {
		delay *= 2
		if delay > reportRetryMaxDelay {
			delay = reportRetryMaxDelay
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

func ListReportRunsContext(ctx context.Context, tenantID, scheduleID string, request model.ReportRunListRequest) (*model.ReportRunListResponse, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureReportScheduleTenantScope(db, tenantID, scheduleID); err != nil {
		return nil, err
	}
	page, pageSize := normalizeReportPage(request.Page, request.PageSize)
	query := db.Model(&model.ReportScheduleRun{}).Where("tenant_id = ? AND schedule_id = ?", tenantID, scheduleID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var runs []*model.ReportScheduleRun
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&runs).Error; err != nil {
		return nil, err
	}
	responses := make([]*model.ReportRunResponse, 0, len(runs))
	for _, run := range runs {
		var delivery model.ReportScheduleDelivery
		deliveryErr := db.Where("run_id = ? AND tenant_id = ?", run.ID, tenantID).Take(&delivery).Error
		if deliveryErr != nil && !errors.Is(deliveryErr, gorm.ErrRecordNotFound) {
			return nil, deliveryErr
		}
		if errors.Is(deliveryErr, gorm.ErrRecordNotFound) {
			responses = append(responses, run.ToResponse(nil))
		} else {
			responses = append(responses, run.ToResponse(&delivery))
		}
	}
	return &model.ReportRunListResponse{List: responses, Total: total, Page: page, PageSize: pageSize}, nil
}

func GetReportRunContext(ctx context.Context, tenantID, scheduleID, runID string) (*model.ReportRunResponse, error) {
	db, err := reportDB(ctx)
	if err != nil {
		return nil, err
	}
	var run model.ReportScheduleRun
	if err := db.Where("id = ? AND schedule_id = ? AND tenant_id = ?", runID, scheduleID, tenantID).Take(&run).Error; err != nil {
		return nil, err
	}
	var delivery model.ReportScheduleDelivery
	err = db.Where("run_id = ? AND tenant_id = ?", runID, tenantID).Take(&delivery).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return run.ToResponse(nil), nil
	}
	if err != nil {
		return nil, err
	}
	return run.ToResponse(&delivery), nil
}

func ensureReportScheduleTenantScope(db *gorm.DB, tenantID, scheduleID string) error {
	var count int64
	if err := db.Model(&model.ReportSchedule{}).Unscoped().Where("id = ? AND tenant_id = ?", scheduleID, tenantID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
