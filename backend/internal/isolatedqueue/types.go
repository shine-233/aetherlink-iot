package isolatedqueue

import (
	"errors"
	"time"
)

// 标准队列名称常量（对标 ThingsBoard 3.6.3+ 核心队列模型）。
const (
	QueueTypeMain                   = "Main"
	QueueTypeHighPriority           = "HighPriority"
	QueueTypeSequentialByOriginator = "SequentialByOriginator"
)

// 提交策略常量。
const (
	SubmitStrategyBurst                  = "BURST"
	SubmitStrategySequentialByOriginator = "SEQUENTIAL_BY_ORIGINATOR"
	SubmitStrategyBatch                  = "BATCH"
)

// 溢出丢弃策略常量。
const (
	DropPolicyBackpressure = "BACKPRESSURE"
	DropPolicyDropOldest   = "DROP_OLDEST"
	DropPolicyDropNewest   = "DROP_NEWEST"
)

// 队列健康度常量。
const (
	HealthStatusHealthy      = "HEALTHY"
	HealthStatusWarning      = "WARNING"
	HealthStatusCriticalFull = "CRITICAL_FULL"
)

// 错误常量。
var (
	ErrQueueNotFound = errors.New("queue not found")
	ErrQueueClosed   = errors.New("queue is closed")
	ErrQueueFull     = errors.New("queue is full")
	ErrInvalidMsg    = errors.New("invalid queue message")
)

// QueueMessage 隔离队列承载的标准消息实体。
type QueueMessage struct {
	ID           string                 `json:"id"`
	QueueName    string                 `json:"queue_name"`
	OriginatorID string                 `json:"originator_id"` // 实体源（如 DeviceID）
	Type         string                 `json:"type"`          // telemetry | status | alarm | command | response
	Payload      []byte                 `json:"payload"`
	Metadata     map[string]interface{} `json:"metadata"`
	Timestamp    time.Time              `json:"timestamp"`
}

// QueueConfig 队列配置。
type QueueConfig struct {
	Name            string `json:"name"`
	Capacity        int    `json:"capacity"`
	SubmitStrategy  string `json:"submit_strategy"`
	DropPolicy      string `json:"drop_policy"`
	Workers         int    `json:"workers"`
	PartitionCount  int    `json:"partition_count"` // 针对 SequentialByOriginator 分区数
}

// QueueStats 队列运行期指标快照。
type QueueStats struct {
	Name           string    `json:"name"`
	Strategy       string    `json:"strategy"`
	Capacity       int       `json:"capacity"`
	Size           int       `json:"size"`
	TotalSubmitted int64     `json:"total_submitted"`
	TotalProcessed int64     `json:"total_processed"`
	TotalDropped   int64     `json:"total_dropped"`
	HealthStatus   string    `json:"health_status"`
	LastActivityAt time.Time `json:"last_activity_at"`
}
