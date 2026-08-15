package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

// AttributeEventDeadLetterFilter is the storage-side scope for operator
// queries. OwnerUserID is intentionally optional: the service layer decides
// whether a caller must be owner-scoped, while storage keeps the predicate in
// the same query as the tenant/device filters.
type AttributeEventDeadLetterFilter struct {
	ID          string
	TenantID    string
	DeviceID    string
	DataType    DataType
	Status      string
	OwnerUserID string
	Page        int
	PageSize    int
}

// AttributeEventDeadLetterMetadata deliberately omits the canonical payload.
// Operators can inspect identity, status and failure context without turning
// the management endpoint into a raw attribute/event data exfiltration path.
type AttributeEventDeadLetterMetadata struct {
	ID          string
	DataType    DataType
	DeviceID    string
	TenantID    string
	TS          int64
	Status      string
	Attempts    int
	LastError   string
	NextRetryAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AttributeEventDeadLetterList struct {
	Total int64
	Items []AttributeEventDeadLetterMetadata
}

type AttributeEventDeadLetterAction string

const (
	AttributeEventDeadLetterActionRetry   AttributeEventDeadLetterAction = "retry"
	AttributeEventDeadLetterActionReplay  AttributeEventDeadLetterAction = "replay"
	AttributeEventDeadLetterActionResolve AttributeEventDeadLetterAction = "resolve"
	AttributeEventDeadLetterActionIgnore  AttributeEventDeadLetterAction = "ignore"

	attributeEventDeadLetterMaxAttempts = 3
)

var (
	ErrAttributeEventDeadLetterStatusConflict = errors.New("attribute/event dead-letter status conflict")
	ErrAttributeEventDeadLetterReplayNotReady = errors.New("attribute/event dead-letter is not ready for replay")
)

type AttributeEventDeadLetterDrainItem struct {
	ID     string
	Status string
	Error  string
}

type AttributeEventDeadLetterDrainResult struct {
	TotalReady int64
	Attempted  int
	Replayed   int
	Failed     int
	Items      []AttributeEventDeadLetterDrainItem
}

// AttributeEventDeadLetterOperator is the small external seam for manual
// operations. The concrete storage implementation still owns claim tokens,
// leases, canonical envelope validation and replay side-effect suppression.
type AttributeEventDeadLetterOperator interface {
	ListAttributeEventDeadLetters(context.Context, AttributeEventDeadLetterFilter) (AttributeEventDeadLetterList, error)
	UpdateAttributeEventDeadLetter(context.Context, string, AttributeEventDeadLetterAction, AttributeEventDeadLetterFilter) error
	DrainAttributeEventDeadLetters(context.Context, AttributeEventDeadLetterFilter, int) (AttributeEventDeadLetterDrainResult, error)
}

var _ AttributeEventDeadLetterOperator = (*storage)(nil)
var _ AttributeEventDeadLetterOperator = (*attributeEventIngress)(nil)

// NewAttributeEventDeadLetterOperator creates a DB-backed operator for
// maintenance tools and tests. The running storage service also implements
// the same interface, so application code should prefer the live instance.
func NewAttributeEventDeadLetterOperator(db *gorm.DB, config Config) AttributeEventDeadLetterOperator {
	return newAttributeEventIngress(db, nil, config, newMetricsCollector(false))
}

func (s *storage) ListAttributeEventDeadLetters(
	ctx context.Context,
	filter AttributeEventDeadLetterFilter,
) (AttributeEventDeadLetterList, error) {
	if s == nil || s.attributeEvent == nil {
		return AttributeEventDeadLetterList{}, fmt.Errorf("attribute/event dead-letter operator is unavailable")
	}
	return s.attributeEvent.ListAttributeEventDeadLetters(ctx, filter)
}

func (s *storage) UpdateAttributeEventDeadLetter(
	ctx context.Context,
	id string,
	action AttributeEventDeadLetterAction,
	filter AttributeEventDeadLetterFilter,
) error {
	if s == nil || s.attributeEvent == nil {
		return fmt.Errorf("attribute/event dead-letter operator is unavailable")
	}
	return s.attributeEvent.UpdateAttributeEventDeadLetter(ctx, id, action, filter)
}

func (s *storage) DrainAttributeEventDeadLetters(
	ctx context.Context,
	filter AttributeEventDeadLetterFilter,
	limit int,
) (AttributeEventDeadLetterDrainResult, error) {
	if s == nil || s.attributeEvent == nil {
		return AttributeEventDeadLetterDrainResult{}, fmt.Errorf("attribute/event dead-letter operator is unavailable")
	}
	return s.attributeEvent.DrainAttributeEventDeadLetters(ctx, filter, limit)
}

func (i *attributeEventIngress) ListAttributeEventDeadLetters(
	ctx context.Context,
	filter AttributeEventDeadLetterFilter,
) (AttributeEventDeadLetterList, error) {
	filter, err := normalizeAttributeEventDeadLetterFilter(filter)
	if err != nil {
		return AttributeEventDeadLetterList{}, err
	}
	query, err := i.attributeEventDeadLetterScopeQuery(ctx, filter)
	if err != nil {
		return AttributeEventDeadLetterList{}, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AttributeEventDeadLetterList{}, err
	}

	// Select only metadata columns. This is a deliberate second guard against
	// accidentally serialising the canonical raw payload in a future handler.
	var rows []uplinkStorageDeadLetter
	if err := query.Select(
		"id, data_type, device_id, tenant_id, ts, status, attempts, last_error, next_retry_at, created_at, updated_at",
	).Order("created_at DESC, id DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&rows).Error; err != nil {
		return AttributeEventDeadLetterList{}, err
	}

	items := make([]AttributeEventDeadLetterMetadata, 0, len(rows))
	for _, row := range rows {
		items = append(items, attributeEventDeadLetterMetadata(row))
	}
	return AttributeEventDeadLetterList{Total: total, Items: items}, nil
}

func (i *attributeEventIngress) UpdateAttributeEventDeadLetter(
	ctx context.Context,
	id string,
	action AttributeEventDeadLetterAction,
	filter AttributeEventDeadLetterFilter,
) error {
	if i == nil || i.db == nil {
		return fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("attribute/event dead-letter id is required")
	}
	filter, err := normalizeAttributeEventDeadLetterFilter(filter)
	if err != nil {
		return err
	}
	filter.ID = id
	if action == AttributeEventDeadLetterActionReplay {
		replayCtx, cancel := i.newReplayContext(ctx)
		defer cancel()
		ctx = replayCtx
	}

	now, err := i.deadLetterDatabaseNow(ctx)
	if err != nil {
		return fmt.Errorf("read attribute/event dead-letter database clock: %w", err)
	}
	if action == AttributeEventDeadLetterActionReplay {
		row, claimed, err := i.claimDeadLetterWithFilter(ctx, id, now, filter)
		if err != nil {
			return err
		}
		if !claimed {
			return ErrAttributeEventDeadLetterReplayNotReady
		}
		return i.replayClaimedAttributeEventDeadLetter(ctx, row, now)
	}

	var updates map[string]interface{}
	switch action {
	case AttributeEventDeadLetterActionRetry:
		updates = map[string]interface{}{
			"status":        uplinkDeadLetterStatusPending,
			"attempts":      0,
			"last_error":    "MANUAL_RETRY",
			"next_retry_at": nil,
			"claim_token":   nil,
			"lease_until":   nil,
			"updated_at":    now,
		}
	case AttributeEventDeadLetterActionResolve:
		updates = map[string]interface{}{
			"status":        uplinkDeadLetterStatusResolved,
			"next_retry_at": nil,
			"claim_token":   nil,
			"lease_until":   nil,
			"updated_at":    now,
		}
	case AttributeEventDeadLetterActionIgnore:
		updates = map[string]interface{}{
			"status":        uplinkDeadLetterStatusDead,
			"next_retry_at": nil,
			"claim_token":   nil,
			"lease_until":   nil,
			"updated_at":    now,
		}
	default:
		return fmt.Errorf("unsupported attribute/event dead-letter action %q", action)
	}

	scopeQuery, err := i.attributeEventDeadLetterScopeQuery(ctx, filter)
	if err != nil {
		return err
	}
	result := i.db.WithContext(ctx).Model(&uplinkStorageDeadLetter{}).
		Where(
			"id = ? AND status IN ? AND id IN (?)",
			id,
			[]string{
				uplinkDeadLetterStatusPending,
				uplinkDeadLetterStatusRetrying,
				uplinkDeadLetterStatusResolved,
				uplinkDeadLetterStatusDead,
			},
			scopeQuery.Select("id"),
		).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAttributeEventDeadLetterStatusConflict
	}
	return nil
}

func (i *attributeEventIngress) DrainAttributeEventDeadLetters(
	ctx context.Context,
	filter AttributeEventDeadLetterFilter,
	limit int,
) (AttributeEventDeadLetterDrainResult, error) {
	if i == nil || i.db == nil {
		return AttributeEventDeadLetterDrainResult{}, fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	passCtx, passCancel := i.newReplayContext(ctx)
	defer passCancel()
	ctx = passCtx
	filter, err := normalizeAttributeEventDeadLetterFilter(filter)
	if err != nil {
		return AttributeEventDeadLetterDrainResult{}, err
	}
	limit = normalizeAttributeEventDeadLetterDrainLimit(limit)

	now, err := i.deadLetterDatabaseNow(ctx)
	if err != nil {
		return AttributeEventDeadLetterDrainResult{}, fmt.Errorf("read attribute/event dead-letter database clock: %w", err)
	}
	reapFilter := filter
	// Status selects replay candidates. Exhausted claims are always processing,
	// so retaining that selector would make status=pending drains skip reaping.
	reapFilter.Status = ""
	if err := i.reapExhaustedDeadLetterClaimsWithFilter(ctx, now, reapFilter); err != nil {
		return AttributeEventDeadLetterDrainResult{}, fmt.Errorf("reap exhausted attribute/event dead-letter claims: %w", err)
	}
	baseQuery, err := i.attributeEventDeadLetterScopeQuery(ctx, filter)
	if err != nil {
		return AttributeEventDeadLetterDrainResult{}, err
	}
	readyQuery := i.claimableDeadLetterQuery(ctx, now)
	readyQuery = readyQuery.Where("id IN (?)", baseQuery.Select("id"))

	var totalReady int64
	if err := readyQuery.Count(&totalReady).Error; err != nil {
		return AttributeEventDeadLetterDrainResult{}, err
	}

	var candidates []uplinkStorageDeadLetter
	if err := readyQuery.Order(
		"CASE WHEN status = 'processing' THEN COALESCE(lease_until, updated_at) ELSE COALESCE(next_retry_at, created_at) END ASC, created_at ASC, id ASC",
	).Limit(limit).Find(&candidates).Error; err != nil {
		return AttributeEventDeadLetterDrainResult{}, err
	}

	result := AttributeEventDeadLetterDrainResult{
		TotalReady: totalReady,
		Items:      make([]AttributeEventDeadLetterDrainItem, 0, len(candidates)),
	}
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		row, claimed, claimErr := i.claimDeadLetterWithFilter(ctx, candidate.ID, now, filter)
		if claimErr != nil {
			result.Failed++
			result.Items = append(result.Items, AttributeEventDeadLetterDrainItem{
				ID: candidate.ID, Status: "failed", Error: claimErr.Error(),
			})
			continue
		}
		if !claimed {
			continue
		}

		result.Attempted++
		item := AttributeEventDeadLetterDrainItem{ID: row.ID, Status: uplinkDeadLetterStatusResolved}
		if replayErr := i.replayClaimedAttributeEventDeadLetter(ctx, row, now); replayErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			result.Failed++
			item.Status = "failed"
			item.Error = replayErr.Error()
		} else {
			result.Replayed++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (i *attributeEventIngress) replayClaimedAttributeEventDeadLetter(
	ctx context.Context,
	row uplinkStorageDeadLetter,
	now time.Time,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		releaseCtx, cancel := context.WithTimeout(context.Background(), attributeEventDeadLetterSettlementTimeout)
		defer cancel()
		_ = i.deferDeadLetterRetry(releaseCtx, row, now)
		return err
	}

	envelope, err := unmarshalAttributeEventEnvelope(row.Payload)
	if err == nil {
		err = verifyDeadLetterEnvelope(row, envelope)
	}
	if err != nil {
		if markErr := i.markDeadLetterDead(ctx, row, now); markErr != nil {
			return errors.Join(err, markErr)
		}
		return err
	}

	if err := i.persistEnvelope(ctx, envelope, false); err != nil {
		if retryErr := i.deferDeadLetterRetry(ctx, row, now); retryErr != nil {
			return errors.Join(err, retryErr)
		}
		return err
	}

	if err := i.settleClaimedDeadLetter(ctx, row, map[string]interface{}{
		"status":        uplinkDeadLetterStatusResolved,
		"last_error":    "",
		"next_retry_at": nil,
		"claim_token":   nil,
		"lease_until":   nil,
		"updated_at":    now,
	}); err != nil {
		return err
	}
	if i.metrics != nil {
		i.metrics.incAttributeEventDeadLetterReplayed()
	}
	return nil
}

func (i *attributeEventIngress) attributeEventDeadLetterScopeQuery(
	ctx context.Context,
	filter AttributeEventDeadLetterFilter,
) (*gorm.DB, error) {
	if i == nil || i.db == nil {
		return nil, fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	query := i.db.WithContext(ctx).Model(&uplinkStorageDeadLetter{})
	if filter.ID != "" {
		query = query.Where("id = ?", filter.ID)
	}
	if filter.TenantID != "" {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.DeviceID != "" {
		query = query.Where("device_id = ?", filter.DeviceID)
	}
	if filter.DataType != "" {
		query = query.Where("data_type = ?", filter.DataType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.OwnerUserID != "" {
		query = query.Where(
			"EXISTS (SELECT 1 FROM devices d WHERE d.id = uplink_storage_dead_letters.device_id AND d.tenant_id = uplink_storage_dead_letters.tenant_id AND d.owner_user_id = ?)",
			filter.OwnerUserID,
		)
	}
	return query, nil
}

func normalizeAttributeEventDeadLetterFilter(filter AttributeEventDeadLetterFilter) (AttributeEventDeadLetterFilter, error) {
	filter.ID = strings.TrimSpace(filter.ID)
	filter.TenantID = strings.TrimSpace(filter.TenantID)
	filter.DeviceID = strings.TrimSpace(filter.DeviceID)
	filter.DataType = DataType(strings.TrimSpace(string(filter.DataType)))
	filter.Status = strings.TrimSpace(filter.Status)
	filter.OwnerUserID = strings.TrimSpace(filter.OwnerUserID)
	if filter.DataType != "" && filter.DataType != DataTypeAttribute && filter.DataType != DataTypeEvent {
		return AttributeEventDeadLetterFilter{}, fmt.Errorf("unsupported attribute/event dead-letter data type %q", filter.DataType)
	}
	if filter.Status != "" {
		switch filter.Status {
		case uplinkDeadLetterStatusPending,
			uplinkDeadLetterStatusRetrying,
			uplinkDeadLetterStatusProcessing,
			uplinkDeadLetterStatusResolved,
			uplinkDeadLetterStatusDead:
		default:
			return AttributeEventDeadLetterFilter{}, fmt.Errorf("unsupported attribute/event dead-letter status %q", filter.Status)
		}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 1000 {
		filter.PageSize = 1000
	}
	return filter, nil
}

func (i *attributeEventIngress) claimDeadLetterWithFilter(
	ctx context.Context,
	id string,
	now time.Time,
	filter AttributeEventDeadLetterFilter,
) (uplinkStorageDeadLetter, bool, error) {
	filter.ID = strings.TrimSpace(id)
	scopeQuery, err := i.attributeEventDeadLetterScopeQuery(ctx, filter)
	if err != nil {
		return uplinkStorageDeadLetter{}, false, err
	}
	token := strings.ToLower(uuid.New())
	if !isUUIDShapedAttributeEventID(token) {
		return uplinkStorageDeadLetter{}, false, fmt.Errorf("failed to create dead-letter claim token")
	}
	leaseUntil := now.Add(i.deadLetterLeaseDuration())
	// attempts 必须在获取租约时就自增，而不是等到 deferDeadLetterRetry 结算。
	// 否则 worker 在 replay 中途崩溃时该行从未记账：租约到期后被重新 claim，
	// attempts 永远停在原值，一条稳定压垮 worker 的记录会无限循环而不会到达
	// attributeEventDeadLetterMaxAttempts 上限。
	result := i.claimableDeadLetterQuery(ctx, now).
		Where("id = ? AND id IN (?)", id, scopeQuery.Select("id")).
		Updates(map[string]interface{}{
			"status":        uplinkDeadLetterStatusProcessing,
			"attempts":      gorm.Expr("attempts + 1"),
			"next_retry_at": nil,
			"claim_token":   token,
			"lease_until":   leaseUntil,
			"updated_at":    now,
		})
	if result.Error != nil {
		return uplinkStorageDeadLetter{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return uplinkStorageDeadLetter{}, false, nil
	}

	var claimed uplinkStorageDeadLetter
	if err := i.db.WithContext(ctx).
		Where("id = ? AND status = ? AND claim_token = ?", id, uplinkDeadLetterStatusProcessing, token).
		Take(&claimed).Error; err != nil {
		releaseCtx, cancel := context.WithTimeout(context.Background(), attributeEventDeadLetterSettlementTimeout)
		defer cancel()
		releaseErr := i.releaseDeadLetterClaim(releaseCtx, id, token, now)
		if releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("release unmaterialized dead-letter claim: %w", releaseErr))
		}
		return uplinkStorageDeadLetter{}, false, fmt.Errorf("load claimed attribute/event dead letter: %w", err)
	}
	return claimed, true, nil
}

func normalizeAttributeEventDeadLetterDrainLimit(limit int) int {
	if limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func attributeEventDeadLetterMetadata(row uplinkStorageDeadLetter) AttributeEventDeadLetterMetadata {
	return AttributeEventDeadLetterMetadata{
		ID:          row.ID,
		DataType:    row.DataType,
		DeviceID:    row.DeviceID,
		TenantID:    row.TenantID,
		TS:          row.TS,
		Status:      row.Status,
		Attempts:    row.Attempts,
		LastError:   row.LastError,
		NextRetryAt: row.NextRetryAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
