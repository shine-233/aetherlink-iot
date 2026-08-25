// 文件用途：计算字段持久化模型与 HTTP 入参/出参契约。
// 核心逻辑：定义 calculated_fields 表结构映射，以及创建/更新/分页/开关请求结构与列表响应。
// 关键注意事项：output_key 是派生遥测键名，业务校验（正则、表达式可解析）在 service 层完成。
// 重构建议：若后续增加单位、标签等派生元数据列，先补 5x.sql 迁移再同步本文件与前端类型。
package model

import "time"

const TableNameCalculatedField = "calculated_fields"

// CalculatedField 计算字段：从设备遥测表达式派生新遥测键的配置行。
type CalculatedField struct {
	ID               string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID         string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Name             string    `gorm:"column:name;not null" json:"name"`
	DeviceTemplateID string    `gorm:"column:device_template_id;not null" json:"device_template_id"`
	OutputKey        string    `gorm:"column:output_key;not null" json:"output_key"`
	Expression       string    `gorm:"column:expression;not null" json:"expression"`
	Enabled          bool      `gorm:"column:enabled" json:"enabled"`
	Remark           *string   `gorm:"column:remark" json:"remark"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 返回计算字段表名。
func (*CalculatedField) TableName() string { return TableNameCalculatedField }

// CalculatedFieldCreateReq 创建计算字段请求。
type CalculatedFieldCreateReq struct {
	Name             string  `json:"name" validate:"required,max=128"`
	DeviceTemplateID string  `json:"device_template_id" validate:"required,max=36"`
	OutputKey        string  `json:"output_key" validate:"required,max=128"`
	Expression       string  `json:"expression" validate:"required,max=2000"`
	Enabled          *bool   `json:"enabled" validate:"omitempty"`
	Remark           *string `json:"remark" validate:"omitempty,max=500"`
}

// CalculatedFieldUpdateReq 更新计算字段请求；ID 由 handler 从路径参数注入。
type CalculatedFieldUpdateReq struct {
	ID               string  `json:"-" validate:"omitempty,max=36"`
	Name             string  `json:"name" validate:"required,max=128"`
	DeviceTemplateID string  `json:"device_template_id" validate:"required,max=36"`
	OutputKey        string  `json:"output_key" validate:"required,max=128"`
	Expression       string  `json:"expression" validate:"required,max=2000"`
	Remark           *string `json:"remark" validate:"omitempty,max=500"`
}

// CalculatedFieldToggleReq 启用/停用请求；Enabled 为空时按当前值取反。
type CalculatedFieldToggleReq struct {
	Enabled *bool `json:"enabled" validate:"omitempty"`
}

// CalculatedFieldListReq 分页查询请求，可按模板或名称过滤。
type CalculatedFieldListReq struct {
	PageReq
	DeviceTemplateID *string `json:"device_template_id" form:"device_template_id" validate:"omitempty,max=36"`
	Name             *string `json:"name" form:"name" validate:"omitempty,max=128"`
}

// CalculatedFieldListRsp 分页响应。
type CalculatedFieldListRsp struct {
	Total int64              `json:"total"`
	List  []*CalculatedField `json:"list"`
}
