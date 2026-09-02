// 文件用途：资产（ROADMAP C2）模型——租户内资产树节点（设备/区域/产线等）。
// 核心逻辑：parent_id 自引用成树；租户边界 tenant_id 由 DAL/Service 按 Scope 过滤。
// 关键注意事项：meta 为 JSONB 扩展信息，跨 PG/sqlite 使用 *string + type:jsonb 保持兼容。
package model

import "time"

// Asset 租户资产树节点。parent_id 空串表示根节点。
type Asset struct {
	ID        string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID  string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ParentID  string     `gorm:"column:parent_id;not null;default:''" json:"parent_id"`
	Name      string     `gorm:"column:name;not null" json:"name"`
	AssetType string     `gorm:"column:asset_type;not null;default:device" json:"asset_type"`
	Meta      *string    `gorm:"column:meta;type:jsonb" json:"meta"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName Asset's table name
func (*Asset) TableName() string { return "assets" }
