// telemetry_writer.go writes telemetry records through storage workers.
//
// It batches or coordinates telemetry persistence and diagnostics. Changes
// affect ingestion throughput, data freshness, and chart/history correctness.
// 文件用途：提供遥测、属性或事件存储模块的 telemetry writer 能力。
// 核心逻辑：管理存储配置、消息模型、批量写入、去重、指标采集和直写通道，主要围绕 type telemetryWriter、type telemetryBatchItem、func newTelemetryWriter、func (w *telemetryWriter) start 等声明展开。
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。
// 重构建议：后续可将批处理策略、指标和数据库写入进一步解耦，便于压测和替换实现。

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/diagnostics"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// telemetryWriter 遥测数据批量写入器
type telemetryWriter struct {
	db      *gorm.DB
	logger  Logger
	config  Config
	metrics *metricsCollector
	spool   *telemetryFileSpool

	buffer   []*telemetryBatchItem // 批次缓冲区
	bufferMu sync.Mutex            // 缓冲区锁

	flushTicker *time.Ticker  // 定时刷新定时器
	spoolTicker *time.Ticker  // 独立文件spool重放定时器
	stopCh      chan struct{} // 停止信号
	doneCh      chan struct{} // 完成信号
	stopOnce    sync.Once
	doneOnce    sync.Once
	stopped     bool
}

// telemetryBatchItem 批次项
type telemetryBatchItem struct {
	deviceID           string               // 设备ID
	tenantID           string               // 租户ID
	timestamp          int64                // 时间戳（毫秒）
	points             []TelemetryDataPoint // 遥测数据点列表
	writeAheadPrepared bool

	// writeAhead 记录本项在入缓冲区前落盘的 receipt。只有确认主库写入成功后
	// 才删除，因此进程被强杀时这些文件仍留在 spool 里等待既有重放。
	writeAhead []telemetryWriteAheadReceipt
}

// telemetryWriteAheadReceipt 指向一条已落盘、等待主库确认的 spool 记录。
// 保存 history 本体：spool 的 identity 与删除路径都由它确定性派生，
// 与失败兜底、重放走同一套 (device_id,key,ts) 口径。
type telemetryWriteAheadReceipt struct {
	history TelemetryData
}

// newTelemetryWriter 创建遥测数据写入器
func newTelemetryWriter(db *gorm.DB, logger Logger, config Config, metrics *metricsCollector) *telemetryWriter {
	return &telemetryWriter{
		db:      db,
		logger:  logger,
		config:  config,
		metrics: metrics,
		spool:   newTelemetryFileSpool(config),
		buffer:  make([]*telemetryBatchItem, 0, config.TelemetryBatchSize),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// start 启动写入器
func (w *telemetryWriter) start(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			w.finish()
		}
	}()
	if w.config.TelemetryWriteAheadSpoolEnabled && w.spool == nil {
		return fmt.Errorf("telemetry write-ahead requires an enabled telemetry spool")
	}
	if w.spool != nil {
		if w.metrics != nil {
			w.metrics.setTelemetrySpoolCapacity(w.config.TelemetrySpoolMaxRecords, w.config.TelemetrySpoolMaxBytes)
		}
		if err := w.spool.init(); err != nil {
			return fmt.Errorf("initialize telemetry file spool: %w", err)
		}
		usage := w.spool.usage()
		if w.metrics != nil {
			w.metrics.setTelemetrySpoolUsage(usage)
		}
		if (usage.Records > w.config.TelemetrySpoolMaxRecords || usage.Bytes > w.config.TelemetrySpoolMaxBytes) && w.logger != nil {
			w.logger.Warnf(
				"telemetry file spool starts above capacity and will accept no new records until replay drains it: records=%d/%d bytes=%d/%d quarantine_records=%d quarantine_bytes=%d",
				usage.Records,
				w.config.TelemetrySpoolMaxRecords,
				usage.Bytes,
				w.config.TelemetrySpoolMaxBytes,
				usage.QuarantinedRecords,
				usage.QuarantinedBytes,
			)
		} else if usage.Records > 0 && w.logger != nil {
			w.logger.Warnf(
				"telemetry file spool recovered backlog: records=%d bytes=%d quarantine_records=%d quarantine_bytes=%d",
				usage.Records,
				usage.Bytes,
				usage.QuarantinedRecords,
				usage.QuarantinedBytes,
			)
		}
		if w.config.TelemetrySpoolReplayInterval <= 0 {
			return fmt.Errorf("telemetry spool replay interval must be positive")
		}
		if w.config.TelemetrySpoolReplayBatchSize < 1 {
			return fmt.Errorf("telemetry spool replay batch size must be positive")
		}
		if w.config.TelemetrySpoolReplayTimeout <= 0 {
			return fmt.Errorf("telemetry spool replay timeout must be positive")
		}
		w.spoolTicker = time.NewTicker(w.config.TelemetrySpoolReplayInterval)
	}
	flushDuration := w.config.GetFlushDuration()
	if flushDuration > 0 {
		w.flushTicker = time.NewTicker(flushDuration)
	}
	go w.run(ctx)
	return nil
}

// run 运行后台flush任务
func (w *telemetryWriter) run(ctx context.Context) {
	defer w.finish()
	var flushCh <-chan time.Time
	if w.flushTicker != nil {
		flushCh = w.flushTicker.C
	}
	var spoolCh <-chan time.Time
	if w.spoolTicker != nil {
		spoolCh = w.spoolTicker.C
		w.replayTelemetryFileSpool(ctx)
	}

	for {
		select {
		case <-w.stopCh:
			w.logger.Info("telemetry writer stopped")
			w.flushRemaining() // 停止前刷新剩余数据
			return
		case <-flushCh:
			w.flush() // 定时刷新
		case <-spoolCh:
			w.replayTelemetryFileSpool(ctx)
		}
	}
}

func (w *telemetryWriter) finish() {
	if w == nil {
		return
	}
	w.doneOnce.Do(func() { close(w.doneCh) })
}

// requestStop 发出停止信号；stop 额外等待写入器完成。
func (w *telemetryWriter) requestStop() {
	w.stopOnce.Do(func() {
		w.bufferMu.Lock()
		w.stopped = true
		w.bufferMu.Unlock()
		close(w.stopCh)
		if w.flushTicker != nil {
			w.flushTicker.Stop()
		}
		if w.spoolTicker != nil {
			w.spoolTicker.Stop()
		}
	})
}

func (w *telemetryWriter) stop(timeout time.Duration) error {
	w.requestStop()

	if !waitForStorageDone(w.doneCh, timeout) {
		w.logger.Warn("telemetry writer stop timeout")
		return fmt.Errorf("telemetry writer stop timeout")
	}
	w.logger.Info("telemetry writer stopped gracefully")
	return nil
}

// write 写入遥测消息
func (w *telemetryWriter) write(msg *Message) error {
	item, err := telemetryBatchItemFromMessage(msg)
	if err != nil {
		return err
	}

	// 崩溃窗口保护：内存缓冲区在 SIGKILL/断电时不留任何痕迹，因此在入队前
	// 先把本批点写成 spool receipt。flush 成功后再删除；进程异常退出时由
	// 既有 spool 重放补回。生产默认开启；需要接受该崩溃窗口时才显式关闭。
	var receiptErr error
	item.writeAhead, receiptErr = w.storeWriteAheadReceipts(item)
	if receiptErr != nil {
		if persistErr := w.persistRejectedTelemetry(context.Background(), msg, receiptErr); persistErr != nil {
			return errors.Join(receiptErr, fmt.Errorf("persist telemetry write-ahead fallback: %w", persistErr))
		}
		return receiptErr
	}

	// 加入缓冲区，检查是否需要刷新
	w.bufferMu.Lock()
	if w.stopped {
		w.bufferMu.Unlock()
		return fmt.Errorf("telemetry writer is stopped")
	}
	w.buffer = append(w.buffer, item)
	shouldFlush := len(w.buffer) >= w.config.TelemetryBatchSize
	w.bufferMu.Unlock()

	// 如果缓冲区满了，立即刷新
	if shouldFlush {
		w.flush()
	}

	return nil
}

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
	historyData, _, _ := w.deduplicateAndConvert([]*telemetryBatchItem{item})
	for _, history := range historyData {
		if _, err := w.spool.store(history, time.Now()); err != nil {
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
	historyData, _, _ := w.deduplicateAndConvert([]*telemetryBatchItem{item})

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
	historyData, _, _ := w.deduplicateAndConvert([]*telemetryBatchItem{item})
	if len(historyData) == 0 {
		return nil, nil
	}
	now := time.Now()
	receipts := make([]telemetryWriteAheadReceipt, 0, len(historyData))
	for _, history := range historyData {
		result, err := w.spool.store(history, now)
		if err != nil {
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
			if err := w.spool.removeWriteAheadReceipt(receipt.history); err != nil {
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

func (w *telemetryWriter) refreshTelemetrySpoolMetrics() {
	if w == nil || w.spool == nil || w.metrics == nil {
		return
	}
	w.metrics.setTelemetrySpoolUsage(w.spool.usage())
}

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
	storeResult, err := w.spool.store(history, now)
	usage := w.spool.usage()
	if w.metrics != nil {
		w.metrics.addTelemetrySpoolCorrupt(int64(storeResult.Corrupt))
		w.metrics.setTelemetrySpoolUsage(usage)
	}
	if storeResult.Corrupt > 0 && w.logger != nil {
		w.logger.Warnf(
			"telemetry file spool detected corrupt deterministic record before replacement: device_id=%s key=%s ts=%d detected=%d quarantined=%d backlog=%d bytes=%d quarantine_records=%d quarantine_bytes=%d",
			history.DeviceID,
			history.Key,
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
				"telemetry saved to independent file spool after PostgreSQL dead-letter failure: device_id=%s, key=%s, ts=%d backlog=%d bytes=%d quarantine_records=%d quarantine_bytes=%d",
				history.DeviceID,
				history.Key,
				history.TS,
				usage.Records,
				usage.Bytes,
				usage.QuarantinedRecords,
				usage.QuarantinedBytes,
			)
		}
	} else if storeResult.Duplicate && w.logger != nil {
		w.logger.Debugf(
			"telemetry file spool already contains durable record: device_id=%s, key=%s, ts=%d backlog=%d bytes=%d",
			history.DeviceID,
			history.Key,
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
			w.logger.Errorf("telemetry dead-letter insert failed: device_id=%s, key=%s, err=%v", history.DeviceID, history.Key, insert.Error)
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
			"telemetry dead-letter already contains durable record: device_id=%s, key=%s, ts=%d",
			history.DeviceID,
			history.Key,
			history.TS,
		)
	}
	return nil
}

func (w *telemetryWriter) replayTelemetryFileSpool(parent context.Context) {
	if w == nil || w.spool == nil {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, w.config.TelemetrySpoolReplayTimeout)
	defer cancel()
	result, err := w.spool.replay(
		ctx,
		w.config.TelemetrySpoolReplayBatchSize,
		w.replayTelemetryFileSpoolRow,
	)
	if w.metrics != nil {
		w.metrics.addTelemetrySpoolReplayed(int64(result.Replayed))
		w.metrics.addTelemetrySpoolCorrupt(int64(result.Corrupt))
		w.metrics.setTelemetrySpoolUsage(result.Usage)
	}
	if result.Replayed > 0 && w.logger != nil {
		w.logger.Infof(
			"telemetry file spool replayed: replayed=%d attempted=%d backlog=%d bytes=%d quarantine_records=%d quarantine_bytes=%d",
			result.Replayed,
			result.Attempted,
			result.Usage.Records,
			result.Usage.Bytes,
			result.Usage.QuarantinedRecords,
			result.Usage.QuarantinedBytes,
		)
	}
	if err != nil && w.logger != nil {
		w.logger.Warnf(
			"telemetry file spool replay incomplete: attempted=%d replayed=%d corrupt=%d backlog=%d quarantine_records=%d quarantine_bytes=%d err=%v",
			result.Attempted,
			result.Replayed,
			result.Corrupt,
			result.Usage.Records,
			result.Usage.QuarantinedRecords,
			result.Usage.QuarantinedBytes,
			err,
		)
	}
}

func (w *telemetryWriter) replayTelemetryFileSpoolRow(ctx context.Context, history TelemetryData) error {
	if w == nil || w.db == nil {
		return fmt.Errorf("telemetry database is unavailable")
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}, {Name: "key"}, {Name: "ts"}},
			DoNothing: true,
		}).Create(&history)
		if insert.Error != nil {
			return fmt.Errorf("insert replayed history data failed: %w", insert.Error)
		}

		authoritative := history
		if insert.RowsAffected == 0 {
			// Another path already persisted this unique history identity. Read
			// that first-writer-wins value before touching current data; otherwise
			// an equal-timestamp spool replay could leave history=B/current=A.
			if err := tx.Where(
				"device_id = ? AND key = ? AND ts = ?",
				history.DeviceID,
				history.Key,
				history.TS,
			).Take(&authoritative).Error; err != nil {
				return fmt.Errorf("load authoritative replay history data failed: %w", err)
			}
		}

		current := telemetryCurrentFromHistory(authoritative)
		if err := tx.Clauses(TelemetryCurrentUpsertClause()).Create(&current).Error; err != nil {
			return fmt.Errorf("upsert replayed current data failed: %w", err)
		}
		return nil
	})
}

func telemetryCurrentFromHistory(history TelemetryData) TelemetryCurrentData {
	return TelemetryCurrentData{
		DeviceID: history.DeviceID,
		Key:      history.Key,
		TS:       time.UnixMilli(history.TS),
		BoolV:    history.BoolV,
		NumberV:  history.NumberV,
		StringV:  history.StringV,
		TenantID: history.TenantID,
	}
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
