// 文件用途：定义 payload schema registry 的持久化模型（手写 gorm 模型，非 gen 生成）。
// 核心逻辑：把租户级 payload 字段约束(name/type/required/min/max/enum/pattern)按 jsonb 持久化,
//
//	让 CRUD 层复用已有的纯校验引擎(ValidatePayload),保持"校验逻辑单一来源"。
//
// 关键注意事项：该表只持久化"声明的约束",broker 侧对上行 payload 的真实拦截仍属外部 MQTT
//
//	契约的破坏性变更,需运行时(broker+PG+设备)验证,不由本模型或其 CRUD 承担。
package model

import "time"

const TableNamePayloadSchema = "payload_schemas"

// PayloadSchemaRecord 是 payload_schemas 表的持久化行。
// Fields 以 jsonb 存储 []PayloadSchemaField 的序列化结果,与纯校验引擎消费的形状一致。
type PayloadSchemaRecord struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description *string   `gorm:"column:description" json:"description,omitempty"`
	Strict      bool      `gorm:"column:strict;not null" json:"strict"`
	Fields      string    `gorm:"column:fields;type:jsonb;not null" json:"fields"`
	CreatedBy   *string   `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*PayloadSchemaRecord) TableName() string {
	return TableNamePayloadSchema
}

// SavePayloadSchemaReq 创建或更新一个持久化 payload schema。
type SavePayloadSchemaReq struct {
	ID          string               `json:"id" validate:"omitempty,max=36"`
	Name        string               `json:"name" validate:"required,max=128"`
	Description *string              `json:"description" validate:"omitempty,max=500"`
	Strict      bool                 `json:"strict"`
	Fields      []PayloadSchemaField `json:"fields" validate:"required,dive"`
}

// PayloadSchemaRsp 是对外返回的 payload schema 视图(fields 已回解为结构化数组)。
type PayloadSchemaRsp struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description *string              `json:"description,omitempty"`
	Strict      bool                 `json:"strict"`
	Fields      []PayloadSchemaField `json:"fields"`
	CreatedBy   *string              `json:"created_by,omitempty"`
	CreatedAt   *time.Time           `json:"created_at,omitempty"`
	UpdatedAt   *time.Time           `json:"updated_at,omitempty"`
}

// PayloadSchemaListRsp 是 payload schema 列表返回。
type PayloadSchemaListRsp struct {
	List []PayloadSchemaRsp `json:"list"`
}
