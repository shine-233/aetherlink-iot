package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	uplinkDeadLetterStatusPending    = "pending"
	uplinkDeadLetterStatusRetrying   = "retrying"
	uplinkDeadLetterStatusProcessing = "processing"
	uplinkDeadLetterStatusResolved   = "resolved"
	uplinkDeadLetterStatusDead       = "dead"

	attributeEventDeadLetterLeaseDuration     = 5 * time.Minute
	attributeEventDeadLetterSettlementTimeout = 2 * time.Second
)

var errAttributeEventIngressStopped = errors.New("attribute/event durable input is stopped")

// uplinkStorageReceipt is the authoritative primary-table receipt for one
// complete attribute envelope. It is inserted in the same transaction as all
// attribute rows, so a receipt can never describe a partially committed
// envelope. Event envelopes use event_datas.id itself as the receipt.
type uplinkStorageReceipt struct {
	ID          string          `gorm:"column:id;primaryKey"`
	Fingerprint string          `gorm:"column:fingerprint"`
	DataType    DataType        `gorm:"column:data_type"`
	DeviceID    string          `gorm:"column:device_id"`
	TenantID    string          `gorm:"column:tenant_id"`
	TS          int64           `gorm:"column:ts"`
	Payload     json.RawMessage `gorm:"column:payload;type:jsonb"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
}

func (uplinkStorageReceipt) TableName() string { return "uplink_storage_receipts" }

// uplinkStorageDeadLetter is deliberately free of device foreign keys: a
// malformed relationship or concurrently deleted device must not prevent the
// full, checksummed envelope from reaching the final database fallback.
type uplinkStorageDeadLetter struct {
	ID          string          `gorm:"column:id;primaryKey"`
	DataType    DataType        `gorm:"column:data_type"`
	DeviceID    string          `gorm:"column:device_id"`
	TenantID    string          `gorm:"column:tenant_id"`
	TS          int64           `gorm:"column:ts"`
	Payload     json.RawMessage `gorm:"column:payload;type:jsonb"`
	Status      string          `gorm:"column:status"`
	Attempts    int             `gorm:"column:attempts"`
	LastError   string          `gorm:"column:last_error"`
	NextRetryAt *time.Time      `gorm:"column:next_retry_at"`
	ClaimToken  *string         `gorm:"column:claim_token"`
	LeaseUntil  *time.Time      `gorm:"column:lease_until"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
}

func (uplinkStorageDeadLetter) TableName() string { return "uplink_storage_dead_letters" }

// attributeEventIngress coordinates canonical envelope validation, PostgreSQL
// transactions, filesystem durability and replay behind one
// small caller contract.
type attributeEventIngress struct {
	db      *gorm.DB
	logger  Logger
	config  Config
	metrics *metricsCollector
	spool   *attributeEventFileSpool

	mu        sync.Mutex
	accepting bool
	active    int
	idle      chan struct{}
	started   bool

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	doneOnce sync.Once
}

func newAttributeEventIngress(
	db *gorm.DB,
	logger Logger,
	config Config,
	metrics *metricsCollector,
) *attributeEventIngress {
	idle := make(chan struct{})
	close(idle)
	return &attributeEventIngress{
		db:        db,
		logger:    logger,
		config:    config,
		metrics:   metrics,
		spool:     newAttributeEventFileSpool(config),
		accepting: true,
		idle:      idle,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (i *attributeEventIngress) start(ctx context.Context) (err error) {
	if i == nil {
		return fmt.Errorf("attribute/event durable input is unavailable")
	}
	defer func() {
		if err == nil {
			return
		}
		i.mu.Lock()
		started := i.started
		i.mu.Unlock()
		if !started {
			i.requestStop()
			i.doneOnce.Do(func() { close(i.doneCh) })
		}
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	if i.config.AttributeEventSpoolReplayInterval <= 0 {
		return fmt.Errorf("attribute/event replay interval must be positive")
	}
	if i.config.AttributeEventSpoolReplayBatchSize <= 0 {
		return fmt.Errorf("attribute/event replay batch size must be positive")
	}
	if i.config.AttributeEventSpoolReplayTimeout <= 0 {
		return fmt.Errorf("attribute/event replay timeout must be positive")
	}
	if i.spool != nil {
		if err := i.spool.init(); err != nil {
			return fmt.Errorf("initialize attribute/event file spool: %w", err)
		}
		i.updateSpoolUsage(i.spool.usage())
		if i.metrics != nil {
			i.metrics.addAttributeEventSpoolCorrupt(int64(i.spool.startupCorruptCount()))
			i.metrics.setAttributeEventSpoolCapacity(
				i.config.AttributeEventSpoolMaxRecords,
				i.config.AttributeEventSpoolMaxBytes,
			)
		}
	}

	i.mu.Lock()
	if !i.accepting {
		i.mu.Unlock()
		return errAttributeEventIngressStopped
	}
	if i.started {
		i.mu.Unlock()
		return fmt.Errorf("attribute/event durable input already started")
	}
	i.started = true
	i.mu.Unlock()

	go i.run(ctx)
	return nil
}

func (i *attributeEventIngress) run(ctx context.Context) {
	defer i.doneOnce.Do(func() { close(i.doneCh) })
	// Recoverable backlog should not have to wait a full interval after start.
	i.replayOnce(ctx)
	ticker := time.NewTicker(i.config.AttributeEventSpoolReplayInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-i.stopCh:
			return
		case <-ticker.C:
			i.replayOnce(ctx)
		}
	}
}

func (i *attributeEventIngress) requestStop() {
	if i == nil {
		return
	}
	i.mu.Lock()
	i.accepting = false
	i.mu.Unlock()
	i.stopOnce.Do(func() { close(i.stopCh) })
}

func (i *attributeEventIngress) stop(timeout time.Duration) error {
	if i == nil {
		return nil
	}
	if timeout <= 0 {
		return fmt.Errorf("attribute/event durable input stop timeout must be positive")
	}
	deadline := time.Now().Add(timeout)
	i.requestStop()

	i.mu.Lock()
	started := i.started
	idle := i.idle
	i.mu.Unlock()
	if started && !waitForStorageDone(i.doneCh, time.Until(deadline)) {
		return fmt.Errorf("attribute/event replay stop timeout")
	}
	if !waitForStorageDone(idle, time.Until(deadline)) {
		return fmt.Errorf("attribute/event durable input drain timeout")
	}
	return nil
}

func (i *attributeEventIngress) begin() bool {
	if i == nil {
		return false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.accepting {
		return false
	}
	if i.active == 0 {
		i.idle = make(chan struct{})
	}
	i.active++
	return true
}

func (i *attributeEventIngress) finish() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.active == 0 {
		return
	}
	i.active--
	if i.active == 0 {
		close(i.idle)
	}
}

func (i *attributeEventIngress) accept(ctx context.Context, msg *Message) (DurabilityReceipt, error) {
	if !i.begin() {
		return DurabilityReceipt{}, errAttributeEventIngressStopped
	}
	defer i.finish()

	envelope, err := buildAttributeEventEnvelope(msg)
	if err != nil {
		return DurabilityReceipt{}, err
	}

	// Once admission has begun, caller cancellation must not interrupt a
	// partially completed durability chain. Each tier receives a fresh bounded
	// context so a slow primary attempt cannot consume the dead-letter or file
	// fallback budget.
	primaryCtx, primaryCancel := i.newDurabilityContext(ctx)
	primaryErr := i.persistEnvelope(primaryCtx, envelope, true)
	primaryCancel()
	if primaryErr == nil {
		// Do not consult ctx.Err() here. A transaction that has committed is a
		// successful durable outcome even if cancellation races with its return.
		return DurabilityReceipt{MessageID: envelope.Identity, Tier: DurabilityTierPrimary}, nil
	}

	deadLetterCtx, deadLetterCancel := i.newDurabilityContext(ctx)
	deadLetterErr := i.persistDeadLetter(deadLetterCtx, envelope)
	deadLetterCancel()
	if deadLetterErr == nil {
		if i.logger != nil {
			i.logger.Warnf(
				"attribute/event envelope retained in PostgreSQL dead letter after primary write failure: message_id=%s data_type=%s device_id=%s",
				envelope.Identity,
				envelope.Kind,
				envelope.DeviceID,
			)
		}
		return DurabilityReceipt{MessageID: envelope.Identity, Tier: DurabilityTierPostgresDeadLetter}, nil
	}
	if i.metrics != nil {
		i.metrics.incAttributeEventDeadLetterFailed()
	}

	if i.spool == nil {
		i.recordSpoolFailure()
		return DurabilityReceipt{}, errors.Join(
			fmt.Errorf("primary attribute/event write: %w", primaryErr),
			fmt.Errorf("persist attribute/event dead letter: %w", deadLetterErr),
			fmt.Errorf("attribute/event file spool is disabled"),
		)
	}

	fileCtx, fileCancel := i.newDurabilityContext(ctx)
	result, spoolErr := i.spool.store(fileCtx, envelope, time.Now().UTC())
	fileCancel()
	i.recordSpoolStoreResult(result)
	if spoolErr != nil {
		i.recordSpoolFailure()
		return DurabilityReceipt{}, errors.Join(
			fmt.Errorf("primary attribute/event write: %w", primaryErr),
			fmt.Errorf("persist attribute/event dead letter: %w", deadLetterErr),
			fmt.Errorf("store attribute/event file spool record: %w", spoolErr),
		)
	}
	if i.logger != nil {
		i.logger.Warnf(
			"attribute/event envelope retained in private file spool after PostgreSQL failure: message_id=%s data_type=%s device_id=%s",
			envelope.Identity,
			envelope.Kind,
			envelope.DeviceID,
		)
	}
	return DurabilityReceipt{MessageID: envelope.Identity, Tier: DurabilityTierFileSpool}, nil
}

func (i *attributeEventIngress) newDurabilityContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	timeout := i.config.AttributeEventSpoolReplayTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

func (i *attributeEventIngress) newReplayContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	timeout := i.config.AttributeEventSpoolReplayTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}

func (i *attributeEventIngress) persistDeadLetter(ctx context.Context, envelope attributeEventEnvelope) error {
	if i == nil || i.db == nil {
		return fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	if _, err := validateAttributeEventEnvelope(envelope); err != nil {
		return err
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal attribute/event dead letter: %w", err)
	}
	now := time.Now().UTC()
	row := uplinkStorageDeadLetter{
		ID:        envelope.Identity,
		DataType:  envelope.Kind,
		DeviceID:  envelope.DeviceID,
		TenantID:  envelope.TenantID,
		TS:        envelope.Timestamp,
		Payload:   payload,
		Status:    uplinkDeadLetterStatusPending,
		Attempts:  0,
		LastError: "PRIMARY_WRITE_FAILED",
		CreatedAt: now,
		UpdatedAt: now,
	}
	result := i.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		if i.metrics != nil {
			i.metrics.incAttributeEventDeadLettered()
		}
		return nil
	}
	var existing uplinkStorageDeadLetter
	if err := i.db.WithContext(ctx).Where("id = ?", envelope.Identity).Take(&existing).Error; err != nil {
		return fmt.Errorf("verify existing attribute/event dead letter: %w", err)
	}
	return verifyDeadLetterEnvelope(existing, envelope)
}

func verifyDeadLetterEnvelope(row uplinkStorageDeadLetter, envelope attributeEventEnvelope) error {
	stored, err := unmarshalAttributeEventEnvelope(row.Payload)
	if err != nil {
		return fmt.Errorf("validate stored attribute/event dead letter: %w", err)
	}
	// The row metadata must describe its own frozen first-writer envelope. A
	// protocol retry may have a later receive timestamp, so compare that retry
	// semantically only after the stored row has passed its integrity check.
	if row.ID != stored.Identity ||
		row.DataType != stored.Kind ||
		row.DeviceID != stored.DeviceID ||
		row.TenantID != stored.TenantID ||
		row.TS != stored.Timestamp {
		return fmt.Errorf("stored attribute/event dead-letter metadata mismatch")
	}
	if !equalAttributeEventEnvelopes(stored, envelope) {
		return fmt.Errorf("attribute/event dead-letter identity collision")
	}
	return nil
}

func (i *attributeEventIngress) replayDeadLetters(parent context.Context) error {
	if i == nil || i.db == nil {
		return fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	limit := i.config.AttributeEventSpoolReplayBatchSize
	if limit < 1 {
		return fmt.Errorf("attribute/event dead-letter replay batch size must be positive")
	}
	passCtx, passCancel := i.newReplayContext(parent)
	defer passCancel()
	now, err := i.deadLetterDatabaseNow(passCtx)
	if err != nil {
		return fmt.Errorf("read attribute/event dead-letter database clock: %w", err)
	}
	// 先回收已耗尽重试次数却仍停留在 processing 的行。attempts 现在在 claim 时自增，
	// 因此 worker 在最后一次尝试中途崩溃会留下 attempts >= 上限、租约已过期的孤儿行：
	// claimableDeadLetterQuery 的 attempts < 上限条件会永久跳过它，operator 的
	// UpdateAttributeEventDeadLetter 也只接受 pending/retrying/resolved/dead，
	// 结果这行既不会被重放也无法被人工处置。显式落到 dead 才能让它可见可处置。
	if err := i.reapExhaustedDeadLetterClaims(passCtx, now); err != nil {
		return fmt.Errorf("reap exhausted attribute/event dead-letter claims: %w", err)
	}

	var candidates []uplinkStorageDeadLetter
	err = i.claimableDeadLetterQuery(passCtx, now).
		Order("CASE WHEN status = 'processing' THEN COALESCE(lease_until, updated_at) ELSE COALESCE(next_retry_at, created_at) END ASC, created_at ASC, id ASC").
		Limit(limit).
		Find(&candidates).Error
	if err != nil {
		return fmt.Errorf("load attribute/event dead letters: %w", err)
	}

	var replayErrors []error
	for _, candidate := range candidates {
		select {
		case <-passCtx.Done():
			return errors.Join(errors.Join(replayErrors...), passCtx.Err())
		case <-i.stopCh:
			return errors.Join(errors.Join(replayErrors...), errAttributeEventIngressStopped)
		default:
		}
		row, claimed, claimErr := i.claimDeadLetter(passCtx, candidate.ID, now)
		if claimErr != nil {
			replayErrors = append(replayErrors, fmt.Errorf("claim dead letter %s: %w", candidate.ID, claimErr))
			continue
		}
		if !claimed {
			continue
		}
		if replayErr := i.replayClaimedAttributeEventDeadLetter(passCtx, row, now); replayErr != nil {
			replayErrors = append(replayErrors, fmt.Errorf("replay dead letter %s: %w", row.ID, replayErr))
		}
	}
	return errors.Join(replayErrors...)
}

func (i *attributeEventIngress) deadLetterDatabaseNow(ctx context.Context) (time.Time, error) {
	if i == nil || i.db == nil {
		return time.Time{}, fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	if i.db.Dialector != nil && i.db.Dialector.Name() == "postgres" {
		var now time.Time
		if err := i.db.WithContext(ctx).Raw("SELECT clock_timestamp()").Scan(&now).Error; err != nil {
			return time.Time{}, err
		}
		return now.UTC(), nil
	}
	return time.Now().UTC(), nil
}

func (i *attributeEventIngress) deadLetterLeaseDuration() time.Duration {
	duration := time.Duration(attributeEventDeadLetterLeaseDuration)
	if timeout := i.config.AttributeEventSpoolReplayTimeout; timeout >= duration {
		// The lease must outlive one bounded replay pass, otherwise a healthy
		// worker can be fenced by a second instance before it settles its row.
		duration = timeout + time.Minute
	}
	return duration
}

func (i *attributeEventIngress) claimableDeadLetterQuery(ctx context.Context, now time.Time) *gorm.DB {
	return i.db.WithContext(ctx).
		Model(&uplinkStorageDeadLetter{}).
		Where(
			"((status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND (lease_until IS NULL OR lease_until <= ?)))",
			[]string{uplinkDeadLetterStatusPending, uplinkDeadLetterStatusRetrying},
			now,
			uplinkDeadLetterStatusProcessing,
			now,
		).
		Where("attempts < ?", attributeEventDeadLetterMaxAttempts)
}

func (i *attributeEventIngress) claimDeadLetter(
	ctx context.Context,
	id string,
	now time.Time,
) (uplinkStorageDeadLetter, bool, error) {
	return i.claimDeadLetterWithFilter(ctx, id, now, AttributeEventDeadLetterFilter{})
}

func (i *attributeEventIngress) releaseDeadLetterClaim(
	ctx context.Context,
	id string,
	token string,
	now time.Time,
) error {
	result := i.db.WithContext(ctx).
		Model(&uplinkStorageDeadLetter{}).
		Where("id = ? AND status = ? AND claim_token = ?", id, uplinkDeadLetterStatusProcessing, token).
		Updates(map[string]interface{}{
			"status":        uplinkDeadLetterStatusRetrying,
			"next_retry_at": now,
			"claim_token":   nil,
			"lease_until":   nil,
			"updated_at":    now,
		})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (i *attributeEventIngress) updateClaimedDeadLetter(
	ctx context.Context,
	row uplinkStorageDeadLetter,
	updates map[string]interface{},
) error {
	if row.ClaimToken == nil || strings.TrimSpace(*row.ClaimToken) == "" {
		return fmt.Errorf("attribute/event dead-letter %s has no claim token", row.ID)
	}
	result := i.db.WithContext(ctx).
		Model(&uplinkStorageDeadLetter{}).
		Where(
			"id = ? AND status = ? AND claim_token = ?",
			row.ID,
			uplinkDeadLetterStatusProcessing,
			strings.TrimSpace(*row.ClaimToken),
		).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("attribute/event dead-letter %s replay lease was lost", row.ID)
	}
	return nil
}

func (i *attributeEventIngress) settleClaimedDeadLetter(
	ctx context.Context,
	row uplinkStorageDeadLetter,
	updates map[string]interface{},
) error {
	err := i.updateClaimedDeadLetter(ctx, row, updates)
	if err == nil || ctx == nil || ctx.Err() == nil {
		return err
	}
	detachedCtx, cancel := context.WithTimeout(context.Background(), attributeEventDeadLetterSettlementTimeout)
	defer cancel()
	if detachedErr := i.updateClaimedDeadLetter(detachedCtx, row, updates); detachedErr == nil {
		return nil
	} else {
		return errors.Join(err, fmt.Errorf("detached dead-letter settlement failed: %w", detachedErr))
	}
}

func (i *attributeEventIngress) deferDeadLetterRetry(
	ctx context.Context,
	row uplinkStorageDeadLetter,
	now time.Time,
) error {
	// claim 时已经自增过 attempts，这里的 row 是 claim 之后重新读出的行，
	// 因此直接沿用它的值，不能再 +1，否则同一次尝试会被记两次。
	attempts := row.Attempts
	if attempts >= attributeEventDeadLetterMaxAttempts {
		return i.settleClaimedDeadLetter(ctx, row, map[string]interface{}{
			"status":        uplinkDeadLetterStatusDead,
			"attempts":      attempts,
			"last_error":    "PRIMARY_REPLAY_FAILED",
			"next_retry_at": nil,
			"claim_token":   nil,
			"lease_until":   nil,
			"updated_at":    now,
		})
	}
	delay := time.Minute
	for count := 1; count < attempts && delay < 15*time.Minute; count++ {
		delay *= 2
	}
	if delay > 15*time.Minute {
		delay = 15 * time.Minute
	}
	nextRetryAt := now.Add(delay)
	return i.settleClaimedDeadLetter(ctx, row, map[string]interface{}{
		"status":        uplinkDeadLetterStatusRetrying,
		"attempts":      attempts,
		"last_error":    "PRIMARY_REPLAY_FAILED",
		"next_retry_at": nextRetryAt,
		"claim_token":   nil,
		"lease_until":   nil,
		"updated_at":    now,
	})
}

func (i *attributeEventIngress) markDeadLetterDead(
	ctx context.Context,
	row uplinkStorageDeadLetter,
	now time.Time,
) error {
	return i.settleClaimedDeadLetter(ctx, row, map[string]interface{}{
		"status":        uplinkDeadLetterStatusDead,
		"last_error":    "ENVELOPE_INVALID",
		"next_retry_at": nil,
		"claim_token":   nil,
		"lease_until":   nil,
		"updated_at":    now,
	})
}

// reapExhaustedDeadLetterClaims 把重试次数已耗尽、租约又已过期的 processing 行落到 dead。
// 这类行是 attempts 在 claim 时自增后的必然产物：worker 在最后一次尝试中途崩溃，
// 没有机会走 deferDeadLetterRetry 结算。不回收的话它对 claim 查询不可见（attempts 已达上限），
// 对 operator 动作也不可用（只接受 pending/retrying/resolved/dead），成为无人可处置的孤儿。
// 这里不依赖 claim token：正是持有 token 的 worker 已经消失才需要回收。
func (i *attributeEventIngress) reapExhaustedDeadLetterClaims(ctx context.Context, now time.Time) error {
	return i.reapExhaustedDeadLetterClaimsWithFilter(ctx, now, AttributeEventDeadLetterFilter{})
}

func (i *attributeEventIngress) reapExhaustedDeadLetterClaimsWithFilter(
	ctx context.Context,
	now time.Time,
	filter AttributeEventDeadLetterFilter,
) error {
	if i == nil || i.db == nil {
		return fmt.Errorf("attribute/event dead-letter database is unavailable")
	}
	scopeQuery, err := i.attributeEventDeadLetterScopeQuery(ctx, filter)
	if err != nil {
		return err
	}
	return i.db.WithContext(ctx).
		Model(&uplinkStorageDeadLetter{}).
		Where(
			"status = ? AND attempts >= ? AND (lease_until IS NULL OR lease_until <= ?) AND id IN (?)",
			uplinkDeadLetterStatusProcessing,
			attributeEventDeadLetterMaxAttempts,
			now,
			scopeQuery.Select("id"),
		).
		Updates(map[string]interface{}{
			"status":        uplinkDeadLetterStatusDead,
			"last_error":    "PRIMARY_REPLAY_FAILED",
			"next_retry_at": nil,
			"claim_token":   nil,
			"lease_until":   nil,
			"updated_at":    now,
		}).Error
}

func (i *attributeEventIngress) persistEnvelope(
	ctx context.Context,
	envelope attributeEventEnvelope,
	recordDatabaseMetrics bool,
) error {
	if i == nil || i.db == nil {
		return fmt.Errorf("attribute/event database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := validateAttributeEventEnvelope(envelope); err != nil {
		return err
	}

	var attributeWritten int64
	var eventWritten int64
	err := i.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		switch envelope.Kind {
		case DataTypeAttribute:
			attributeWritten, err = insertAttributeEnvelope(tx, envelope)
		case DataTypeEvent:
			eventWritten, err = insertEventEnvelope(tx, envelope)
		default:
			err = fmt.Errorf("unsupported durable envelope kind %q", envelope.Kind)
		}
		return err
	})
	if err != nil {
		if recordDatabaseMetrics && i.metrics != nil {
			switch envelope.Kind {
			case DataTypeAttribute:
				var points []canonicalAttributePoint
				if json.Unmarshal(envelope.Payload, &points) == nil {
					i.metrics.addAttributeFailed(int64(len(points)))
				}
			case DataTypeEvent:
				i.metrics.incEventFailed()
			}
		}
		return err
	}
	if i.metrics != nil {
		i.metrics.addAttributeWritten(attributeWritten)
		i.metrics.addEventWritten(eventWritten)
	}
	return nil
}

func insertAttributeEnvelope(tx *gorm.DB, envelope attributeEventEnvelope) (int64, error) {
	duplicate, err := claimAttributeEnvelopeReceipt(tx, envelope)
	if err != nil {
		return 0, err
	}
	if duplicate {
		return 0, nil
	}

	var points []canonicalAttributePoint
	if err := json.Unmarshal(envelope.Payload, &points); err != nil {
		return 0, fmt.Errorf("decode canonical attribute payload: %w", err)
	}
	var written int64
	for index, point := range points {
		value, err := decodeCanonicalAttributeValue(point.Value)
		if err != nil {
			return 0, fmt.Errorf("decode attribute point %d (%q): %w", index, point.Key, err)
		}
		boolV, numberV, stringV := convertValue(value)
		row := AttributeData{
			ID:       deterministicAttributeEventRowID("attribute", envelope.Identity, point.Key),
			DeviceID: envelope.DeviceID,
			Key:      point.Key,
			TS:       time.UnixMilli(envelope.Timestamp),
			BoolV:    boolV,
			NumberV:  numberV,
			StringV:  stringV,
			TenantID: envelope.TenantID,
		}
		result := tx.Clauses(AttributeCurrentUpsertClause()).Create(&row)
		if result.Error != nil {
			return 0, fmt.Errorf("insert attribute point %d (%q): %w", index, point.Key, result.Error)
		}
		written += result.RowsAffected
	}
	return written, nil
}

func insertEventEnvelope(tx *gorm.DB, envelope attributeEventEnvelope) (int64, error) {
	var payload canonicalEventPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return 0, fmt.Errorf("decode canonical event payload: %w", err)
	}
	row := EventDataModel{
		// event_datas.id is the durable envelope receipt itself. Primary write,
		// dead letter, file spool and every replay therefore share one ID.
		ID:       envelope.Identity,
		DeviceID: envelope.DeviceID,
		Identify: payload.Identify,
		TS:       time.UnixMilli(envelope.Timestamp),
		Data:     append(json.RawMessage(nil), payload.Data...),
		TenantID: envelope.TenantID,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&row)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		return result.RowsAffected, nil
	}

	var existing EventDataModel
	if err := tx.Where("id = ?", row.ID).Take(&existing).Error; err != nil {
		return 0, fmt.Errorf("verify existing event envelope: %w", err)
	}
	existingData, err := canonicalizeRawJSON(existing.Data)
	if err != nil {
		return 0, fmt.Errorf("canonicalize existing event data: %w", err)
	}
	if existing.DeviceID != row.DeviceID ||
		existing.TenantID != row.TenantID ||
		existing.Identify != row.Identify ||
		!bytes.Equal(existingData, row.Data) {
		return 0, fmt.Errorf("event deterministic identity collision")
	}
	return 0, nil
}

func claimAttributeEnvelopeReceipt(tx *gorm.DB, envelope attributeEventEnvelope) (bool, error) {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return false, fmt.Errorf("marshal attribute receipt envelope: %w", err)
	}
	receipt := uplinkStorageReceipt{
		ID:          envelope.Identity,
		Fingerprint: envelope.Fingerprint,
		DataType:    envelope.Kind,
		DeviceID:    envelope.DeviceID,
		TenantID:    envelope.TenantID,
		TS:          envelope.Timestamp,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&receipt)
	if result.Error != nil {
		return false, fmt.Errorf("insert attribute envelope receipt: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return false, nil
	}

	var existing uplinkStorageReceipt
	if err := tx.Where("id = ?", envelope.Identity).Take(&existing).Error; err != nil {
		return false, fmt.Errorf("verify attribute envelope receipt: %w", err)
	}
	if err := verifyReceiptEnvelope(existing, envelope); err != nil {
		return false, err
	}
	return true, nil
}

func verifyReceiptEnvelope(receipt uplinkStorageReceipt, envelope attributeEventEnvelope) error {
	stored, err := unmarshalAttributeEventEnvelope(receipt.Payload)
	if err != nil {
		return fmt.Errorf("validate stored attribute receipt: %w", err)
	}
	if receipt.ID != stored.Identity ||
		receipt.Fingerprint != stored.Fingerprint ||
		receipt.DataType != stored.Kind ||
		receipt.DeviceID != stored.DeviceID ||
		receipt.TenantID != stored.TenantID ||
		receipt.TS != stored.Timestamp {
		return fmt.Errorf("stored attribute envelope receipt metadata mismatch")
	}
	if !equalAttributeEventEnvelopes(stored, envelope) {
		return fmt.Errorf("attribute envelope receipt identity collision")
	}
	return nil
}

func (i *attributeEventIngress) replayOnce(parent context.Context) {
	if i == nil {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	select {
	case <-i.stopCh:
		return
	default:
	}
	deadLetterErr := i.replayDeadLetters(parent)
	if deadLetterErr != nil && i.logger != nil {
		i.logger.Errorf("attribute/event PostgreSQL dead-letter replay incomplete: %v", deadLetterErr)
	}
	// A stop request can arrive while the database replay is in progress. Do
	// not start another potentially blocking filesystem pass after shutdown has
	// already closed producer admission; accepted writes are drained separately.
	select {
	case <-i.stopCh:
		return
	default:
	}
	if i.spool == nil {
		return
	}
	ctx, cancel := i.newReplayContext(parent)
	result, err := i.spool.replay(
		ctx,
		i.config.AttributeEventSpoolReplayBatchSize,
		func(replayCtx context.Context, envelope attributeEventEnvelope) error {
			return i.persistEnvelope(replayCtx, envelope, false)
		},
	)
	cancel()
	if i.metrics != nil {
		i.metrics.addAttributeEventSpoolReplayed(int64(result.Replayed))
		i.metrics.addAttributeEventSpoolCorrupt(int64(result.Corrupt))
	}
	i.updateSpoolUsage(result.Usage)
	if err != nil && i.logger != nil {
		i.logger.Errorf(
			"attribute/event file spool replay incomplete: attempted=%d replayed=%d corrupt=%d: %v",
			result.Attempted,
			result.Replayed,
			result.Corrupt,
			err,
		)
	}
}

func (i *attributeEventIngress) recordSpoolStoreResult(result attributeEventFileSpoolStoreResult) {
	if i == nil {
		return
	}
	if i.metrics != nil {
		if result.Stored {
			i.metrics.incAttributeEventSpooled()
		}
		i.metrics.addAttributeEventSpoolCorrupt(int64(result.Corrupt))
	}
	if i.spool != nil {
		i.updateSpoolUsage(i.spool.usage())
	}
}

func (i *attributeEventIngress) recordSpoolFailure() {
	if i != nil && i.metrics != nil {
		i.metrics.incAttributeEventSpoolFailed()
	}
}

func (i *attributeEventIngress) updateSpoolUsage(usage attributeEventFileSpoolUsage) {
	if i != nil && i.metrics != nil {
		i.metrics.setAttributeEventSpoolUsage(usage)
	}
}
