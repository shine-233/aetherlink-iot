// 文件用途：数据转发规则（forward_rules）的手写模型与 HTTP DTO。
// 核心逻辑：定义规则实体、分页查询请求/响应与脱敏响应装配。
// 关键注意事项：mqtt_password 属敏感字段，任何列表/详情出参一律以掩码返回；
// 存储侧 Phase 1 暂为明文列，加密挂账见 backend-hardening-plan.md。

package model

import "time"

const TableNameForwardRule = "forward_rules"

// ForwardRule 数据转发规则实体（手写模型，表由 52.sql 创建）。
type ForwardRule struct {
	ID               string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID         string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Name             string    `gorm:"column:name;not null" json:"name"`
	Enabled          bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	SourceType       string    `gorm:"column:source_type;not null" json:"source_type"` // telemetry|property|event|status
	DeviceTemplateID *string   `gorm:"column:device_template_id" json:"device_template_id,omitempty"`
	Script           *string   `gorm:"column:script" json:"script,omitempty"`
	TargetType       string    `gorm:"column:target_type;not null" json:"target_type"` // http|mqtt
	HttpURL          *string   `gorm:"column:http_url" json:"http_url,omitempty"`
	HttpMethod       *string   `gorm:"column:http_method" json:"http_method,omitempty"`
	HttpHeaders      *string   `gorm:"column:http_headers" json:"http_headers,omitempty"` // JSON 对象字符串
	MqttBroker       *string   `gorm:"column:mqtt_broker" json:"mqtt_broker,omitempty"`
	MqttTopic        *string   `gorm:"column:mqtt_topic" json:"mqtt_topic,omitempty"`
	MqttUsername     *string   `gorm:"column:mqtt_username" json:"mqtt_username,omitempty"`
	MqttPassword     *string   `gorm:"column:mqtt_password" json:"-"`
	Remark           *string   `gorm:"column:remark" json:"remark,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName 表名。
func (*ForwardRule) TableName() string { return TableNameForwardRule }

// GetForwardRuleListByPageReq 分页查询请求。
type GetForwardRuleListByPageReq struct {
	PageReq
	Name       *string `json:"name" form:"name" validate:"omitempty,max=128"`
	Enabled    *bool   `json:"enabled" form:"enabled"`
	SourceType *string `json:"source_type" form:"source_type" validate:"omitempty,oneof=telemetry property event status"`
	TargetType *string `json:"target_type" form:"target_type" validate:"omitempty,oneof=http mqtt"`
}
