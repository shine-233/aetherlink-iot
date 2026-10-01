// 文件用途：操作日志（审计）的异步批量写入器（marathon db-schema#11 收尾）。
//
// 背景：saveOperationLog 此前在每个请求的收尾路径上同步执行一次
// `query.OperationLog.Create`，即每个 API 请求都带一次 DB 往返。本文件把落库
// 挪到后台协程，请求路径只做一次非阻塞投递。
//
// 核心逻辑：
//   - 默认**关闭**（operation_log.async_enabled=false），保持既有同步行为不变，
//     由部署方显式开启——与 141.sql「客户数据默认关闭」、timescale_mode「默认 auto
//     保持原行为」同一惯例。审计日志的持久性语义变化应由部署方决定。
//   - 队列满时**回退同步写**，而不是丢弃：审计条目宁可慢一次，不能静默少一条。
//     只有真正写失败才计数并告警。
//   - 停止时先 drain 再退出，保证正常关停不丢条目。
//
// 关键注意事项：
//   - **硬崩溃（kill -9 / 掉电）会丢失尚在队列中的条目**，这是异步审计的固有代价。
//     需要"一条都不能丢"的部署请保持默认关闭。
//   - 本写入器是包级单例，Start 幂等；测试可用 resetOperationLogWriterForTest 复位。
package middleware

import (
	"sync"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// OperationLogWriterConfig 异步写入器配置。
type OperationLogWriterConfig struct {
	// Enabled 为 false 时 StartOperationLogWriter 不做任何事，saveOperationLog 走同步写。
	Enabled bool
	// QueueSize 队列容量；满时回退同步写。
	QueueSize int
	// BatchSize 单次批量落库的行数上限。
	BatchSize int
	// FlushInterval 定时刷新的间隔。
	FlushInterval time.Duration
}

// DefaultOperationLogWriterConfig 返回保守默认值：关闭、队列 4096、批 200、间隔 1s。
func DefaultOperationLogWriterConfig() OperationLogWriterConfig {
	return OperationLogWriterConfig{
		Enabled:       false,
		QueueSize:     4096,
		BatchSize:     200,
		FlushInterval: time.Second,
	}
}

type operationLogWriter struct {
	config OperationLogWriterConfig

	initOnce    sync.Once
	initialized bool

	ch       chan *model.OperationLog
	stopCh   chan struct{}
	doneCh   chan struct{}
	ticker   *time.Ticker
	stopOnce sync.Once

	// dropped 只统计"投递失败且同步回退也失败"的条目；正常回退不计数。
	dropped atomic.Uint64
	// queued 统计已成功投递到队列的条目（用于观测异步化比例）。
	queued atomic.Uint64
}

var (
	operationLogWriterMu     sync.Mutex
	operationLogWriterGlobal *operationLogWriter
)

// StartOperationLogWriter 启动异步写入器。幂等：重复调用直接返回。
// 未启用时返回 nil 且不启动任何协程。
func StartOperationLogWriter(config OperationLogWriterConfig) error {
	operationLogWriterMu.Lock()
	defer operationLogWriterMu.Unlock()

	if operationLogWriterGlobal != nil && operationLogWriterGlobal.initialized {
		return nil
	}
	if !config.Enabled {
		logrus.Info("operation log async writer disabled, keeping synchronous insert")
		return nil
	}

	if config.QueueSize <= 0 {
		config.QueueSize = DefaultOperationLogWriterConfig().QueueSize
	}
	if config.BatchSize <= 0 {
		config.BatchSize = DefaultOperationLogWriterConfig().BatchSize
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = DefaultOperationLogWriterConfig().FlushInterval
	}

	w := &operationLogWriter{
		config: config,
		ch:     make(chan *model.OperationLog, config.QueueSize),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	w.ticker = time.NewTicker(config.FlushInterval)
	w.initialized = true
	go w.run()

	operationLogWriterGlobal = w
	logrus.Infof(
		"operation log async writer started: queue=%d batch=%d interval=%v",
		config.QueueSize, config.BatchSize, config.FlushInterval,
	)
	return nil
}

// StopOperationLogWriter 停止写入器：先 drain 队列，再退出。超时后放弃剩余条目并告警。
// 幂等；未启动时直接返回。
func StopOperationLogWriter(timeout time.Duration) {
	operationLogWriterMu.Lock()
	w := operationLogWriterGlobal
	operationLogWriterMu.Unlock()

	if w == nil || !w.initialized {
		return
	}

	w.stopOnce.Do(func() {
		close(w.stopCh)
		if w.ticker != nil {
			w.ticker.Stop()
		}

		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		select {
		case <-w.doneCh:
			logrus.Infof(
				"operation log async writer stopped gracefully: queued=%d dropped=%d",
				w.queued.Load(), w.dropped.Load(),
			)
		case <-time.After(timeout):
			logrus.Warnf(
				"operation log async writer stop timeout, %d entries may be lost",
				len(w.ch),
			)
		}
	})
}

// run 后台循环：定时 flush，收到 stop 时把队列 drain 干净再退出。
//
// 必须带 recover：**后台 goroutine 里的 panic 会直接终止整个进程**（不像 HTTP
// 请求路径有 gin 的恢复中间件兜底）。审计写入失败绝不能拖垮服务。
func (w *operationLogWriter) run() {
	defer close(w.doneCh)
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("operation log async writer panicked, writer stopped: %v", r)
		}
	}()

	for {
		select {
		case <-w.stopCh:
			w.drain()
			return
		case <-w.ticker.C:
			w.flushBatch(w.config.BatchSize)
		}
	}
}

// drain 把队列里剩余条目全部落库。
func (w *operationLogWriter) drain() {
	for {
		select {
		case entry := <-w.ch:
			w.writeBatch([]*model.OperationLog{entry})
		default:
			return
		}
	}
}

// flushBatch 最多取 max 条落库（队列空时立即返回）。
func (w *operationLogWriter) flushBatch(max int) {
	batch := make([]*model.OperationLog, 0, max)
	for len(batch) < max {
		select {
		case entry := <-w.ch:
			batch = append(batch, entry)
		default:
			if len(batch) == 0 {
				return
			}
			w.writeBatch(batch)
			return
		}
	}
	if len(batch) > 0 {
		w.writeBatch(batch)
	}
}

// writeBatch 批量落库；失败时逐条重试一次，仍失败才计 dropped 并告警。
func (w *operationLogWriter) writeBatch(batch []*model.OperationLog) {
	if len(batch) == 0 {
		return
	}
	// 数据库尚未初始化（启动早期、或单测未装 DB）时不能解引用 query 单例——
	// 那会在后台协程里 nil panic。计数 + 告警即可，绝不 panic。
	if global.DB == nil {
		w.dropped.Add(uint64(len(batch)))
		logrus.Warnf("operation log batch dropped: database not initialized, rows=%d", len(batch))
		return
	}
	if err := query.OperationLog.CreateInBatches(batch, len(batch)); err == nil {
		return
	} else {
		logrus.Warnf("operation log batch insert failed, falling back to per-row: %v", err)
	}

	for _, entry := range batch {
		if err := query.OperationLog.Create(entry); err != nil {
			w.dropped.Add(1)
			logrus.Warnf("save operation log failed: %v", err)
		}
	}
}

// enqueueOperationLog 尝试把条目投递到异步队列。
// 返回 true 表示已接管（调用方不要再同步写）；false 表示未启用或队列已满，
// 调用方应回退同步写——**审计条目不因背压而丢弃**。
func enqueueOperationLog(entry *model.OperationLog) bool {
	operationLogWriterMu.Lock()
	w := operationLogWriterGlobal
	operationLogWriterMu.Unlock()

	if w == nil || !w.initialized {
		return false
	}

	select {
	case w.ch <- entry:
		w.queued.Add(1)
		return true
	default:
		// 队列满：交给调用方同步写，避免审计条目丢失。
		logrus.Warn("operation log queue full, falling back to synchronous insert")
		return false
	}
}

// OperationLogWriterStats 返回观测指标（queued/dropped/backlog）。
// 供诊断或测试使用。
func OperationLogWriterStats() (queued, dropped uint64, backlog int) {
	operationLogWriterMu.Lock()
	w := operationLogWriterGlobal
	operationLogWriterMu.Unlock()

	if w == nil || !w.initialized {
		return 0, 0, 0
	}
	return w.queued.Load(), w.dropped.Load(), len(w.ch)
}

// resetOperationLogWriterForTest 复位单例，仅供测试使用。
func resetOperationLogWriterForTest() {
	operationLogWriterMu.Lock()
	defer operationLogWriterMu.Unlock()
	operationLogWriterGlobal = nil
}
