package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	telemetryDeadLetterActionRetry   = "retry"
	telemetryDeadLetterActionResolve = "resolve"
	telemetryDeadLetterActionIgnore  = "ignore"
	telemetryDeadLetterActionReplay  = "replay"

	telemetryDeadLetterProcessingTimeout   = 5 * time.Minute
	telemetryDeadLetterClaimReleaseTimeout = 2 * time.Second
	telemetryDeadLetterStatusConflict      = "dead letter status conflict; refresh and retry"
)

func (*TelemetryData) GetTelemetryDeadLetterList(req *model.GetTelemetryDeadLetterListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireTelemetryClaims(claims, telemetryReadPermissionMessage); err != nil {
		return nil, err
	}

	query := telemetryDeadLetterScopedQuery(req, claims)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	page, pageSize := normalizeTelemetryDeadLetterPage(req.Page, req.PageSize)
	var rows []storage.TelemetryDeadLetter
	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	return map[string]interface{}{
		"total": total,
		"list":  buildTelemetryDeadLetterList(rows),
	}, nil
}

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
		return replayTelemetryDeadLetter(claimed, now)
	}

	return updateTelemetryDeadLetterManualStatus(row.ID, row.Status, req.Action, now)
}

func (*TelemetryData) DrainTelemetryDeadLetters(req *model.DrainTelemetryDeadLetterReq, claims *utils.UserClaims) (*model.DrainTelemetryDeadLetterRsp, error) {
	if err := requireTelemetryClaims(claims, telemetryWritePermissionMessage); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	limit := normalizeTelemetryDeadLetterDrainLimit(req.Limit)
	query := telemetryDeadLetterReadyDrainQuery(req, claims, now)

	return drainTelemetryDeadLetterQuery(query, limit, now)
}

func (*TelemetryData) DrainReadyTelemetryDeadLettersForWorker(ctx context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	query := telemetryDeadLetterReadyQuery(global.DB.WithContext(ctx).Model(&storage.TelemetryDeadLetter{}), now)
	return drainTelemetryDeadLetterQueryContext(ctx, query, normalizeTelemetryDeadLetterDrainLimit(limit), now)
}

func drainTelemetryDeadLetterQuery(query *gorm.DB, limit int, now time.Time) (*model.DrainTelemetryDeadLetterRsp, error) {
	return drainTelemetryDeadLetterQueryContext(context.Background(), query, limit, now)
}

func drainTelemetryDeadLetterQueryContext(
	ctx context.Context,
	query *gorm.DB,
	limit int,
	now time.Time,
) (*model.DrainTelemetryDeadLetterRsp, error) {
	rows, totalReady, err := claimTelemetryDeadLetterRowsContext(ctx, query, limit, now)
	if err != nil {
		return nil, releaseTelemetryDeadLetterClaimsAfterError(rows, err)
	}

	result, err := drainTelemetryDeadLetterRowsContext(ctx, rows, totalReady, now)
	if err != nil {
		return result, releaseTelemetryDeadLetterClaimsAfterError(rows, err)
	}
	return result, nil
}

func claimTelemetryDeadLetterRows(query *gorm.DB, limit int, now time.Time) ([]storage.TelemetryDeadLetter, int64, error) {
	return claimTelemetryDeadLetterRowsContext(context.Background(), query, limit, now)
}

func claimTelemetryDeadLetterRowsContext(
	ctx context.Context,
	query *gorm.DB,
	limit int,
	now time.Time,
) ([]storage.TelemetryDeadLetter, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	query = query.WithContext(ctx)
	var totalReady int64
	if err := query.Count(&totalReady).Error; err != nil {
		return nil, 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	var candidates []storage.TelemetryDeadLetter
	if err := query.Order("COALESCE(next_retry_at, created_at) ASC").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return nil, 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}

	claimed := make([]storage.TelemetryDeadLetter, 0, len(candidates))
	for _, row := range candidates {
		if err := ctx.Err(); err != nil {
			return claimed, totalReady, err
		}
		result := telemetryDeadLetterClaimableQuery(global.DB.WithContext(ctx).Model(&storage.TelemetryDeadLetter{}), now).
			Where("id = ?", row.ID).
			Updates(map[string]interface{}{
				"status":        storage.TelemetryDeadLetterStatusProcessing,
				"next_retry_at": nil,
				"updated_at":    now,
			})
		if result.Error != nil {
			return claimed, totalReady, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error": result.Error.Error(),
				"id":    row.ID,
			})
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

func releaseTelemetryDeadLetterClaimsAfterError(rows []storage.TelemetryDeadLetter, cause error) error {
	return releaseTelemetryDeadLetterClaimsAfterErrorAt(rows, cause, time.Now().UTC())
}

func releaseTelemetryDeadLetterClaimsAfterErrorAt(
	rows []storage.TelemetryDeadLetter,
	cause error,
	releasedAt time.Time,
) error {
	releaseErr := releaseTelemetryDeadLetterClaims(rows, releasedAt)
	if releaseErr == nil {
		return cause
	}
	return errors.Join(cause, releaseErr)
}

func releaseTelemetryDeadLetterClaims(rows []storage.TelemetryDeadLetter, releasedAt time.Time) error {
	if len(rows) == 0 {
		return nil
	}
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
		Updates(map[string]interface{}{
			"status":        storage.TelemetryDeadLetterStatusRetrying,
			"next_retry_at": releasedAt,
			"updated_at":    releasedAt,
		}).Error; err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"ids":   ids,
		})
	}
	return nil
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

func normalizeTelemetryDeadLetterDrainLimit(limit int) int {
	if limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func telemetryDeadLetterReadyDrainQuery(req *model.DrainTelemetryDeadLetterReq, claims *utils.UserClaims, now time.Time) *gorm.DB {
	listReq := &model.GetTelemetryDeadLetterListReq{
		TenantID: req.TenantID,
		DeviceID: req.DeviceID,
		Key:      req.Key,
	}
	return telemetryDeadLetterReadyQuery(telemetryDeadLetterScopedQuery(listReq, claims), now)
}

func telemetryDeadLetterReadyQuery(query *gorm.DB, now time.Time) *gorm.DB {
	return telemetryDeadLetterClaimableQuery(query, now)
}

func telemetryDeadLetterClaimableQuery(query *gorm.DB, now time.Time) *gorm.DB {
	staleProcessingBefore := now.Add(-telemetryDeadLetterProcessingTimeout)
	return query.
		Where(
			"(status IN ? OR (status = ? AND updated_at <= ?))",
			[]string{storage.TelemetryDeadLetterStatusPending, storage.TelemetryDeadLetterStatusRetrying},
			storage.TelemetryDeadLetterStatusProcessing,
			staleProcessingBefore,
		).
		Where("attempts < ?", storage.TelemetryDeadLetterMaxAttempts()).
		Where("(next_retry_at IS NULL OR next_retry_at <= ?)", now)
}

func drainTelemetryDeadLetterRows(rows []storage.TelemetryDeadLetter, totalReady int64, now time.Time) *model.DrainTelemetryDeadLetterRsp {
	result, _ := drainTelemetryDeadLetterRowsContext(context.Background(), rows, totalReady, now)
	return result
}

func drainTelemetryDeadLetterRowsContext(
	ctx context.Context,
	rows []storage.TelemetryDeadLetter,
	totalReady int64,
	now time.Time,
) (*model.DrainTelemetryDeadLetterRsp, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
			item.Status = "failed"
			item.Error = err.Error()
		} else {
			result.Replayed++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func replayTelemetryDeadLetter(row storage.TelemetryDeadLetter, now time.Time) error {
	return replayTelemetryDeadLetterContext(context.Background(), row, now)
}

func replayTelemetryDeadLetterContext(ctx context.Context, row storage.TelemetryDeadLetter, now time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	history, err := storage.TelemetryDataFromDeadLetter(row)
	if err != nil {
		markErr := markTelemetryDeadLetterReplayFailureContext(ctx, row, err, now)
		if markErr != nil {
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

		return markTelemetryDeadLetterResolvedTx(tx, row.ID, now)
	})
	if replayErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		markErr := markTelemetryDeadLetterReplayFailureContext(ctx, row, replayErr, now)
		if markErr != nil {
			return markErr
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": replayErr.Error(),
			"id":    row.ID,
		})
	}
	return nil
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

func markTelemetryDeadLetterResolved(id string, now time.Time) error {
	return updateTelemetryDeadLetterFields(id, map[string]interface{}{
		"status":        storage.TelemetryDeadLetterStatusResolved,
		"next_retry_at": nil,
		"updated_at":    now,
	})
}

func markTelemetryDeadLetterResolvedTx(tx *gorm.DB, id string, now time.Time) error {
	return updateTelemetryDeadLetterFieldsTx(tx, id, map[string]interface{}{
		"status":        storage.TelemetryDeadLetterStatusResolved,
		"next_retry_at": nil,
		"updated_at":    now,
	})
}

func markTelemetryDeadLetterReplayFailure(row storage.TelemetryDeadLetter, replayErr error, now time.Time) error {
	return markTelemetryDeadLetterReplayFailureContext(context.Background(), row, replayErr, now)
}

func markTelemetryDeadLetterReplayFailureContext(
	ctx context.Context,
	row storage.TelemetryDeadLetter,
	replayErr error,
	now time.Time,
) error {
	attempts := row.Attempts + 1
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
	return updateTelemetryDeadLetterFieldsContext(ctx, row.ID, map[string]interface{}{
		"status":        status,
		"attempts":      gorm.Expr("attempts + ?", 1),
		"last_error":    fmt.Sprintf("replay failed: %s", lastError),
		"next_retry_at": nextRetryAt,
		"updated_at":    now,
	})
}

func updateTelemetryDeadLetterFields(id string, updates map[string]interface{}) error {
	return updateTelemetryDeadLetterFieldsContext(context.Background(), id, updates)
}

func updateTelemetryDeadLetterFieldsContext(ctx context.Context, id string, updates map[string]interface{}) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return updateTelemetryDeadLetterFieldsTx(global.DB.WithContext(ctx), id, updates)
}

func updateTelemetryDeadLetterFieldsTx(tx *gorm.DB, id string, updates map[string]interface{}) error {
	if err := tx.Model(&storage.TelemetryDeadLetter{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"id":    id,
		})
	}
	return nil
}

func telemetryDeadLetterScopedQuery(req *model.GetTelemetryDeadLetterListReq, claims *utils.UserClaims) *gorm.DB {
	query := global.DB.Model(&storage.TelemetryDeadLetter{})
	if claims.Authority != constant.SYS_ADMIN {
		query = query.Where("tenant_id = ?", claims.TenantID)
	} else if req.TenantID != "" {
		query = query.Where("tenant_id = ?", strings.TrimSpace(req.TenantID))
	}
	if claims.Authority == constant.TENANT_USER {
		query = query.Where(
			"EXISTS (SELECT 1 FROM devices d WHERE d.id = telemetry_dead_letters.device_id AND d.tenant_id = telemetry_dead_letters.tenant_id AND d.owner_user_id = ?)",
			strings.TrimSpace(claims.ID),
		)
	}
	if deviceID := strings.TrimSpace(req.DeviceID); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	if key := strings.TrimSpace(req.Key); key != "" {
		query = query.Where(`"key" = ?`, key)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	return query
}

func normalizeTelemetryDeadLetterPage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return page, pageSize
}

func buildTelemetryDeadLetterList(rows []storage.TelemetryDeadLetter) []model.TelemetryDeadLetterRsp {
	list := make([]model.TelemetryDeadLetterRsp, 0, len(rows))
	for _, row := range rows {
		list = append(list, buildTelemetryDeadLetterRsp(row))
	}
	return list
}

func buildTelemetryDeadLetterRsp(row storage.TelemetryDeadLetter) model.TelemetryDeadLetterRsp {
	return model.TelemetryDeadLetterRsp{
		ID:          row.ID,
		DeviceID:    row.DeviceID,
		TenantID:    row.TenantID,
		Key:         row.Key,
		TS:          row.TS,
		BoolV:       row.BoolV,
		NumberV:     row.NumberV,
		StringV:     row.StringV,
		Status:      row.Status,
		Attempts:    row.Attempts,
		LastError:   row.LastError,
		NextRetryAt: formatOptionalTelemetryDeadLetterTime(row.NextRetryAt),
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   row.UpdatedAt.Format(time.RFC3339),
	}
}

func formatOptionalTelemetryDeadLetterTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func getTelemetryDeadLetterForAccess(id string) (storage.TelemetryDeadLetter, error) {
	var row storage.TelemetryDeadLetter
	err := global.DB.Where("id = ?", id).First(&row).Error
	if err != nil {
		return row, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"id":    id,
		})
	}
	return row, nil
}

func ensureTelemetryDeadLetterAccess(row storage.TelemetryDeadLetter, claims *utils.UserClaims) error {
	if claims.Authority == constant.SYS_ADMIN {
		return nil
	}
	if row.TenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, telemetryReadPermissionMessage)
	}
	if claims.Authority == constant.TENANT_USER {
		if _, err := ensureTelemetryDeviceWriteAccess(row.DeviceID, claims); err != nil {
			return err
		}
	}
	return nil
}

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
		return map[string]interface{}{
			"status":        storage.TelemetryDeadLetterStatusResolved,
			"next_retry_at": nil,
			"updated_at":    now,
		}, nil
	case telemetryDeadLetterActionIgnore:
		return map[string]interface{}{
			"status":        storage.TelemetryDeadLetterStatusDead,
			"next_retry_at": nil,
			"updated_at":    now,
		}, nil
	case telemetryDeadLetterActionReplay:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "replay action must execute the dead letter")
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "unsupported dead letter action")
	}
}

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
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": result.Error.Error(),
			"id":    id,
		})
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
