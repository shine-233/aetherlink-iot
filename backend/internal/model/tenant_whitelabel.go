// 文件用途：定义白标（TB-47）租户翻译覆盖与自定义 CSS 的存储模型与 DTO。
// 核心逻辑：tenant_translations（UNIQUE(tenant_id,lang,key)，UPSERT 覆盖）与
//
//	tenant_custom_css（tenant_id 主键单行）两张表的 GORM 模型，
//	外加 CRUD 入参与"登录后可读覆盖获取"的出参结构。
//
// 关键注意事项：模型仅做存储与传输；lang 白名单 / key 形态 / value 与 css 的
//
//	内容约束统一在 service 层校验（ValidateTenantTranslation* /
//	SanitizeTenantCustomCSS），避免两套校验口径分叉。
//
// 重构建议：若后续需要"按用户/角色细分覆盖"或"全局行向租户级联"，
//
//	在 service 合并层扩展，不要改本表唯一键形状。
package model

import "time"

const (
	// TableNameTenantTranslation 租户翻译覆盖表（134.sql）。
	TableNameTenantTranslation = "tenant_translations"
	// TableNameTenantCustomCSS 租户自定义 CSS 表（134.sql，tenant_id 主键单行）。
	TableNameTenantCustomCSS = "tenant_custom_css"
)

// TenantTranslation 对应数据库表 tenant_translations（租户级 UI 翻译覆盖）。
// tenant_id 为空串表示系统全局行（SYS_ADMIN 作用域，语义同 logo 全局兜底行）。
type TenantTranslation struct {
	ID        string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	TenantID  string     `gorm:"column:tenant_id;not null;uniqueIndex:uk_tenant_translations_tenant_lang_key,priority:1" json:"tenant_id"`
	Lang      string     `gorm:"column:lang;not null;size:35;uniqueIndex:uk_tenant_translations_tenant_lang_key,priority:2" json:"lang"`
	Key       string     `gorm:"column:key;not null;size:200;uniqueIndex:uk_tenant_translations_tenant_lang_key,priority:3" json:"key"`
	Value     string     `gorm:"column:value;type:text;not null" json:"value"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 GORM 表名（默认复数化会得到错误的表名）。
func (*TenantTranslation) TableName() string {
	return TableNameTenantTranslation
}

// TenantCustomCSS 对应数据库表 tenant_custom_css（租户级自定义 CSS，单行）。
// 无行或 css 为空串均视为"未配置自定义样式"。
type TenantCustomCSS struct {
	TenantID  string     `gorm:"column:tenant_id;primaryKey;size:36" json:"tenant_id"`
	CSS       string     `gorm:"column:css;type:text;not null" json:"css"`
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定 GORM 表名。
func (*TenantCustomCSS) TableName() string {
	return TableNameTenantCustomCSS
}

// TenantTranslationItem 翻译覆盖写入条目（UPSERT 键 = lang + key）。
type TenantTranslationItem struct {
	Lang  string `json:"lang" validate:"required,max=35"`
	Key   string `json:"key" validate:"required,max=200"`
	Value string `json:"value" validate:"required,max=4000"`
}

// UpsertTenantTranslationsReq 批量写入翻译覆盖入参（同请求内键重复以后者为准）。
type UpsertTenantTranslationsReq struct {
	Items []TenantTranslationItem `json:"items" validate:"required,min=1,max=500"`
}

// TenantTranslationKeyItem 翻译覆盖删除定位键（不携带 value）。
type TenantTranslationKeyItem struct {
	Lang string `json:"lang" validate:"required,max=35"`
	Key  string `json:"key" validate:"required,max=200"`
}

// DeleteTenantTranslationsReq 批量删除翻译覆盖入参。
type DeleteTenantTranslationsReq struct {
	Items []TenantTranslationKeyItem `json:"items" validate:"required,min=1,max=500"`
}

// ListTenantTranslationsReq 管理面列表入参（GET query 绑定；lang 可选过滤）。
type ListTenantTranslationsReq struct {
	Lang string `form:"lang" json:"lang" validate:"omitempty,max=35"`
}

// UpsertTenantCustomCSSReq 自定义 CSS 写入入参；空串语义为"清除自定义样式"。
type UpsertTenantCustomCSSReq struct {
	CSS string `json:"css" validate:"max=65536"`
}

// TenantWhitelabelOverrides 登录后可读的覆盖获取端点出参：
// Translations 按 lang 分组为 key→value 映射（前端 i18n 初始化后合并），CSS 为本租户自定义样式。
type TenantWhitelabelOverrides struct {
	TenantID     string                       `json:"tenant_id"`
	Translations map[string]map[string]string `json:"translations"`
	CSS          string                       `json:"css"`
}
