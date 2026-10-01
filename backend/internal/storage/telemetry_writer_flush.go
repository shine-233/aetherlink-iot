// telemetry_writer_flush.go owns the batch flush pipeline: buffer hand-off,
// the primary batch transaction and the chunk/single-row fallbacks.
//
// Invariants:
//   - doFlush runs under flushMu, so at most one telemetry batch transaction is
//     in flight per writer.
//   - Rows reach the database sorted by (device_id,key[,ts]); with a single
//     global lock order, concurrent writers (this flusher, spool replay, other
//     instances) can only wait on each other, never deadlock.
//   - A transaction never carries more than TelemetryBatchSize items, even when
//     the buffer grew past one batch while a previous flush was in flight.

package storage

import (
	"encoding/json"
	"fmt"

	"aetherlink-iot/backend/internal/diagnostics"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// telemetryFallbackChunkSize is the row count of one fallback transaction.
	telemetryFallbackChunkSize = 100
	// telemetryFailurePreviewRows caps the rows serialized into a failure log.
	telemetryFailurePreviewRows = 5
)

// takeBuffer swaps the buffer out under bufferMu and wakes producers blocked
// at the high-water mark. replacement is the new buffer (nil on shutdown).
func (w *telemetryWriter) takeBuffer(replacement []*telemetryBatchItem) []*telemetryBatchItem {
	w.bufferMu.Lock()
	defer w.bufferMu.Unlock()
	batch := w.buffer
	if len(batch) == 0 && replacement != nil {
		return nil
	}
	w.buffer = replacement
	if w.drained != nil {
		w.drained.Broadcast()
	}
	return batch
}

// flush 刷新缓冲区: drains everything buffered so far in batch-sized
// transactions.
func (w *telemetryWriter) flush() {
	batch := w.takeBuffer(make([]*telemetryBatchItem, 0, w.batchSize()))
	w.flushInBatches(batch)
}

// flushRemaining 刷新剩余数据（停止时调用）
func (w *telemetryWriter) flushRemaining() {
	batch := w.takeBuffer(nil)
	if len(batch) > 0 {
		if w.logger != nil {
			w.logger.Infof("flushing remaining %d telemetry items", len(batch))
		}
		w.flushInBatches(batch)
	}
}

func (w *telemetryWriter) flushInBatches(batch []*telemetryBatchItem) {
	size := w.batchSize()
	for start := 0; start < len(batch); start += size {
		w.doFlush(batch[start:min(start+size, len(batch))])
	}
}

// doFlush 执行实际的刷新操作
func (w *telemetryWriter) doFlush(batch []*telemetryBatchItem) {
	w.flushMu.Lock()
	defer w.flushMu.Unlock()
	if hook := w.flushHook; hook != nil {
		hook(true)
		defer hook(false)
	}

	// 1. 合并每项已缓存的转换结果，跨批次去重并按锁顺序排序
	historyData, currentData, duplicates := w.deduplicateAndConvert(batch)

	// 记录批次内重复数
	if duplicates > 0 && w.metrics != nil {
		w.metrics.addTelemetryDuplicates(int64(duplicates))
	}

	if len(historyData) == 0 {
		return
	}

	// 2. 批量写入数据库
	written, failedRows := w.batchInsert(historyData, currentData)
	failed := len(failedRows)

	// 2b. 只释放主库已确认行的 write-ahead receipt。失败行的 receipt 保留，
	// 交给既有 spool 重放；部分失败不再拖累同批已写入的行被重复重放。
	w.releaseWriteAheadReceipts(batch, failedRows)

	// 3. 记录监控指标
	if w.metrics != nil {
		w.metrics.addTelemetryWritten(int64(written))
		w.metrics.addTelemetryFailed(int64(failed))
		w.metrics.recordTelemetryBatch(len(historyData))
	}

	if w.logger != nil {
		w.logger.Debugf("【设备诊断】flushed batch: total=%d, written=%d, failed=%d, duplicates=%d",
			len(historyData), written, failed, duplicates)
	}
}

// telemetryFailedRows is the set of history identities the primary database
// did not confirm. nil means every row was written.
type telemetryFailedRows map[telemetryPointIdentity]struct{}

func (f *telemetryFailedRows) add(row TelemetryData) {
	if *f == nil {
		*f = make(telemetryFailedRows)
	}
	(*f)[telemetryIdentityOf(row)] = struct{}{}
}

func (f telemetryFailedRows) has(row TelemetryData) bool {
	_, ok := f[telemetryIdentityOf(row)]
	return ok
}

// batchInsert 批量插入数据库; returns the written count and the rows that
// failed even the single-row fallback.
func (w *telemetryWriter) batchInsert(historyData []TelemetryData, currentData []TelemetryCurrentData) (int, telemetryFailedRows) {
	err := w.db.Transaction(func(tx *gorm.DB) error {
		return w.insertTelemetryBatch(tx, historyData, currentData)
	})

	if err != nil {
		w.logTelemetryBatchFailure("batch insert failed", len(historyData), err, historyData)
		return w.fallbackInsert(historyData, currentData)
	}

	return len(historyData), nil
}

func telemetryHistoryConflictClause() clause.OnConflict {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "key"}, {Name: "ts"}},
		DoNothing: true,
	}
}

func (w *telemetryWriter) insertTelemetryBatch(tx *gorm.DB, historyData []TelemetryData, currentData []TelemetryCurrentData) error {
	if len(historyData) > 0 {
		if err := tx.Clauses(telemetryHistoryConflictClause()).Create(&historyData).Error; err != nil {
			return fmt.Errorf("insert history data failed: %w", err)
		}
	}

	if len(currentData) > 0 {
		if err := tx.Clauses(TelemetryCurrentUpsertClause()).Create(&currentData).Error; err != nil {
			return fmt.Errorf("insert current data failed: %w", err)
		}
	}

	return nil
}

// logTelemetryBatchFailure logs one line with a bounded JSON preview. JSON
// encoding keeps device-controlled strings escaped (log-injection gate).
func (w *telemetryWriter) logTelemetryBatchFailure(prefix string, total int, err error, historyData []TelemetryData) {
	if w.logger == nil {
		return
	}
	previewRows := telemetryHistoryPreviewRows(historyData, telemetryFailurePreviewRows)
	if j, jerr := json.Marshal(previewRows); jerr == nil {
		w.logger.Errorf("%s: total=%d, err=%v, preview=%s", prefix, total, err, string(j))
		return
	}
	w.logger.Errorf("%s: total=%d, err=%v", prefix, total, err)
}

// fallbackInsert 分块兜底（批量失败时使用）: one transaction per chunk, then
// one per row only inside a failing chunk, isolating poison rows.
func (w *telemetryWriter) fallbackInsert(historyData []TelemetryData, currentData []TelemetryCurrentData) (written int, failedRows telemetryFailedRows) {
	currentByKey := buildTelemetryCurrentLookup(currentData)

	for start := 0; start < len(historyData); start += telemetryFallbackChunkSize {
		chunkHistory := historyData[start:min(start+telemetryFallbackChunkSize, len(historyData))]
		chunkCurrent := buildTelemetryCurrentChunk(chunkHistory, currentByKey)

		err := w.db.Transaction(func(tx *gorm.DB) error {
			return w.insertTelemetryBatch(tx, chunkHistory, chunkCurrent)
		})
		if err == nil {
			written += len(chunkHistory)
			continue
		}

		w.logTelemetryBatchFailure("fallback chunk insert failed, downgrade to single insert", len(chunkHistory), err, chunkHistory)
		written += w.insertSingleRows(chunkHistory, currentByKey, &failedRows)
	}

	return written, failedRows
}

// fallbackInsertSingleRows writes each row in its own transaction. Failures
// are summarized in one log line per call (one call per chunk) instead of
// one JSON-marshalled preview per row.
func (w *telemetryWriter) fallbackInsertSingleRows(
	historyData []TelemetryData,
	currentByKey map[telemetrySeriesKey]TelemetryCurrentData,
) (written, failed int) {
	var failedRows telemetryFailedRows
	written = w.insertSingleRows(historyData, currentByKey, &failedRows)
	return written, len(failedRows)
}

// insertSingleRows is fallbackInsertSingleRows recording failed identities.
func (w *telemetryWriter) insertSingleRows(
	historyData []TelemetryData,
	currentByKey map[telemetrySeriesKey]TelemetryCurrentData,
	failedRows *telemetryFailedRows,
) (written int) {
	failed := 0
	var (
		failedPreview []TelemetryData
		firstErr      error
	)
	for _, history := range historyData {
		current, hasCurrent := currentByKey[telemetryCurrentLookupKey(history.DeviceID, history.Key)]
		err := w.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(telemetryHistoryConflictClause()).Create(&history).Error; err != nil {
				return err
			}
			if !hasCurrent {
				return nil
			}
			// 插入最新值表
			return tx.Clauses(TelemetryCurrentUpsertClause()).Create(&current).Error
		})
		if err == nil {
			written++
			continue
		}

		failed++
		failedRows.add(history)
		if firstErr == nil {
			firstErr = err
		}
		if len(failedPreview) < telemetryFailurePreviewRows {
			failedPreview = append(failedPreview, history)
		}
		// 记录诊断：仅在单条插入真实失败时，增加 storage_failed 并记录失败详情到失败列表。
		diagnostics.GetInstance().RecordStorageFailed(history.DeviceID, fmt.Sprintf("存储失败：%v", err))
		if persistErr := w.persistFailedTelemetry(history, err); persistErr != nil && w.logger != nil {
			w.logger.Errorf(
				"telemetry durability fallback exhausted: ts=%d, err=%v (device/key omitted: device-controlled strings are kept out of logs per log-injection gate)",
				history.TS,
				persistErr,
			)
		}
	}
	if failed > 0 {
		w.logTelemetryBatchFailure("single insert failed", failed, firstErr, failedPreview)
	}
	return written
}
