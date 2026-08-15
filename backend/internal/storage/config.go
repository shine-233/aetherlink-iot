// 文件用途：提供遥测、属性或事件存储模块的 config 能力。
// 核心逻辑：管理存储配置、消息模型、批量写入、去重、指标采集和直写通道，主要围绕 type Config、func DefaultConfig、func (c Config) GetFlushDuration 等声明展开。
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。
// 重构建议：后续可将批处理策略、指标和数据库写入进一步解耦，便于压测和替换实现。

package storage

import "time"

// Config 存储层配置
type Config struct {
	// 输入channel缓冲区大小
	ChannelBufferSize int

	// 遥测数据批量大小
	TelemetryBatchSize int

	// 遥测数据flush间隔(毫秒)，0表示关闭定时flush
	TelemetryFlushInterval int

	// 是否启用Prometheus监控
	EnableMetrics bool

	// TelemetrySpoolEnabled controls the independent filesystem fallback used
	// when both the primary telemetry write and PostgreSQL dead-letter write fail.
	TelemetrySpoolEnabled bool

	// TelemetrySpoolDirectory must live on persistent storage independent from
	// PostgreSQL. Files contain raw telemetry values and require restricted OS
	// permissions (and encrypted storage when those values are sensitive).
	TelemetrySpoolDirectory string

	// TelemetrySpoolMaxBytes is the maximum logical JSON payload size retained.
	// The spool never evicts older records to make room for newer ones.
	TelemetrySpoolMaxBytes int64

	// TelemetrySpoolMaxRecords caps the number of finalized spool records.
	TelemetrySpoolMaxRecords int

	// TelemetrySpoolMaxRecordBytes rejects a single unexpectedly large point
	// before it can monopolize spool capacity or replay memory.
	TelemetrySpoolMaxRecordBytes int64

	// TelemetrySpoolReplayInterval controls background replay attempts.
	TelemetrySpoolReplayInterval time.Duration

	// TelemetrySpoolReplayBatchSize caps each replay pass.
	TelemetrySpoolReplayBatchSize int

	// TelemetrySpoolReplayTimeout bounds one database replay pass.
	TelemetrySpoolReplayTimeout time.Duration

	// TelemetryWriteAheadSpoolEnabled closes the in-memory batch buffer loss
	// window: each point is spooled to disk before it enters the buffer and the
	// record is removed only after the batch commits. Without it a SIGKILL or
	// crash loses up to TelemetryBatchSize points that the protocol layer has
	// already acknowledged.
	//
	// This costs one fsync per point. Operators may explicitly opt out, but the
	// production default keeps the producer-to-consumer queue gap closed.
	TelemetryWriteAheadSpoolEnabled bool

	// AttributeEventSpoolEnabled controls the independent filesystem fallback
	// used when an attribute/event envelope cannot be committed to PostgreSQL.
	AttributeEventSpoolEnabled bool

	// AttributeEventSpoolDirectory must be a private persistent directory that
	// is independent from both PostgreSQL and the telemetry spool.
	AttributeEventSpoolDirectory string

	// AttributeEventSpoolMaxBytes and AttributeEventSpoolMaxRecords bound all
	// committed and quarantined envelopes. Existing records are never evicted.
	AttributeEventSpoolMaxBytes   int64
	AttributeEventSpoolMaxRecords int

	// AttributeEventSpoolMaxRecordBytes bounds one canonical envelope.
	AttributeEventSpoolMaxRecordBytes int64

	// AttributeEventSpoolReplayInterval controls background replay attempts.
	AttributeEventSpoolReplayInterval time.Duration

	// AttributeEventSpoolReplayBatchSize caps each replay pass.
	AttributeEventSpoolReplayBatchSize int

	// AttributeEventSpoolReplayTimeout bounds replay transactions and the
	// detached fallback attempted after a caller context is cancelled.
	AttributeEventSpoolReplayTimeout time.Duration
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		ChannelBufferSize:                  10000,
		TelemetryBatchSize:                 500,
		TelemetryFlushInterval:             1000, // 1秒
		EnableMetrics:                      true,
		TelemetrySpoolEnabled:              true,
		TelemetrySpoolDirectory:            "./data/telemetry-spool",
		TelemetrySpoolMaxBytes:             512 * 1024 * 1024,
		TelemetrySpoolMaxRecords:           100000,
		TelemetrySpoolMaxRecordBytes:       16 * 1024 * 1024,
		TelemetrySpoolReplayInterval:       30 * time.Second,
		TelemetrySpoolReplayBatchSize:      100,
		TelemetrySpoolReplayTimeout:        10 * time.Second,
		TelemetryWriteAheadSpoolEnabled:    true,
		AttributeEventSpoolEnabled:         true,
		AttributeEventSpoolDirectory:       "./data/uplink-spool",
		AttributeEventSpoolMaxBytes:        512 * 1024 * 1024,
		AttributeEventSpoolMaxRecords:      100000,
		AttributeEventSpoolMaxRecordBytes:  16 * 1024 * 1024,
		AttributeEventSpoolReplayInterval:  30 * time.Second,
		AttributeEventSpoolReplayBatchSize: 100,
		AttributeEventSpoolReplayTimeout:   10 * time.Second,
	}
}

// GetFlushDuration 获取flush间隔时长
func (c Config) GetFlushDuration() time.Duration {
	if c.TelemetryFlushInterval <= 0 {
		return 0 // 关闭定时flush
	}
	return time.Duration(c.TelemetryFlushInterval) * time.Millisecond
}
