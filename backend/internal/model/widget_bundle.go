// 文件用途：定义部件库（widget_bundles，ROADMAP TB-04）相关模型与 DTO。
// 核心逻辑：租户级部件库实体（部件定义 JSONB + 版本/行业类型）与 CRUD 入参出参；
//
//	另含资源中心导出/导入用的 WidgetBundleExport 描述符。
//
// 关键注意事项：widgets 列形状对齐 service.WidgetDefinition（type/version/schema/
//
//	capabilities/commands），本模型只做存储与传输，结构校验统一在 service 层
//	（复用 ValidateWidgetDefinition），避免两套校验口径分叉。
//
// 重构建议：若后续部件库需要封面图与分类扩展，优先复用 type_key 与资源中心列表，
//
//	不要另立一张市场表。
package model

import "time"

const TableNameWidgetBundle = "widget_bundles"

// WidgetBundle 对应数据库表 widget_bundles
type WidgetBundle struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	Name        string     `gorm:"column:name;not null" json:"name"`
	TenantID    string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Widgets     string     `gorm:"column:widgets;type:jsonb;not null" json:"widgets"` // 部件定义 JSON 数组
	Description *string    `gorm:"column:description" json:"description"`
	Version     string     `gorm:"column:version;not null;default:'1.0.0'" json:"version"`
	TypeKey     *string    `gorm:"column:type_key" json:"type_key"` // 行业分类（资源中心目录聚合用）
	CreatedAt   *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (*WidgetBundle) TableName() string {
	return TableNameWidgetBundle
}

// WidgetBundleExport 部件库导出描述符（资源中心统一打包载荷用）。
// Kind 固定 aetherlink-widget-bundle，导入侧校验；Widgets 为部件定义 JSON 数组字符串。
type WidgetBundleExport struct {
	Kind        string  `json:"kind" validate:"omitempty"`                // 固定 aetherlink-widget-bundle
	Name        string  `json:"name" validate:"required,max=255"`         // 部件库名称
	Author      *string `json:"author" validate:"omitempty,max=99"`       // 作者
	Version     *string `json:"version" validate:"omitempty,max=32"`      // 版本号
	Description *string `json:"description" validate:"omitempty,max=500"` // 描述
	TypeKey     *string `json:"type_key" validate:"omitempty,max=64"`     // 行业分类
	Widgets     string  `json:"widgets" validate:"omitempty"`             // 部件定义 JSON 数组
	ExportedAt  string  `json:"exported_at" validate:"omitempty"`         // 导出时间（RFC3339）
}

// ImportWidgetBundleReq 部件库导入入参（资源中心逐项回放用）。
type ImportWidgetBundleReq = WidgetBundleExport

// CreateWidgetBundleReq 创建部件库入参
type CreateWidgetBundleReq struct {
	Name        string  `json:"name" validate:"required,max=255"`
	Widgets     *string `json:"widgets" validate:"omitempty,max=1048576"` // 缺省落为空数组 []
	Description *string `json:"description" validate:"omitempty,max=500"`
	Version     *string `json:"version" validate:"omitempty,max=32"`
	TypeKey     *string `json:"type_key" validate:"omitempty,max=64"`
}

// UpdateWidgetBundleReq 更新部件库入参
type UpdateWidgetBundleReq struct {
	ID          string  `json:"id" validate:"required,max=36"`
	Name        *string `json:"name" validate:"omitempty,max=255"`
	Widgets     *string `json:"widgets" validate:"omitempty,max=1048576"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	Version     *string `json:"version" validate:"omitempty,max=32"`
	TypeKey     *string `json:"type_key" validate:"omitempty,max=64"`
}

// GetWidgetBundleListReq 分页查询部件库列表入参
type GetWidgetBundleListReq struct {
	PageReq
	Search  *string `form:"search" json:"search" validate:"omitempty,max=100"`
	TypeKey *string `form:"type_key" json:"type_key" validate:"omitempty,max=64"`
}

// WidgetBundleSeedRsp 内置部件种子 bundle 一键落库响应。
// Idempotent 标记本次是否命中"已存在同名种子库"的幂等路径。
type WidgetBundleSeedRsp struct {
	Bundle     *WidgetBundle `json:"bundle"`
	Idempotent bool          `json:"idempotent"`
}
