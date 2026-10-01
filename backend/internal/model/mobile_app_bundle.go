// 文件用途：定义移动应用中心（mobile_app_bundles，ROADMAP TB-23）的持久化模型与 HTTP 入参/出参契约。
// 核心逻辑：应用包版本登记实体（平台/版本/文件路径/SHA-256 校验和/发布状态机）与
//
//	列表查询、发布/归档流转的请求响应结构。
//
// 关键注意事项：UNIQUE(tenant_id, platform, version) 与 136.sql 的
//
//	uk_mobile_app_bundles_tenant_platform_version 同源，版本唯一性是租户内概念；
//	file_path 是对外访问路径（./files/apps/...），磁盘真实路径的解析与包含性校验
//	统一在 service 层完成，禁止客户端直接传 file_path；status 流转合法矩阵
//	（draft→published→archived）的判定集中在 service.CanTransitionAppBundle，本文件不写业务规则。
//
// 重构建议：若后续 uniapp 对接阶段需要"已发布包"的独立查询面（按 platform 取最新 published），
//
//	加只读 DAL 查询而不是给 status 加第四种值；多文件（obb/dSYM）需求出现时另立明细表。
package model

import "time"

const TableNameMobileAppBundle = "mobile_app_bundles"

// 应用包目标平台枚举（136.sql CHECK 约束同源，服务层以本组常量为准）。
const (
	AppBundlePlatformAndroid = "android"
	AppBundlePlatformIOS     = "ios"
	AppBundlePlatformH5      = "h5"
)

// 应用包发布状态枚举（136.sql CHECK 约束同源）。
const (
	AppBundleStatusDraft     = "draft"
	AppBundleStatusPublished = "published"
	AppBundleStatusArchived  = "archived"
)

// MobileAppBundle 对应数据库表 mobile_app_bundles（移动应用包版本登记行）。
// UNIQUE(tenant_id, platform, version) 与 136.sql 的
// uk_mobile_app_bundles_tenant_platform_version 同源，重复版本的最终保障。
type MobileAppBundle struct {
	ID       string `gorm:"column:id;primaryKey" json:"id"`
	TenantID string `gorm:"column:tenant_id;not null;uniqueIndex:uk_mobile_app_bundles_tenant_platform_version,priority:1" json:"tenant_id"`
	Platform string `gorm:"column:platform;not null;uniqueIndex:uk_mobile_app_bundles_tenant_platform_version,priority:2" json:"platform"`
	Version  string `gorm:"column:version;not null;uniqueIndex:uk_mobile_app_bundles_tenant_platform_version,priority:3" json:"version"`
	FileName string `gorm:"column:file_name;not null" json:"file_name"`
	// FilePath 对外访问路径（./files/apps/<platform>/<日期>/<哈希>.<ext>）。
	FilePath string `gorm:"column:file_path;not null" json:"file_path"`
	FileSize int64  `gorm:"column:file_size;not null;default:0" json:"file_size"`
	// Checksum 文件 SHA-256 十六进制（上传时计算，入库后只读）。
	Checksum     string `gorm:"column:checksum;not null" json:"checksum"`
	ReleaseNotes string `gorm:"column:release_notes" json:"release_notes"`
	Status       string `gorm:"column:status;not null;default:'draft'" json:"status"`
	// PublishedAt 发布时间（draft→published 流转落 UTC 时刻；归档不清除，保留发布履历）。
	PublishedAt *time.Time `gorm:"column:published_at" json:"published_at"`
	CreatedAt   *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 返回移动应用包登记表名。
func (*MobileAppBundle) TableName() string { return TableNameMobileAppBundle }

// IsValidAppBundlePlatform 平台枚举判定（服务层 fail-closed 校验用）。
func IsValidAppBundlePlatform(platform string) bool {
	switch platform {
	case AppBundlePlatformAndroid, AppBundlePlatformIOS, AppBundlePlatformH5:
		return true
	default:
		return false
	}
}

// GetAppBundleListReq 应用包列表查询入参：分页 + 平台/状态精确过滤。
type GetAppBundleListReq struct {
	PageReq
	Platform *string `form:"platform" json:"platform" validate:"omitempty,oneof=android ios h5"`
	Status   *string `form:"status" json:"status" validate:"omitempty,oneof=draft published archived"`
}

// GetAppBundleListRsp 应用包列表分页响应。
type GetAppBundleListRsp struct {
	Total int64              `json:"total"`
	List  []*MobileAppBundle `json:"list"`
}

// UpdateAppBundleReq 应用包更新入参：仅 draft 状态可改（服务层强制）。
type UpdateAppBundleReq struct {
	ReleaseNotes *string `json:"release_notes" validate:"omitempty,max=2000"`
}

// AppBundleCreateInput 上传登记入参（由 API 层从 multipart 表单字段组装；文件本体以
// *multipart.FileHeader 单独传入 service，落盘与校验和计算都在 service 层完成；
// tenant 由 API 层按 claims 解析后显式传入 TenantID，模型层不感知凭证）。
type AppBundleCreateInput struct {
	TenantID     string
	Platform     string
	Version      string
	ReleaseNotes string
	FileName     string // 上传时的原始文件名（调用方负责清洗）
}
