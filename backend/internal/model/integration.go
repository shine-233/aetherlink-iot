// 文件用途：定义统一集成实体（Integration，TB-45 对标 ThingsBoard Integrations）模型与 DTO。
// 核心逻辑：Integration = 连接器（opcua/snmp/plugin）+ 上下行转换器绑定 + config JSONB
//
//	（device_ids 数组即设备绑定关系）；供 CRUD 栈与采集器 Integration 管线共用。
//
// 关键注意事项：converter_* 外键 ON DELETE SET NULL（130.sql），删转换器自动解绑不级联；
// 下行转换器反向下发执行按批次范围不做，converter_downlink_id 仅存储绑定语义。
// 重构建议：SNMP/插件连接器同构接入后，可把 connector 专属配置从 config JSONB 拆成类型化列。
package model

import (
	"encoding/json"
	"time"
)

const TableNameIntegration = "integrations"

// Integration 连接器类型常量（CHECK 约束与 oneof 校验双保险）。
const (
	IntegrationConnectorOpcua  = "opcua"
	IntegrationConnectorSnmp   = "snmp"
	IntegrationConnectorPlugin = "plugin"
)

// Integration 对应数据库表 integrations（130.sql）。
type Integration struct {
	ID                  string     `gorm:"column:id;primaryKey" json:"id"`
	Name                string     `gorm:"column:name;not null" json:"name"`
	TenantID            string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ConnectorType       string     `gorm:"column:connector_type;not null" json:"connector_type"` // opcua / snmp / plugin
	ConverterUplinkID   *string    `gorm:"column:converter_uplink_id" json:"converter_uplink_id"`
	ConverterDownlinkID *string    `gorm:"column:converter_downlink_id" json:"converter_downlink_id"`
	Config              string     `gorm:"column:config;not null;type:jsonb" json:"config"`
	Enabled             bool       `gorm:"column:enabled;not null" json:"enabled"`
	CreatedAt           *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (*Integration) TableName() string {
	return TableNameIntegration
}

// CreateIntegrationReq 创建集成实例入参
type CreateIntegrationReq struct {
	Name                string  `json:"name" validate:"required,max=255"`
	ConnectorType       string  `json:"connector_type" validate:"required,oneof=opcua snmp plugin"`
	ConverterUplinkID   *string `json:"converter_uplink_id" validate:"omitempty,max=36"`
	ConverterDownlinkID *string `json:"converter_downlink_id" validate:"omitempty,max=36"`
	Config              *string `json:"config" validate:"omitempty,max=100000"`
	Enabled             *bool   `json:"enabled"`
}

// UpdateIntegrationReq 更新集成实例入参
type UpdateIntegrationReq struct {
	ID                  string  `json:"id" validate:"required,max=36"`
	Name                *string `json:"name" validate:"omitempty,max=255"`
	ConnectorType       *string `json:"connector_type" validate:"omitempty,oneof=opcua snmp plugin"`
	ConverterUplinkID   *string `json:"converter_uplink_id" validate:"omitempty,max=36"`
	ConverterDownlinkID *string `json:"converter_downlink_id" validate:"omitempty,max=36"`
	Config              *string `json:"config" validate:"omitempty,max=100000"`
	Enabled             *bool   `json:"enabled"`
}

// GetIntegrationListReq 分页查询列表入参
type GetIntegrationListReq struct {
	PageReq
	ConnectorType *string `form:"connector_type" json:"connector_type" validate:"omitempty,oneof=opcua snmp plugin"`
	Enabled       *bool   `form:"enabled" json:"enabled"`
	Search        *string `form:"search" json:"search" validate:"omitempty,max=100"`
}

// IntegrationBindingConfig 集成实例 config JSONB 中与管线相关的绑定段。
// device_ids 为绑定的设备 UUID 列表——采集器据此判断设备是否命中集成实例。
type IntegrationBindingConfig struct {
	DeviceIDs []string `json:"device_ids,omitempty"`
}

// ParseDeviceIDs 从 config JSONB 提取 device_ids；坏 JSON/缺失时返回 nil（绑定视为空）。
func ParseDeviceIDs(configJSON string) []string {
	var conf IntegrationBindingConfig
	if err := json.Unmarshal([]byte(configJSON), &conf); err != nil {
		return nil
	}
	return conf.DeviceIDs
}
