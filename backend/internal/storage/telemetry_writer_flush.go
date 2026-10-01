// telemetry_writer_flush.go owns the batch flush pipeline: buffer hand-off,
// the primary batch transaction and the chunk/single-row fallbacks.

package storage

import (
	"aetherlink-iot/backend/internal/diagnostics"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// flush 刷新缓冲区
func (w *telemetryWriter) flush() {
	w.bufferMu.Lock()
	if len(w.buffer) == 0 {
		w.bufferMu.Unlock()
		return
	}

	// 取出当前批次，创建新缓冲区
	batch := w.buffer
	w.buffer = make([]*telemetryBatchItem, 0, w.config.TelemetryBatchSize)
	w.bufferMu.Unlock()

	w.doFlush(batch)
}

// flushRemaining 刷新剩余数据（停止时调用）
func (w *telemetryWriter) flushRemaining() {
	w.bufferMu.Lock()
	batch := w.buffer
	w.buffer = nil
	w.bufferMu.Unlock()

	if len(batch) > 0 {
		w.logger.Infof("flushing remaining %d telemetry items", len(batch))
		w.doFlush(batch)
	}
}

// doFlush 执行实际的刷新操作
func (w *telemetryWriter) doFlush(batch []*telemetryBatchItem) {
	// 1. 批次内去重并转换为数据库模型
	historyData, currentData, duplicates := w.deduplicateAndConvert(batch)

	// 记录批次内重复数
	if duplicates > 0 {
		w.metrics.addTelemetryDuplicates(int64(duplicates))
	}

	if len(historyData) == 0 {
		return
	}

	// 2. 批量写入数据库
	written, failed := w.batchInsert(historyData, currentData)

	// 2b. 只有主库确认成功才释放 write-ahead receipt。失败时保留，交给既有
	// spool 重放；主写失败的行另有 dead-letter/spool 路径，重复是幂等的。
	if failed == 0 {
		w.releaseWriteAheadReceipts(batch)
	}

	// 3. 记录监控指标
	w.metrics.addTelemetryWritten(int64(written))
	w.metrics.addTelemetryFailed(int64(failed))
	w.metrics.recordTelemetryBatch(len(historyData))

	w.logger.Debugf("【设备诊断】flushed batch: total=%d, written=%d, failed=%d, duplicates=%d",
		len(historyData), written, failed, duplicates)
}

// batchInsert 批量插入数据库
const telemetryFallbackChunkSize = 100

func (w *telemetryWriter) batchInsert(historyData []TelemetryData, currentData []TelemetryCurrentData) (written, failed int) {
	err := w.db.Transaction(func(tx *gorm.DB) error {
		return w.insertTelemetryBatch(tx, historyData, currentData)
	})

	if err != nil {
		w.logTelemetryBatchFailure("batch insert failed", len(historyData), err, historyData)
		return w.fallbackInsert(historyData, currentData)
	}

	return len(historyData), 0
}

func (w *telemetryWriter) insertTelemetryBatch(tx *gorm.DB, historyData []TelemetryData, currentData []TelemetryCurrentData) error {
	if len(historyData) > 0 {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}, {Name: "key"}, {Name: "ts"}},
			DoNothing: true,
		}).Create(&historyData).Error; err != nil {
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

func (w *telemetryWriter) logTelemetryBatchFailure(prefix string, total int, err error, historyData []TelemetryData) {
	previewRows := telemetryHistoryPreviewRows(historyData, 5)
	if j, jerr := json.Marshal(previewRows); jerr == nil {
		w.logger.Errorf("%s: total=%d, err=%v, preview=%s", prefix, total, err, string(j))
		return
	}
	w.logger.Errorf("%s: total=%d, err=%v", prefix, total, err)
}

// fallbackInsert 逐条插入兜底（批量失败时使用）
func (w *telemetryWriter) fallbackInsert(historyData []TelemetryData, currentData []TelemetryCurrentData) (written, failed int) {
	currentByKey := buildTelemetryCurrentLookup(currentData)

	for start := 0; start < len(historyData); start += telemetryFallbackChunkSize {
		end := start + telemetryFallbackChunkSize
		if end > len(historyData) {
			end = len(historyData)
		}
		chunkHistory := historyData[start:end]
		chunkCurrent := buildTelemetryCurrentChunk(chunkHistory, currentByKey)

		err := w.db.Transaction(func(tx *gorm.DB) error {
			return w.insertTelemetryBatch(tx, chunkHistory, chunkCurrent)
		})
		if err == nil {
			written += len(chunkHistory)
			continue
		}

		w.logTelemetryBatchFailure("fallback chunk insert failed, downgrade to single insert", len(chunkHistory), err, chunkHistory)
		chunkWritten, chunkFailed := w.fallbackInsertSingleRows(chunkHistory, currentByKey)
		written += chunkWritten
		failed += chunkFailed
	}

	return written, failed
}

func (w *telemetryWriter) fallbackInsertSingleRows(
	historyData []TelemetryData,
	currentByKey map[string]TelemetryCurrentData,
) (written, failed int) {
	for _, history := range historyData {
		current, hasCurrent := currentByKey[telemetryCurrentLookupKey(history.DeviceID, history.Key)]
		err := w.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "device_id"}, {Name: "key"}, {Name: "ts"}},
				DoNothing: true,
			}).Create(&history).Error; err != nil {
				return err
			}

			if !hasCurrent {
				return nil
			}

			// 插入最新值表
			if err := tx.Clauses(TelemetryCurrentUpsertClause()).Create(&current).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			previewRow := map[string]interface{}{
				"device_id": history.DeviceID,
				"key":       history.Key,
				"ts":        history.TS,
				"tenant_id": history.TenantID,
			}
			if j, jerr := json.Marshal(previewRow); jerr == nil {
				w.logger.Errorf("single insert failed: preview=%s, err=%v", string(j), err)
			} else {
				w.logger.Errorf("single insert failed: device_id=%s, key=%s, err=%v", history.DeviceID, history.Key, err)
			}

			// 记录诊断：仅在单条插入真实失败时，增加 storage_failed 并记录失败详情到失败列表。
			diagnostics.GetInstance().RecordStorageFailed(history.DeviceID, fmt.Sprintf("存储失败：%v", err))
			if persistErr := w.persistFailedTelemetry(history, err); persistErr != nil && w.logger != nil {
				w.logger.Errorf(
					"telemetry durability fallback exhausted: device_id=%s, key=%s, ts=%d, err=%v",
					history.DeviceID,
					history.Key,
					history.TS,
					persistErr,
				)
			}
			failed++
		} else {
			written++
		}
	}

	return written, failed
}
