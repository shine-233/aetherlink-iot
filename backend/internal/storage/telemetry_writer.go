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
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
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

func (w *telemetryWriter) refreshTelemetrySpoolMetrics() {
	if w == nil || w.spool == nil || w.metrics == nil {
		return
	}
	w.metrics.setTelemetrySpoolUsage(w.spool.usage())
}
