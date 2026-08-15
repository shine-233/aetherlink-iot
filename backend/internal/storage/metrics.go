// 文件用途：提供遥测、属性或事件存储模块的 metrics 能力。
// 核心逻辑：管理存储配置、消息模型、批量写入、去重、指标采集和直写通道，主要围绕 type Metrics、type metricsCollector、func newMetricsCollector、func (m *metricsCollector) incTelemetryReceived 等声明展开。
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。
// 重构建议：后续可将批处理策略、指标和数据库写入进一步解耦，便于压测和替换实现。

package storage

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	telemetrySpooledPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spooled_total",
		Help:      "Telemetry points persisted to the independent file spool after PostgreSQL fallbacks failed.",
	})
	telemetrySpoolReplayedPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_replayed_total",
		Help:      "Telemetry points successfully replayed from the independent file spool.",
	})
	telemetrySpoolFailedPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_failures_total",
		Help:      "Telemetry file spool persistence failures, including capacity and filesystem errors.",
	})
	telemetrySpoolCorruptPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_corrupt_total",
		Help:      "Telemetry spool records detected as corrupt during integrity validation or replacement.",
	})
	telemetrySpoolBacklogPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_backlog_records",
		Help:      "Current telemetry file spool records, including quarantined records that consume capacity.",
	})
	telemetrySpoolBytesPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_bytes",
		Help:      "Current logical bytes consumed by telemetry file spool and quarantine records.",
	})
	telemetrySpoolQuarantineRecordsPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_quarantine_records",
		Help:      "Current telemetry spool records retained in quarantine and unavailable for automatic replay.",
	})
	telemetrySpoolQuarantineBytesPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_quarantine_bytes",
		Help:      "Current logical bytes retained by telemetry spool quarantine records.",
	})
	telemetrySpoolCapacityRecordsPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_capacity_records",
		Help:      "Configured maximum number of telemetry spool records.",
	})
	telemetrySpoolCapacityBytesPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "telemetry_spool_capacity_bytes",
		Help:      "Configured maximum logical bytes retained by the telemetry spool.",
	})
	attributeEventSpooledPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_durable_fallback_total",
		Help:      "Attribute/event envelopes durably persisted to the independent file spool after a PostgreSQL failure.",
	})
	attributeEventSpoolReplayedPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_replayed_total",
		Help:      "Attribute/event envelopes successfully replayed from the independent file spool.",
	})
	attributeEventSpoolFailedPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_failures_total",
		Help:      "Attribute/event file-spool persistence failures, including capacity and filesystem errors.",
	})
	attributeEventSpoolCorruptPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_corrupt_total",
		Help:      "Attribute/event spool records detected as corrupt during integrity validation.",
	})
	attributeEventSpoolBacklogPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_backlog_records",
		Help:      "Current attribute/event spool records, including quarantined records that consume capacity.",
	})
	attributeEventSpoolBytesPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_bytes",
		Help:      "Current bytes consumed by attribute/event spool and quarantine records.",
	})
	attributeEventSpoolQuarantineRecordsPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_quarantine_records",
		Help:      "Current quarantined attribute/event spool records unavailable for automatic replay.",
	})
	attributeEventSpoolQuarantineBytesPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_quarantine_bytes",
		Help:      "Current bytes retained by the attribute/event spool quarantine.",
	})
	attributeEventSpoolCapacityRecordsPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_capacity_records",
		Help:      "Configured maximum number of attribute/event spool records.",
	})
	attributeEventSpoolCapacityBytesPrometheus = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_spool_capacity_bytes",
		Help:      "Configured maximum bytes retained by the attribute/event spool.",
	})
	attributeEventDeadLetteredPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_dead_letter_durable_fallback_total",
		Help:      "Attribute/event envelopes durably retained in the PostgreSQL dead-letter table after primary-write failure.",
	})
	attributeEventDeadLetterReplayedPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_dead_letter_replayed_total",
		Help:      "Attribute/event PostgreSQL dead-letter envelopes replayed into their primary tables.",
	})
	attributeEventDeadLetterFailedPrometheus = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "AetherLinkIoT",
		Subsystem: "storage",
		Name:      "attribute_event_dead_letter_failures_total",
		Help:      "Failures to retain an attribute/event envelope in the PostgreSQL dead-letter table.",
	})
)

// Metrics 监控指标
type Metrics struct {
	// 遥测数据
	TelemetryReceived               int64     // 接收的遥测消息数
	TelemetryWritten                int64     // 成功写入的遥测数据点数
	TelemetryFailed                 int64     // 写入失败的遥测数据点数
	TelemetryDuplicatesInBatch      int64     // 批次内重复数
	TelemetryBatchCount             int64     // flush次数
	TelemetryAvgBatch               float64   // 平均批次大小
	TelemetryLastFlush              time.Time // 最后flush时间
	TelemetrySpooled                int64     // 主库与数据库死信均失败后成功写入本地spool的数据点数
	TelemetrySpoolReplayed          int64     // 从本地spool成功重放并清理的数据点数
	TelemetrySpoolFailed            int64     // 本地spool写入失败的数据点数
	TelemetrySpoolCorrupt           int64     // 完整性校验发现损坏的spool记录数；隔离是否成功由quarantine gauges反映
	TelemetrySpoolBacklog           int64     // 当前本地spool记录数
	TelemetrySpoolBytes             int64     // 当前本地spool逻辑JSON字节数
	TelemetrySpoolQuarantineRecords int64     // 当前隔离且不可自动重放的spool记录数
	TelemetrySpoolQuarantineBytes   int64     // 当前隔离记录占用的逻辑字节数

	// 属性/事件 durable envelope spool（按完整 envelope 计数）
	AttributeEventSpooled                int64
	AttributeEventSpoolReplayed          int64
	AttributeEventSpoolFailed            int64
	AttributeEventSpoolCorrupt           int64
	AttributeEventSpoolBacklog           int64
	AttributeEventSpoolBytes             int64
	AttributeEventSpoolQuarantineRecords int64
	AttributeEventSpoolQuarantineBytes   int64
	AttributeEventDeadLettered           int64
	AttributeEventDeadLetterReplayed     int64
	AttributeEventDeadLetterFailed       int64

	// 属性数据
	AttributeWritten int64 // 成功写入的属性数
	AttributeFailed  int64 // 写入失败的属性数

	// 事件数据
	EventWritten int64 // 成功写入的事件数
	EventFailed  int64 // 写入失败的事件数
}

// metricsCollector 内部指标收集器
type metricsCollector struct {
	prometheusEnabled bool

	telemetryReceived                    int64
	telemetryWritten                     int64
	telemetryFailed                      int64
	telemetryDuplicatesInBatch           int64
	telemetryBatchCount                  int64
	telemetryTotalBatchSize              int64
	telemetryLastFlush                   int64 // Unix nano
	telemetrySpooled                     int64
	telemetrySpoolReplayed               int64
	telemetrySpoolFailed                 int64
	telemetrySpoolCorrupt                int64
	telemetrySpoolBacklog                int64
	telemetrySpoolBytes                  int64
	telemetrySpoolQuarantineRecords      int64
	telemetrySpoolQuarantineBytes        int64
	attributeEventSpooled                int64
	attributeEventSpoolReplayed          int64
	attributeEventSpoolFailed            int64
	attributeEventSpoolCorrupt           int64
	attributeEventSpoolBacklog           int64
	attributeEventSpoolBytes             int64
	attributeEventSpoolQuarantineRecords int64
	attributeEventSpoolQuarantineBytes   int64
	attributeEventDeadLettered           int64
	attributeEventDeadLetterReplayed     int64
	attributeEventDeadLetterFailed       int64

	attributeWritten int64
	attributeFailed  int64

	eventWritten int64
	eventFailed  int64
}

func newMetricsCollector(prometheusEnabled ...bool) *metricsCollector {
	enabled := true
	if len(prometheusEnabled) > 0 {
		enabled = prometheusEnabled[0]
	}
	return &metricsCollector{prometheusEnabled: enabled}
}

// 遥测数据指标

func (m *metricsCollector) incTelemetryReceived() {
	atomic.AddInt64(&m.telemetryReceived, 1)
}

func (m *metricsCollector) addTelemetryWritten(count int64) {
	atomic.AddInt64(&m.telemetryWritten, count)
}

func (m *metricsCollector) addTelemetryFailed(count int64) {
	atomic.AddInt64(&m.telemetryFailed, count)
}

func (m *metricsCollector) addTelemetryDuplicates(count int64) {
	atomic.AddInt64(&m.telemetryDuplicatesInBatch, count)
}

func (m *metricsCollector) recordTelemetryBatch(batchSize int) {
	atomic.AddInt64(&m.telemetryBatchCount, 1)
	atomic.AddInt64(&m.telemetryTotalBatchSize, int64(batchSize))
	atomic.StoreInt64(&m.telemetryLastFlush, time.Now().UnixNano())
}

func (m *metricsCollector) incTelemetrySpooled() {
	atomic.AddInt64(&m.telemetrySpooled, 1)
	if m.prometheusEnabled {
		telemetrySpooledPrometheus.Inc()
	}
}

func (m *metricsCollector) addTelemetrySpoolReplayed(count int64) {
	atomic.AddInt64(&m.telemetrySpoolReplayed, count)
	if m.prometheusEnabled && count > 0 {
		telemetrySpoolReplayedPrometheus.Add(float64(count))
	}
}

func (m *metricsCollector) incTelemetrySpoolFailed() {
	atomic.AddInt64(&m.telemetrySpoolFailed, 1)
	if m.prometheusEnabled {
		telemetrySpoolFailedPrometheus.Inc()
	}
}

func (m *metricsCollector) addTelemetrySpoolCorrupt(count int64) {
	atomic.AddInt64(&m.telemetrySpoolCorrupt, count)
	if m.prometheusEnabled && count > 0 {
		telemetrySpoolCorruptPrometheus.Add(float64(count))
	}
}

func (m *metricsCollector) setTelemetrySpoolUsage(usage telemetryFileSpoolUsage) {
	atomic.StoreInt64(&m.telemetrySpoolBacklog, int64(usage.Records))
	atomic.StoreInt64(&m.telemetrySpoolBytes, usage.Bytes)
	atomic.StoreInt64(&m.telemetrySpoolQuarantineRecords, int64(usage.QuarantinedRecords))
	atomic.StoreInt64(&m.telemetrySpoolQuarantineBytes, usage.QuarantinedBytes)
	if m.prometheusEnabled {
		telemetrySpoolBacklogPrometheus.Set(float64(usage.Records))
		telemetrySpoolBytesPrometheus.Set(float64(usage.Bytes))
		telemetrySpoolQuarantineRecordsPrometheus.Set(float64(usage.QuarantinedRecords))
		telemetrySpoolQuarantineBytesPrometheus.Set(float64(usage.QuarantinedBytes))
	}
}

func (m *metricsCollector) setTelemetrySpoolCapacity(maxRecords int, maxBytes int64) {
	if !m.prometheusEnabled {
		return
	}
	telemetrySpoolCapacityRecordsPrometheus.Set(float64(maxRecords))
	telemetrySpoolCapacityBytesPrometheus.Set(float64(maxBytes))
}

func (m *metricsCollector) incAttributeEventSpooled() {
	atomic.AddInt64(&m.attributeEventSpooled, 1)
	if m.prometheusEnabled {
		attributeEventSpooledPrometheus.Inc()
	}
}

func (m *metricsCollector) addAttributeEventSpoolReplayed(count int64) {
	atomic.AddInt64(&m.attributeEventSpoolReplayed, count)
	if m.prometheusEnabled && count > 0 {
		attributeEventSpoolReplayedPrometheus.Add(float64(count))
	}
}

func (m *metricsCollector) incAttributeEventSpoolFailed() {
	atomic.AddInt64(&m.attributeEventSpoolFailed, 1)
	if m.prometheusEnabled {
		attributeEventSpoolFailedPrometheus.Inc()
	}
}

func (m *metricsCollector) addAttributeEventSpoolCorrupt(count int64) {
	atomic.AddInt64(&m.attributeEventSpoolCorrupt, count)
	if m.prometheusEnabled && count > 0 {
		attributeEventSpoolCorruptPrometheus.Add(float64(count))
	}
}

func (m *metricsCollector) setAttributeEventSpoolUsage(usage attributeEventFileSpoolUsage) {
	atomic.StoreInt64(&m.attributeEventSpoolBacklog, int64(usage.Records))
	atomic.StoreInt64(&m.attributeEventSpoolBytes, usage.Bytes)
	atomic.StoreInt64(&m.attributeEventSpoolQuarantineRecords, int64(usage.QuarantinedRecords))
	atomic.StoreInt64(&m.attributeEventSpoolQuarantineBytes, usage.QuarantinedBytes)
	if m.prometheusEnabled {
		attributeEventSpoolBacklogPrometheus.Set(float64(usage.Records))
		attributeEventSpoolBytesPrometheus.Set(float64(usage.Bytes))
		attributeEventSpoolQuarantineRecordsPrometheus.Set(float64(usage.QuarantinedRecords))
		attributeEventSpoolQuarantineBytesPrometheus.Set(float64(usage.QuarantinedBytes))
	}
}

func (m *metricsCollector) setAttributeEventSpoolCapacity(maxRecords int, maxBytes int64) {
	if !m.prometheusEnabled {
		return
	}
	attributeEventSpoolCapacityRecordsPrometheus.Set(float64(maxRecords))
	attributeEventSpoolCapacityBytesPrometheus.Set(float64(maxBytes))
}

func (m *metricsCollector) incAttributeEventDeadLettered() {
	atomic.AddInt64(&m.attributeEventDeadLettered, 1)
	if m.prometheusEnabled {
		attributeEventDeadLetteredPrometheus.Inc()
	}
}

func (m *metricsCollector) incAttributeEventDeadLetterReplayed() {
	atomic.AddInt64(&m.attributeEventDeadLetterReplayed, 1)
	if m.prometheusEnabled {
		attributeEventDeadLetterReplayedPrometheus.Inc()
	}
}

func (m *metricsCollector) incAttributeEventDeadLetterFailed() {
	atomic.AddInt64(&m.attributeEventDeadLetterFailed, 1)
	if m.prometheusEnabled {
		attributeEventDeadLetterFailedPrometheus.Inc()
	}
}

// 属性数据指标

func (m *metricsCollector) incAttributeWritten() {
	atomic.AddInt64(&m.attributeWritten, 1)
}

func (m *metricsCollector) addAttributeWritten(count int64) {
	atomic.AddInt64(&m.attributeWritten, count)
}

func (m *metricsCollector) incAttributeFailed() {
	atomic.AddInt64(&m.attributeFailed, 1)
}

func (m *metricsCollector) addAttributeFailed(count int64) {
	atomic.AddInt64(&m.attributeFailed, count)
}

// 事件数据指标

func (m *metricsCollector) incEventWritten() {
	atomic.AddInt64(&m.eventWritten, 1)
}

func (m *metricsCollector) addEventWritten(count int64) {
	atomic.AddInt64(&m.eventWritten, count)
}

func (m *metricsCollector) incEventFailed() {
	atomic.AddInt64(&m.eventFailed, 1)
}

// GetMetrics 获取当前指标快照
func (m *metricsCollector) GetMetrics() Metrics {
	batchCount := atomic.LoadInt64(&m.telemetryBatchCount)
	totalBatchSize := atomic.LoadInt64(&m.telemetryTotalBatchSize)

	var avgBatch float64
	if batchCount > 0 {
		avgBatch = float64(totalBatchSize) / float64(batchCount)
	}

	lastFlushNano := atomic.LoadInt64(&m.telemetryLastFlush)
	var lastFlush time.Time
	if lastFlushNano > 0 {
		lastFlush = time.Unix(0, lastFlushNano)
	}

	return Metrics{
		TelemetryReceived:                    atomic.LoadInt64(&m.telemetryReceived),
		TelemetryWritten:                     atomic.LoadInt64(&m.telemetryWritten),
		TelemetryFailed:                      atomic.LoadInt64(&m.telemetryFailed),
		TelemetryDuplicatesInBatch:           atomic.LoadInt64(&m.telemetryDuplicatesInBatch),
		TelemetryBatchCount:                  batchCount,
		TelemetryAvgBatch:                    avgBatch,
		TelemetryLastFlush:                   lastFlush,
		TelemetrySpooled:                     atomic.LoadInt64(&m.telemetrySpooled),
		TelemetrySpoolReplayed:               atomic.LoadInt64(&m.telemetrySpoolReplayed),
		TelemetrySpoolFailed:                 atomic.LoadInt64(&m.telemetrySpoolFailed),
		TelemetrySpoolCorrupt:                atomic.LoadInt64(&m.telemetrySpoolCorrupt),
		TelemetrySpoolBacklog:                atomic.LoadInt64(&m.telemetrySpoolBacklog),
		TelemetrySpoolBytes:                  atomic.LoadInt64(&m.telemetrySpoolBytes),
		TelemetrySpoolQuarantineRecords:      atomic.LoadInt64(&m.telemetrySpoolQuarantineRecords),
		TelemetrySpoolQuarantineBytes:        atomic.LoadInt64(&m.telemetrySpoolQuarantineBytes),
		AttributeEventSpooled:                atomic.LoadInt64(&m.attributeEventSpooled),
		AttributeEventSpoolReplayed:          atomic.LoadInt64(&m.attributeEventSpoolReplayed),
		AttributeEventSpoolFailed:            atomic.LoadInt64(&m.attributeEventSpoolFailed),
		AttributeEventSpoolCorrupt:           atomic.LoadInt64(&m.attributeEventSpoolCorrupt),
		AttributeEventSpoolBacklog:           atomic.LoadInt64(&m.attributeEventSpoolBacklog),
		AttributeEventSpoolBytes:             atomic.LoadInt64(&m.attributeEventSpoolBytes),
		AttributeEventSpoolQuarantineRecords: atomic.LoadInt64(&m.attributeEventSpoolQuarantineRecords),
		AttributeEventSpoolQuarantineBytes:   atomic.LoadInt64(&m.attributeEventSpoolQuarantineBytes),
		AttributeEventDeadLettered:           atomic.LoadInt64(&m.attributeEventDeadLettered),
		AttributeEventDeadLetterReplayed:     atomic.LoadInt64(&m.attributeEventDeadLetterReplayed),
		AttributeEventDeadLetterFailed:       atomic.LoadInt64(&m.attributeEventDeadLetterFailed),
		AttributeWritten:                     atomic.LoadInt64(&m.attributeWritten),
		AttributeFailed:                      atomic.LoadInt64(&m.attributeFailed),
		EventWritten:                         atomic.LoadInt64(&m.eventWritten),
		EventFailed:                          atomic.LoadInt64(&m.eventFailed),
	}
}
