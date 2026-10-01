package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Write side of telemetry dead letters: the claim/lease -> replay ->
// resolve/retry/dead state machine (see the diagram in telemetry_dead_letters.go).
//
// Every transition's column set is produced by a pure telemetryDeadLetter*Updates
// builder so the state machine can be reviewed and unit tested without a DB;
// the functions that apply them only add the compare-and-set WHERE clause.

// ---- entrypoints ----------------------------------------------------------

func (*TelemetryData) UpdateTelemetryDeadLetterStatus(id string, req *model.UpdateTelemetryDeadLetterStatusReq, claims *utils.UserClaims) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "dead letter id is required")
	}
	if err := requireTelemetryClaims(claims, telemetryWritePermissionMessage); err != nil {
		return err
	}

	row, err := getTelemetryDeadLetterForAccess(id)
	if err != nil {
		return err
	}
	if err := ensureTelemetryDeadLetterAccess(row, claims); err != nil {
		return err
	}

	now := time.Now().UTC()
	if req.Action == telemetryDeadLetterActionReplay {
		claimed, err := claimTelemetryDeadLetterForReplay(row.ID, now)
		if err != nil {
			return err
		}
		return replayTelemetryDeadLetterContext(context.Background(), claimed, now)
	}

	return updateTelemetryDeadLetterManualStatus(row.ID, row.Status, req.Action, now)
}

func (*TelemetryData) DrainTelemetryDeadLetters(req *model.DrainTelemetryDeadLetterReq, claims *utils.UserClaims) (*model.DrainTelemetryDeadLetterRsp, error) {
	if err := requireTelemetryClaims(claims, telemetryWritePermissionMessage); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	limit := normalizeTelemetryDeadLetterDrainLimit(req.Limit)
	return drainTelemetryDeadLetterQuery(telemetryDeadLetterReadyDrainQuery(req, claims, now), limit, now)
}

func (*TelemetryData) DrainReadyTelemetryDeadLettersForWorker(ctx context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
	ctx = telemetryDeadLetterContext(ctx)
	now := time.Now().UTC()
	query := telemetryDeadLetterReadyQuery(global.DB.WithContext(ctx).Model(&storage.TelemetryDeadLetter{}), now)
	return drainTelemetryDeadLetterQueryContext(ctx, query, normalizeTelemetryDeadLetterDrainLimit(limit), now)
}

// ---- drain: claim a batch, replay each, release leftovers on abort ---------

func drainTelemetryDeadLetterQuery(query *gorm.DB, limit int, now time.Time) (*model.DrainTelemetryDeadLetterRsp, error) {
	return drainTelemetryDeadLetterQueryContext(context.Background(), query, limit, now)
}

// drainTelemetryDeadLetterQueryContext claims up to limit rows from query and
// replays them. If claiming or draining aborts (DB error, ctx cancel), every
// claimed row still in processing is handed back to retrying immediately
// instead of waiting out the processing lease.
func drainTelemetryDeadLetterQueryContext(
	ctx context.Context,
	query *gorm.DB,
	limit int,
	now time.Time,
) (*model.DrainTelemetryDeadLetterRsp, error) {
	rows, totalReady, err := claimTelemetryDeadLetterRowsContext(ctx, query, limit, now)
	if err != nil {
		return nil, releaseTelemetryDeadLetterClaimsAfterErrorAt(rows, err, time.Now().UTC())
	}

	result, err := drainTelemetryDeadLetterRowsContext(ctx, rows, totalReady, now)
	if err != nil {
		return result, releaseTelemetryDeadLetterClaimsAfterErrorAt(rows, err, time.Now().UTC())
	}
	return result, nil
}

// drainTelemetryDeadLetterRowsContext replays already-claimed rows in order.
// A per-row replay failure is recorded in the result and draining continues;
// a ctx cancellation stops immediately and is returned as the error.
func drainTelemetryDeadLetterRowsContext(
	ctx context.Context,
	rows []storage.TelemetryDeadLetter,
	totalReady int64,
	now time.Time,
) (*model.DrainTelemetryDeadLetterRsp, error) {
	ctx = telemetryDeadLetterContext(ctx)
	result := &model.DrainTelemetryDeadLetterRsp{
		TotalReady: totalReady,
		Items:      make([]model.DrainTelemetryDeadLetterItemRsp, 0, len(rows)),
	}

	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.Attempted++
		item := model.DrainTelemetryDeadLetterItemRsp{
			ID:     row.ID,
			Status: storage.TelemetryDeadLetterStatusResolved,
		}
		if err := replayTelemetryDeadLetterContext(ctx, row, now); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			result.Failed++
			item.Status = telemetryDeadLetterDrainItemFailed
			item.Error = err.Error()
		} else {
			result.Replayed++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// ---- claim / lease ---------------------------------------------------------

func claimTelemetryDeadLetterRows(query *gorm.DB, limit int, now time.Time) ([]storage.TelemetryDeadLetter, int64, error) {
	return claimTelemetryDeadLetterRowsContext(context.Background(), query, limit, now)
}

// claimTelemetryDeadLetterRowsContext counts the ready set, picks the oldest
// limit candidates and claims each with a per-row compare-and-set against the
// claimable predicate, so concurrent drainers never claim the same row. Rows
// lost to a concurrent claimer are skipped. On error the rows claimed so far
// are returned so the caller can release them.
func claimTelemetryDeadLetterRowsContext(
	ctx context.Context,
	query *gorm.DB,
	limit int,
	now time.Time,
) ([]storage.TelemetryDeadLetter, int64, error) {
	ctx = telemetryDeadLetterContext(ctx)
	query = query.WithContext(ctx)
	var totalReady int64
	if err := query.Count(&totalReady).Error; err != nil {
		return nil, 0, telemetryDeadLetterDBError(err, nil)
	}

	var candidates []storage.TelemetryDeadLetter
	if err := query.Order("COALESCE(next_retry_at, created_at) ASC").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return nil, 0, telemetryDeadLetterDBError(err, nil)
	}

	claimed := make([]storage.TelemetryDeadLetter, 0, len(candidates))
	for _, row := range candidates {
		if err := ctx.Err(); err != nil {
			return claimed, totalReady, err
		}
		result := telemetryDeadLetterClaimableQuery(global.DB.WithContext(ctx).Model(&storage.TelemetryDeadLetter{}), now).
			Where("id = ?", row.ID).
			Updates(telemetryDeadLetterClaimUpdates(now))
		if result.Error != nil {
			return claimed, totalReady, telemetryDeadLetterDBError(result.Error, map[string]interface{}{"id": row.ID})
		}
		if result.RowsAffected == 0 {
			continue
		}
		row.Status = storage.TelemetryDeadLetterStatusProcessing
		row.NextRetryAt = nil
		row.UpdatedAt = now
		claimed = append(claimed, row)
	}
	return claimed, totalReady, nil
}

func claimTelemetryDeadLetterForReplay(id string, now time.Time) (storage.TelemetryDeadLetter, error) {
	rows, _, err := claimTelemetryDeadLetterRows(
		global.DB.Model(&storage.TelemetryDeadLetter{}).Where("id = ?", id),
		1,
		now,
	)
	if err != nil {
		return storage.TelemetryDeadLetter{}, err
	}
	if len(rows) == 0 {
		return storage.TelemetryDeadLetter{}, errcode.NewWithMessage(errcode.CodeParamError, "dead letter is not ready for replay")
	}
	return rows[0], nil
}

// releaseTelemetryDeadLetterClaimsAfterErrorAt releases rows and returns cause,
// joined with the release error if releasing also failed.
func releaseTelemetryDeadLetterClaimsAfterErrorAt(
	rows []storage.TelemetryDeadLetter,
	cause error,
	releasedAt time.Time,
) error {
	if releaseErr := releaseTelemetryDeadLetterClaims(rows, releasedAt); releaseErr != nil {
		return errors.Join(cause, releaseErr)
	}
	return cause
}

// releaseTelemetryDeadLetterClaims returns rows that are still processing to
// retrying. Rows already resolved/retried by replay are untouched (CAS on
// status). It runs on a fresh bounded context because the caller's ctx is
// typically the one that was just canceled.
func releaseTelemetryDeadLetterClaims(rows []storage.TelemetryDeadLetter, releasedAt time.Time) error {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := strings.TrimSpace(row.ID); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	releaseCtx, cancel := context.WithTimeout(context.Background(), telemetryDeadLetterClaimReleaseTimeout)
	defer cancel()
	if err := global.DB.WithContext(releaseCtx).
		Model(&storage.TelemetryDeadLetter{}).
		Where("id IN ? AND status = ?", ids, storage.TelemetryDeadLetterStatusProcessing).
		Updates(telemetryDeadLetterReleaseUpdates(releasedAt)).Error; err != nil {
		return telemetryDeadLetterDBError(err, map[string]interface{}{"ids": ids})
	}
	return nil
}

// ---- replay ----------------------------------------------------------------

// replayTelemetryDeadLetterContext writes a claimed row back into history and
// current telemetry and resolves it in one transaction. History inserts are
// idempotent (ON CONFLICT DO NOTHING) and the current upsert never regresses a
// newer value. Any failure other than ctx cancellation is recorded on the row
// (retrying with backoff, or dead once attempts are exhausted).
func replayTelemetryDeadLetterContext(ctx context.Context, row storage.TelemetryDeadLetter, now time.Time) error {
	ctx = telemetryDeadLetterContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	history, err := storage.TelemetryDataFromDeadLetter(row)
	if err != nil {
		if markErr := markTelemetryDeadLetterReplayFailureContext(ctx, row, err, now); markErr != nil {
			return markErr
		}
		return errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}

	current := telemetryCurrentFromHistory(history)
	replayErr := global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}, {Name: "key"}, {Name: "ts"}},
			DoNothing: true,
		}).Create(&history).Error; err != nil {
			return err
		}
		if err := tx.Clauses(storage.TelemetryCurrentUpsertClause()).Create(&current).Error; err != nil {
			return err
		}
		return updateTelemetryDeadLetterFieldsTx(tx, row.ID, telemetryDeadLetterResolvedUpdates(now))
	})
	if replayErr == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if markErr := markTelemetryDeadLetterReplayFailureContext(ctx, row, replayErr, now); markErr != nil {
		return markErr
	}
	return telemetryDeadLetterDBError(replayErr, map[string]interface{}{"id": row.ID})
}

func telemetryCurrentFromHistory(history storage.TelemetryData) storage.TelemetryCurrentData {
	return storage.TelemetryCurrentData{
		DeviceID: history.DeviceID,
		Key:      history.Key,
		TS:       time.UnixMilli(history.TS),
		BoolV:    history.BoolV,
		NumberV:  history.NumberV,
		StringV:  history.StringV,
		TenantID: history.TenantID,
	}
}

func markTelemetryDeadLetterReplayFailureContext(
	ctx context.Context,
	row storage.TelemetryDeadLetter,
	replayErr error,
	now time.Time,
) error {
	return updateTelemetryDeadLetterFieldsTx(
		global.DB.WithContext(telemetryDeadLetterContext(ctx)),
		row.ID,
		telemetryDeadLetterReplayFailureUpdates(row.Attempts, replayErr, now),
	)
}

func updateTelemetryDeadLetterFieldsTx(tx *gorm.DB, id string, updates map[string]interface{}) error {
	if err := tx.Model(&storage.TelemetryDeadLetter{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return telemetryDeadLetterDBError(err, map[string]interface{}{"id": id})
	}
	return nil
}

// ---- manual operator transitions -------------------------------------------

// updateTelemetryDeadLetterManualStatus applies retry/resolve/ignore with a
// compare-and-set on the status the operator saw, so it can never clobber a
// row a drainer claimed (processing) or changed in the meantime.
func updateTelemetryDeadLetterManualStatus(id string, previousStatus string, action string, now time.Time) error {
	if !telemetryDeadLetterManualStatusMutable(previousStatus) {
		return telemetryDeadLetterStatusConflictError(previousStatus)
	}

	updates, err := telemetryDeadLetterStatusUpdates(action, now)
	if err != nil {
		return err
	}
	result := global.DB.
		Model(&storage.TelemetryDeadLetter{}).
		Where("id = ? AND status = ?", id, previousStatus).
		Updates(updates)
	if result.Error != nil {
		return telemetryDeadLetterDBError(result.Error, map[string]interface{}{"id": id})
	}
	if result.RowsAffected != 1 {
		return telemetryDeadLetterStatusConflictError(previousStatus)
	}
	return nil
}

func telemetryDeadLetterManualStatusMutable(status string) bool {
	switch status {
	case storage.TelemetryDeadLetterStatusPending,
		storage.TelemetryDeadLetterStatusRetrying,
		storage.TelemetryDeadLetterStatusResolved,
		storage.TelemetryDeadLetterStatusDead:
		return true
	default:
		return false
	}
}

func telemetryDeadLetterStatusConflictError(previousStatus string) error {
	return errcode.NewWithMessage(
		errcode.CodeOpDenied,
		fmt.Sprintf("%s (expected status %q)", telemetryDeadLetterStatusConflict, previousStatus),
	)
}

// ---- transition column sets (pure) -----------------------------------------

// telemetryDeadLetterStatusUpdates maps a manual operator action to its column set.
func telemetryDeadLetterStatusUpdates(action string, now time.Time) (map[string]interface{}, error) {
	switch action {
	case telemetryDeadLetterActionRetry:
		return map[string]interface{}{
			"status":        storage.TelemetryDeadLetterStatusPending,
			"attempts":      0,
			"next_retry_at": nil,
			"updated_at":    now,
		}, nil
	case telemetryDeadLetterActionResolve:
		return telemetryDeadLetterResolvedUpdates(now), nil
	case telemetryDeadLetterActionIgnore:
		return telemetryDeadLetterStatusOnlyUpdates(storage.TelemetryDeadLetterStatusDead, now), nil
	case telemetryDeadLetterActionReplay:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "replay action must execute the dead letter")
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported dead letter action")
	}
}

// telemetryDeadLetterClaimUpdates takes the processing lease; updated_at is
// the lease start that the stale-processing predicate measures from.
func telemetryDeadLetterClaimUpdates(now time.Time) map[string]interface{} {
	return telemetryDeadLetterStatusOnlyUpdates(storage.TelemetryDeadLetterStatusProcessing, now)
}

// telemetryDeadLetterReleaseUpdates hands a lease back as immediately retryable.
func telemetryDeadLetterReleaseUpdates(releasedAt time.Time) map[string]interface{} {
	return map[string]interface{}{
		"status":        storage.TelemetryDeadLetterStatusRetrying,
		"next_retry_at": releasedAt,
		"updated_at":    releasedAt,
	}
}

func telemetryDeadLetterResolvedUpdates(now time.Time) map[string]interface{} {
	return telemetryDeadLetterStatusOnlyUpdates(storage.TelemetryDeadLetterStatusResolved, now)
}

// telemetryDeadLetterReplayFailureUpdates records a failed replay given the
// attempts count the row had when it was claimed: retrying with backoff, or
// dead once the incremented count reaches the max. attempts is incremented in
// SQL so concurrent failures can't lose an increment.
func telemetryDeadLetterReplayFailureUpdates(previousAttempts int, replayErr error, now time.Time) map[string]interface{} {
	attempts := previousAttempts + 1
	status := storage.TelemetryDeadLetterStatusRetrying
	var nextRetryAt *time.Time
	if attempts >= storage.TelemetryDeadLetterMaxAttempts() {
		status = storage.TelemetryDeadLetterStatusDead
	} else {
		nextRetryAt = storage.NextTelemetryDeadLetterRetryAt(attempts, now)
	}

	lastError := ""
	if replayErr != nil {
		lastError = replayErr.Error()
	}
	return map[string]interface{}{
		"status":        status,
		"attempts":      gorm.Expr("attempts + ?", 1),
		"last_error":    fmt.Sprintf("replay failed: %s", lastError),
		"next_retry_at": nextRetryAt,
		"updated_at":    now,
	}
}

// telemetryDeadLetterStatusOnlyUpdates sets status and clears any scheduled
// retry (used for claim, resolve and ignore).
func telemetryDeadLetterStatusOnlyUpdates(status string, now time.Time) map[string]interface{} {
	return map[string]interface{}{
		"status":        status,
		"next_retry_at": nil,
		"updated_at":    now,
	}
}

func telemetryDeadLetterContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
