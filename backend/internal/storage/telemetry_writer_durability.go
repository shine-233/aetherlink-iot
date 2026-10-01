// telemetry_writer_durability.go owns every durability tier of the telemetry
// writer: pre-buffer write-ahead receipts, rejected-message persistence, the
// PostgreSQL dead-letter table and the independent file spool fallback.

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (w *telemetryWriter) prepareTelemetryWriteAhead(ctx context.Context, msg *Message) error {
	if w == nil || !w.config.TelemetryWriteAheadSpoolEnabled {
		return nil
	}
	if msg == nil || msg.DataType != DataTypeTelemetry {
		return fmt.Errorf("pre-enqueue message is not telemetry")
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if w.spool == nil {
		return fmt.Errorf("telemetry write-ahead spool is unavailable")
	}
	defer w.refreshTelemetrySpoolMetrics()
	item, err := telemetryBatchItemFromMessage(msg)
	if err != nil {
		return err
	}
	historyData, _ := item.convertedRows()
	// One grouped directory fsync covers every point of the message.
	_, errs := w.spool.storeBatch(ctx, historyData, time.Now())
	for _, err := range errs {
		if err != nil {
			return fmt.Errorf("persist telemetry write-ahead receipt: %w", err)
		}
	}
	msg.telemetryWriteAheadPrepared = true
	return nil
}

// persistRejectedTelemetry converts a rejected message through the same typed
// row path as normal batching, then applies the existing PostgreSQL
// dead-letter -> independent file spool durability order to every unique point.
func (w *telemetryWriter) persistRejectedTelemetry(ctx context.Context, msg *Message, cause error) error {
	if w == nil {
		return fmt.Errorf("telemetry writer is unavailable")
	}
	item, err := telemetryBatchItemFromMessage(msg)
	if err != nil {
		return err
	}
	return w.persistRejectedItem(ctx, item, cause)
}

// persistRejectedItem is persistRejectedTelemetry for an already converted
// item, so write() never converts the same message twice.
func (w *telemetryWriter) persistRejectedItem(ctx context.Context, item *telemetryBatchItem, cause error) error {
	historyData, _ := item.convertedRows()

	var persistErr error
	for _, history := range historyData {
		if err := w.persistFailedTelemetryContext(ctx, history, cause); err != nil {
			persistErr = errors.Join(
				persistErr,
				fmt.Errorf(
					"device_id=%s key=%s ts=%d: %w",
					history.DeviceID,
					history.Key,
					history.TS,
					err,
				),
			)
		}
	}
	return persistErr
}

// storeWriteAheadReceipts 在遥测点进入内存缓冲区之前，先把它们写成 spool
// receipt。内存缓冲区在 SIGKILL、断电或 panic 时不留任何痕迹，既有的
// dead-letter/spool 兜底只覆盖"数据库写入失败"，覆盖不到"还没写就没了"。
//
// 落盘用的是与失败兜底完全相同的 store 路径，因此 identity 仍是
// (device_id,key,ts) 的确定性派生，重复投递被视为成功，重放路径也无需改动。
//
// 返回值只在主库确认写入后用于删除。启用 write-ahead 时任一点落盘失败
// 都会返回错误并阻止该消息进入内存缓冲，保持 fail-closed。
func (w *telemetryWriter) storeWriteAheadReceipts(item *telemetryBatchItem) ([]telemetryWriteAheadReceipt, error) {
	if w == nil || w.spool == nil || !w.config.TelemetryWriteAheadSpoolEnabled || item == nil {
		if w != nil && w.config.TelemetryWriteAheadSpoolEnabled {
			return nil, fmt.Errorf("telemetry write-ahead spool is unavailable")
		}
		return nil, nil
	}
	defer w.refreshTelemetrySpoolMetrics()
	// The rows converted here are cached on the item and reused by doFlush.
	historyData, _ := item.convertedRows()
	if len(historyData) == 0 {
		return nil, nil
	}
	receipts := make([]telemetryWriteAheadReceipt, 0, len(historyData))
	// Group commit: every point of the item is file-fsynced and renamed, then a
	// single directory fsync acknowledges them all. No point is admitted to the
	// memory buffer before that fsync completed.
	results, errs := w.spool.storeBatch(context.Background(), historyData, time.Now())
	for index, history := range historyData {
		if err := errs[index]; err != nil {
			// 容量耗尽或文件系统错误时拒绝本次内存写入；上游可重试，
			// 已经成功落盘的确定性 receipt 会由重放路径继续处理。
			if w.logger != nil {
				w.logger.Warnf(
					"telemetry write-ahead receipt failed; rejecting buffer admission: device_id=%s key=%s ts=%d: %v",
					history.DeviceID, history.Key, history.TS, err,
				)
			}
			return nil, err
		}
	}
	for index, history := range historyData {
		result := results[index]
		// 确定性重复也算已持久：identity 已经在盘上，但它不是本次新建的记录，
		// 删除应交给先写入它的那一方，避免两个批次互相删掉对方的 receipt。
		if result.Stored || (item.writeAheadPrepared && result.Duplicate) {
			receipts = append(receipts, telemetryWriteAheadReceipt{history: history})
		}
	}
	return receipts, nil
}

// releaseWriteAheadReceipts 在主库确认写入后删除对应 receipt。删除失败不算
// 数据问题：记录仍在盘上，重放是幂等的，最坏结果只是多一次无害重放。
func (w *telemetryWriter) releaseWriteAheadReceipts(batch []*telemetryBatchItem) {
	if w == nil || w.spool == nil {
		return
	}
	defer w.refreshTelemetrySpoolMetrics()
	for _, item := range batch {
		if item == nil {
			continue
		}
		for _, receipt := range item.writeAhead {
			if err := removeTelemetryWriteAheadReceipt(w.spool, receipt.history); err != nil {
				if w.logger != nil {
					w.logger.Warnf(
						"release telemetry write-ahead receipt failed, replay will retry idempotently: device_id=%s key=%s ts=%d: %v",
						receipt.history.DeviceID, receipt.history.Key, receipt.history.TS, err,
					)
				}
			}
		}
		item.writeAhead = nil
	}
}

func (w *telemetryWriter) persistFailedTelemetry(history TelemetryData, cause error) error {
	ctx, cancel := w.newDurabilityContext(context.Background())
	defer cancel()
	return w.persistFailedTelemetryContext(ctx, history, cause)
}

func (w *telemetryWriter) persistFailedTelemetryContext(ctx context.Context, history TelemetryData, cause error) error {
	deadLetterErr := w.recordTelemetryDeadLetterContext(ctx, history, cause)
	if deadLetterErr == nil {
		return nil
	}
	if w.spool == nil {
		if w.metrics != nil {
			w.metrics.incTelemetrySpoolFailed()
		}
		return errors.Join(deadLetterErr, fmt.Errorf("telemetry file spool is disabled"))
	}
	now := time.Now().UTC()
	storeResult, err := w.spool.store(context.Background(), history, now)
	usage := w.spool.usage()
	if w.metrics != nil {
		w.metrics.addTelemetrySpoolCorrupt(int64(storeResult.Corrupt))
		w.metrics.setTelemetrySpoolUsage(usage)
	}
	if storeResult.Corrupt > 0 && w.logger != nil {
		w.logger.Warnf(
			"telemetry file spool detected corrupt deterministic record before replacement: ts=%d detected=%d quarantined=%d backlog=%d bytes=%d quarantine_records=%d quarantine_bytes=%d (device/key omitted: device-controlled strings are kept out of logs per log-injection gate)",
			history.TS,
			storeResult.Corrupt,
			storeResult.Quarantined,
			usage.Records,
			usage.Bytes,
			usage.QuarantinedRecords,
			usage.QuarantinedBytes,
		)
	}
	if err != nil {
		if w.metrics != nil {
			w.metrics.incTelemetrySpoolFailed()
		}
		return errors.Join(deadLetterErr, fmt.Errorf("store telemetry file spool record: %w", err))
	}
	if storeResult.Stored {
		if w.metrics != nil {
			w.metrics.incTelemetrySpooled()
		}
		if w.logger != nil {
			w.logger.Warnf(
				"telemetry saved to independent file spool after PostgreSQL dead-letter failure: ts=%d backlog=%d bytes=%d quarantine_records=%d quarantine_bytes=%d (device/key omitted: device-controlled strings are kept out of logs per log-injection gate)",
				history.TS,
				usage.Records,
				usage.Bytes,
				usage.QuarantinedRecords,
				usage.QuarantinedBytes,
			)
		}
	} else if storeResult.Duplicate && w.logger != nil {
		w.logger.Debugf(
			"telemetry file spool already contains durable record: ts=%d backlog=%d bytes=%d (device/key omitted: device-controlled strings are kept out of logs per log-injection gate)",
			history.TS,
			usage.Records,
			usage.Bytes,
		)
	}
	return nil
}

func (w *telemetryWriter) recordTelemetryDeadLetter(history TelemetryData, cause error) error {
	ctx, cancel := w.newDurabilityContext(context.Background())
	defer cancel()
	return w.recordTelemetryDeadLetterContext(ctx, history, cause)
}

func (w *telemetryWriter) newDurabilityContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	timeout := 10 * time.Second
	if w != nil && w.config.TelemetrySpoolReplayTimeout > 0 {
		timeout = w.config.TelemetrySpoolReplayTimeout
	}
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

func (w *telemetryWriter) recordTelemetryDeadLetterContext(ctx context.Context, history TelemetryData, cause error) error {
	if w == nil || w.db == nil {
		return fmt.Errorf("telemetry dead-letter database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db := w.db.WithContext(ctx)
	var existing TelemetryDeadLetter
	lookup := db.Where(
		"device_id = ? AND key = ? AND ts = ?",
		history.DeviceID,
		history.Key,
		history.TS,
	).Order("created_at ASC, id ASC").Take(&existing)
	if lookup.Error == nil {
		return w.acceptExistingTelemetryDeadLetter(existing, history)
	}
	if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("lookup existing telemetry dead-letter: %w", lookup.Error)
	}

	now := time.Now().UTC()
	deadLetter := buildTelemetryDeadLetter(history, cause, now)
	insert := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&deadLetter)
	if insert.Error != nil {
		if w.logger != nil {
			w.logger.Errorf("telemetry dead-letter insert failed: ts=%d, err=%v (device/key omitted: device-controlled strings are kept out of logs per log-injection gate)", history.TS, insert.Error)
		}
		return insert.Error
	}
	if insert.RowsAffected == 0 {
		if err := db.Where("id = ?", deadLetter.ID).Take(&existing).Error; err != nil {
			return fmt.Errorf("verify existing telemetry dead-letter: %w", err)
		}
		return w.acceptExistingTelemetryDeadLetter(existing, history)
	}
	return nil
}

func (w *telemetryWriter) acceptExistingTelemetryDeadLetter(existing TelemetryDeadLetter, history TelemetryData) error {
	if existing.DeviceID != history.DeviceID || existing.TenantID != history.TenantID || existing.Key != history.Key || existing.TS != history.TS {
		return fmt.Errorf("telemetry dead-letter deterministic identity collision")
	}
	if w.logger != nil {
		w.logger.Debugf(
			"telemetry dead-letter already contains durable record: ts=%d (device/key omitted: device-controlled strings are kept out of logs per log-injection gate)",
			history.TS,
		)
	}
	return nil
}

func buildTelemetryDeadLetter(history TelemetryData, cause error, now time.Time) TelemetryDeadLetter {
	payload, _ := json.Marshal(history)
	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}
	return TelemetryDeadLetter{
		ID:         telemetryDeadLetterID(history),
		DeviceID:   history.DeviceID,
		TenantID:   history.TenantID,
		Key:        history.Key,
		TS:         history.TS,
		BoolV:      history.BoolV,
		NumberV:    history.NumberV,
		StringV:    history.StringV,
		RawPayload: payload,
		Status:     "pending",
		Attempts:   1,
		LastError:  lastError,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func telemetryDeadLetterID(history TelemetryData) string {
	// Use the same authoritative history identity as the file spool, shortened
	// to the existing varchar(36) UUID-shaped dead-letter primary key. This keeps
	// repeated failures idempotent without changing the database schema.
	identity := telemetryFileSpoolIdentity(history)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		identity[0:8],
		identity[8:12],
		identity[12:16],
		identity[16:20],
		identity[20:32],
	)
}
