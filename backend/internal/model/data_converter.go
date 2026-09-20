// 文件用途：定义数据转换器（ThingsBoard Data Converter 载荷解析与编解码）相关模型与 DTO。
// 核心逻辑：对标 ThingsBoard Integrations，支持 UPLINK/DOWNLINK 转换器配置、仿真调试与规则链挂载。
package model

import "time"

const TableNameDataConverter = "data_converters"

// DataConverter 对应数据库表 data_converters
type DataConverter struct {
	ID            string     `gorm:"column:id;primaryKey" json:"id"`
	Name          string     `gorm:"column:name;not null" json:"name"`
	Type          string     `gorm:"column:type;not null" json:"type"`                     // UPLINK / DOWNLINK
	ConverterMode string     `gorm:"column:converter_mode;not null" json:"converter_mode"` // SCRIPT / HEX_BINARY / JSON_PATH
	DebugMode     bool       `gorm:"column:debug_mode;not null" json:"debug_mode"`
	TenantID      string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Configuration string     `gorm:"column:configuration;not null" json:"configuration"`
	Script        *string    `gorm:"column:script" json:"script"`
	Description   *string    `gorm:"column:description" json:"description"`
	CreatedAt     *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (*DataConverter) TableName() string {
	return TableNameDataConverter
}

// CreateDataConverterReq 创建转换器入参
type CreateDataConverterReq struct {
	Name          string  `json:"name" validate:"required,max=255"`
	Type          string  `json:"type" validate:"required,oneof=UPLINK DOWNLINK"`
	ConverterMode string  `json:"converter_mode" validate:"required,oneof=SCRIPT HEX_BINARY JSON_PATH"`
	DebugMode     bool    `json:"debug_mode"`
	Configuration *string `json:"configuration"`
	Script        *string `json:"script"`
	Description   *string `json:"description" validate:"omitempty,max=500"`
}

// UpdateDataConverterReq 更新转换器入参
type UpdateDataConverterReq struct {
	ID            string  `json:"id" validate:"required,max=36"`
	Name          *string `json:"name" validate:"omitempty,max=255"`
	Type          *string `json:"type" validate:"omitempty,oneof=UPLINK DOWNLINK"`
	ConverterMode *string `json:"converter_mode" validate:"omitempty,oneof=SCRIPT HEX_BINARY JSON_PATH"`
	DebugMode     *bool   `json:"debug_mode"`
	Configuration *string `json:"configuration"`
	Script        *string `json:"script"`
	Description   *string `json:"description" validate:"omitempty,max=500"`
}

// GetDataConverterListReq 分页查询列表入参
type GetDataConverterListReq struct {
	PageReq
	Type   *string `form:"type" json:"type" validate:"omitempty,oneof=UPLINK DOWNLINK"`
	Search *string `form:"search" json:"search" validate:"omitempty,max=100"`
}

// TestDataConverterReq 仿真调试入参
type TestDataConverterReq struct {
	Type          string            `json:"type" validate:"omitempty,oneof=UPLINK DOWNLINK"`
	ConverterMode string            `json:"converter_mode" validate:"omitempty,oneof=SCRIPT HEX_BINARY JSON_PATH"`
	Payload       string            `json:"payload" validate:"required"` // 16进制字符串、JSON或文本
	Metadata      map[string]string `json:"metadata"`
	Configuration *string           `json:"configuration"`
	Script        *string           `json:"script"`
	ConverterID   *string           `json:"converter_id"`
}

// TestDataConverterResp 仿真调试返回结果
type TestDataConverterResp struct {
	Success    bool                   `json:"success"`
	DeviceName string                 `json:"device_name,omitempty"`
	DeviceType string                 `json:"device_type,omitempty"`
	Telemetry  map[string]interface{} `json:"telemetry,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	RawOutput  string                 `json:"raw_output,omitempty"`
	Logs       []string               `json:"logs,omitempty"`
	Error      string                 `json:"error,omitempty"`
}
