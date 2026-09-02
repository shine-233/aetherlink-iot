// 文件用途：实体版本控制（ROADMAP C7）的持久化模型与 HTTP 入参/出参契约。
// 核心逻辑：定义 entity_versions 表映射，以及创建快照、列表查询、恢复请求与响应结构。
// 关键注意事项：entity_type 只允许白名单值，service 层负责映射为固定表名；
// snapshot 为 JSONB，反序列化后按 map 回写，不做任意结构体绑定以防越权字段写入。
// 重构建议：若后续支持跨租户模板市场导入，先引入 manifest 校验再放开 entity_type 白名单。
package model

import "time"

const TableNameEntityVersion = "entity_versions"

// EntityVersion 实体快照版本行。
type EntityVersion struct {
	ID            string          `gorm:"column:id;primaryKey" json:"id"`
	TenantID      string          `gorm:"column:tenant_id;not null" json:"tenant_id"`
	EntityType    string          `gorm:"column:entity_type;not null" json:"entity_type"`
	EntityID      string          `gorm:"column:entity_id;not null" json:"entity_id"`
	VersionNumber int             `gorm:"column:version_number;not null" json:"version_number"`
	Snapshot      string          `gorm:"column:snapshot;type:jsonb;not null" json:"snapshot"`
	Remark        *string         `gorm:"column:remark" json:"remark"`
	CreatedBy     *string         `gorm:"column:created_by" json:"created_by"`
	CreatedAt     time.Time       `gorm:"column:created_at" json:"created_at"`
}

// TableName 返回实体版本表名。
func (*EntityVersion) TableName() string { return TableNameEntityVersion }

// EntityVersionCreateReq 创建快照请求；快照内容由 service 从实体当前行读取，不接受客户端传入。
type EntityVersionCreateReq struct {
	EntityType string  `json:"entity_type" validate:"required,max=32"`
	EntityID   string  `json:"entity_id" validate:"required,max=36"`
	Remark     *string `json:"remark" validate:"omitempty,max=500"`
}

// EntityVersionListReq 版本列表查询请求；entity_type 与 entity_id 共同定位一个实体。
type EntityVersionListReq struct {
	PageReq
	EntityType string `json:"entity_type" form:"entity_type" validate:"required,max=32"`
	EntityID   string `json:"entity_id" form:"entity_id" validate:"required,max=36"`
}

// EntityVersionListRsp 版本列表分页响应。
type EntityVersionListRsp struct {
	Total int64            `json:"total"`
	List  []*EntityVersion `json:"list"`
}

// EntityVersionRestoreReq 恢复请求；DryRun 为真时只返回将写入的字段，不落库。
type EntityVersionRestoreReq struct {
	DryRun *bool `json:"dry_run" validate:"omitempty"`
}
