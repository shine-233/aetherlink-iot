// telemetry_writer.go owns the telemetry writer lifecycle and buffer admission.
//
// Pipeline layering:
//   - write (producer, storage input goroutine): convert once, write-ahead
//     receipt, append under bufferMu, signal the flusher. Never touches the DB.
//   - run (dedicated flusher goroutine): the only caller of doFlush, so batch
//     transactions are strictly serialized.
//   - replayLoop (own goroutine): file spool replay, so a slow replay pass can
//     not stall buffer drainage.
//
// Memory stays bounded: once the buffer reaches telemetryHighWaterFactor x
// TelemetryBatchSize items, write blocks until the flusher drains it.
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。

package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// telemetryHighWaterFactor bounds the buffer at this many batches.
const telemetryHighWaterFactor = 4

// telemetryWriter 遥测数据批量写入器
type telemetryWriter struct {
	db      *gorm.DB
	logger  Logger
	config  Config
	metrics *metricsCollector
	spool   *telemetryFileSpool

	buffer   []*telemetryBatchItem // 批次缓冲区
	bufferMu sync.Mutex            // 缓冲区锁
	// drained is signalled (under bufferMu) whenever the flusher takes the
	// buffer or the writer stops, waking producers blocked at the high-water mark.
	drained *sync.Cond
	// running is true while the flusher goroutine owns flushing. Before start
	// (and in unit tests that never start it) write flushes inline instead.
	running bool
	stopped bool

	// flushReq carries "buffer reached a batch" hints to the flusher (cap 1).
	flushReq chan struct{}
	// flushMu serializes doFlush even for the inline pre-start path.
	flushMu sync.Mutex
	// flushHook is a test seam observing each doFlush entry/exit.
	flushHook func(entering bool)

	flushTicker *time.Ticker  // 定时刷新定时器
	spoolTicker *time.Ticker  // 独立文件spool重放定时器
	stopCh      chan struct{} // 停止信号
	doneCh      chan struct{} // 完成信号
	replayWG    sync.WaitGroup
	stopOnce    sync.Once
	doneOnce    sync.Once
}

// telemetryBatchItem 批次项
type telemetryBatchItem struct {
	deviceID           string               // 设备ID
	tenantID           string               // 租户ID
	timestamp          int64                // 时间戳（毫秒）
	points             []TelemetryDataPoint // 原始点；转换后释放
	writeAheadPrepared bool

	// converted rows are produced exactly once by convertedRows and reused by
	// write-ahead, rejection persistence and flush.
	converted  bool
	rows       []TelemetryData
	duplicates int

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
	w := &telemetryWriter{
		db:       db,
		logger:   logger,
		config:   config,
		metrics:  metrics,
		spool:    newTelemetryFileSpool(config),
		buffer:   make([]*telemetryBatchItem, 0, max(config.TelemetryBatchSize, 0)),
		flushReq: make(chan struct{}, 1),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	w.drained = sync.NewCond(&w.bufferMu)
	return w
}

func (w *telemetryWriter) batchSize() int {
	return max(w.config.TelemetryBatchSize, 1)
}

func (w *telemetryWriter) highWaterMark() int {
	return w.batchSize() * telemetryHighWaterFactor
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

	w.bufferMu.Lock()
	if w.drained == nil {
		w.drained = sync.NewCond(&w.bufferMu)
	}
	if w.flushReq == nil {
		w.flushReq = make(chan struct{}, 1)
	}
	if w.stopped {
		w.bufferMu.Unlock()
		return fmt.Errorf("telemetry writer is stopped")
	}
	w.running = true
	w.bufferMu.Unlock()

	if w.spoolTicker != nil {
		w.replayWG.Add(1)
		go w.replayLoop(ctx, w.spoolTicker.C)
	}
	go w.run()
	return nil
}

// run is the dedicated flusher. It is the only goroutine calling doFlush
// while the writer is running.
func (w *telemetryWriter) run() {
	defer w.finish()
	var flushCh <-chan time.Time
	if w.flushTicker != nil {
		flushCh = w.flushTicker.C
	}

	for {
		select {
		case <-w.stopCh:
			if w.logger != nil {
				w.logger.Info("telemetry writer stopped")
			}
			w.flushRemaining() // 停止前刷新剩余数据
			// Replay shares the database; wait for its in-flight pass so stop
			// really means "no telemetry goroutine is touching storage".
			w.replayWG.Wait()
			return
		case <-w.flushReq:
			w.flush()
		case <-flushCh:
			w.flush() // 定时刷新
		}
	}
}

// replayLoop drives file spool replay independently of flushing.
func (w *telemetryWriter) replayLoop(ctx context.Context, tick <-chan time.Time) {
	defer w.replayWG.Done()
	w.replayTelemetryFileSpool(ctx)
	for {
		select {
		case <-w.stopCh:
			return
		case <-tick:
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
		if w.drained != nil {
			w.drained.Broadcast()
		}
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
		if w.logger != nil {
			w.logger.Warn("telemetry writer stop timeout")
		}
		return fmt.Errorf("telemetry writer stop timeout")
	}
	if w.logger != nil {
		w.logger.Info("telemetry writer stopped gracefully")
	}
	return nil
}

// write 写入遥测消息。It converts and write-ahead-persists the item, admits
// it to the buffer and signals the flusher; it never runs a DB transaction
// while the flusher is running.
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
		if persistErr := w.persistRejectedItem(context.Background(), item, receiptErr); persistErr != nil {
			return errors.Join(receiptErr, fmt.Errorf("persist telemetry write-ahead fallback: %w", persistErr))
		}
		return receiptErr
	}

	w.bufferMu.Lock()
	// Backpressure: bounded memory. Only meaningful while a flusher exists to
	// drain the buffer; requestStop broadcasts so waiters never hang.
	for w.running && !w.stopped && len(w.buffer) >= w.highWaterMark() {
		w.drained.Wait()
	}
	if w.stopped {
		w.bufferMu.Unlock()
		// Any write-ahead receipt stays on disk and is recovered by replay.
		return fmt.Errorf("telemetry writer is stopped")
	}
	w.buffer = append(w.buffer, item)
	reachedBatch := len(w.buffer) >= w.batchSize()
	running := w.running
	w.bufferMu.Unlock()

	if !reachedBatch {
		return nil
	}
	if !running {
		// No flusher yet (writer constructed but not started): keep the
		// historical synchronous behaviour. flushMu still serializes doFlush.
		w.flush()
		return nil
	}
	select {
	case w.flushReq <- struct{}{}:
	default: // a flush request is already pending
	}
	return nil
}

func (w *telemetryWriter) refreshTelemetrySpoolMetrics() {
	if w == nil || w.spool == nil || w.metrics == nil {
		return
	}
	w.metrics.setTelemetrySpoolUsage(w.spool.usage())
}
