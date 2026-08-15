// 文件用途：提供遥测、属性或事件存储模块的 types 能力。
// 核心逻辑：管理存储配置、消息模型、批量写入、去重、指标采集和直写通道，主要围绕 type DataType、type Message、type TelemetryDataPoint、type AttributeDataPoint 等声明展开。
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。
// 重构建议：后续可将批处理策略、指标和数据库写入进一步解耦，便于压测和替换实现。

package storage

import (
	"encoding/json"
	"time"
)

// DataType 数据类型
type DataType string

const (
	DataTypeTelemetry DataType = "telemetry"
	DataTypeAttribute DataType = "attribute"
	DataTypeEvent     DataType = "event"
)

// Message 统一消息格式
type Message struct {
	// MessageID is the stable identity of a frozen attribute/event envelope.
	// It is UUID-shaped because it is also used by event_datas and the durable
	// receipt/dead-letter tables. The boundary fills it once when omitted.
	MessageID string `json:"message_id,omitempty"`
	// SourceMessageID is an opaque protocol identity (for example the MQTT topic
	// message_id). It is never persisted verbatim; storage derives a scoped UUID
	// from it so retransmission is idempotent without exposing protocol values.
	SourceMessageID string      `json:"-"`
	DeviceID        string      `json:"device_id"`
	TenantID        string      `json:"tenant_id"`
	DataType        DataType    `json:"data_type"`
	Timestamp       int64       `json:"timestamp"` // 毫秒时间戳
	Data            interface{} `json:"data"`
	// telemetryWriteAheadPrepared is set only after the producer-side durable
	// pre-enqueue hook has persisted every point to the telemetry spool.
	telemetryWriteAheadPrepared bool
}

// TelemetryDataPoint 遥测数据点
type TelemetryDataPoint struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// AttributeDataPoint 属性数据点
type AttributeDataPoint struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// EventData 事件数据
type EventData struct {
	Identify string          `json:"identify"`
	Data     json.RawMessage `json:"data"`
}

// 数据库模型

// TelemetryData 遥测历史数据
type TelemetryData struct {
	DeviceID string   `gorm:"column:device_id;primaryKey"`
	Key      string   `gorm:"column:key;primaryKey"`
	TS       int64    `gorm:"column:ts;primaryKey"` // 毫秒时间戳
	BoolV    *bool    `gorm:"column:bool_v"`
	NumberV  *float64 `gorm:"column:number_v"`
	StringV  *string  `gorm:"column:string_v"`
	TenantID string   `gorm:"column:tenant_id"`
}

func (TelemetryData) TableName() string {
	return "telemetry_datas"
}

// TelemetryCurrentData 遥测最新值
type TelemetryCurrentData struct {
	DeviceID string    `gorm:"column:device_id;primaryKey"`
	Key      string    `gorm:"column:key;primaryKey"`
	TS       time.Time `gorm:"column:ts"`
	BoolV    *bool     `gorm:"column:bool_v"`
	NumberV  *float64  `gorm:"column:number_v"`
	StringV  *string   `gorm:"column:string_v"`
	TenantID string    `gorm:"column:tenant_id"`
}

func (TelemetryCurrentData) TableName() string {
	return "telemetry_current_datas"
}

// TelemetryDeadLetter stores telemetry rows that could not be written even
// after fallback insertion, preserving a replayable payload for operators.
type TelemetryDeadLetter struct {
	ID          string          `gorm:"column:id;primaryKey"`
	DeviceID    string          `gorm:"column:device_id"`
	TenantID    string          `gorm:"column:tenant_id"`
	Key         string          `gorm:"column:key"`
	TS          int64           `gorm:"column:ts"`
	BoolV       *bool           `gorm:"column:bool_v"`
	NumberV     *float64        `gorm:"column:number_v"`
	StringV     *string         `gorm:"column:string_v"`
	RawPayload  json.RawMessage `gorm:"column:raw_payload;type:jsonb"`
	Status      string          `gorm:"column:status"`
	Attempts    int             `gorm:"column:attempts"`
	LastError   string          `gorm:"column:last_error"`
	NextRetryAt *time.Time      `gorm:"column:next_retry_at"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
}

func (TelemetryDeadLetter) TableName() string {
	return "telemetry_dead_letters"
}

// AttributeData 属性数据
type AttributeData struct {
	ID       string    `gorm:"column:id;primaryKey"`
	DeviceID string    `gorm:"column:device_id"`
	Key      string    `gorm:"column:key"`
	TS       time.Time `gorm:"column:ts"`
	BoolV    *bool     `gorm:"column:bool_v"`
	NumberV  *float64  `gorm:"column:number_v"`
	StringV  *string   `gorm:"column:string_v"`
	TenantID string    `gorm:"column:tenant_id"`
}

func (AttributeData) TableName() string {
	return "attribute_datas"
}

// EventDataModel 事件数据
type EventDataModel struct {
	ID       string          `gorm:"column:id;primaryKey"`
	DeviceID string          `gorm:"column:device_id"`
	Identify string          `gorm:"column:identify"`
	TS       time.Time       `gorm:"column:ts"`
	Data     json.RawMessage `gorm:"column:data;type:json"`
	TenantID string          `gorm:"column:tenant_id"`
}

func (EventDataModel) TableName() string {
	return "event_datas"
}
