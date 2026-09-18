// 文件用途：通用 Secrets Storage（ROADMAP TB-18）模型定义与 HTTP DTO。
// 核心逻辑：定义 sys_secrets 实体、创建/更新/列表请求与安全脱敏出参。
// 关键注意事项：
//  1. 数据库实体仅存 EncryptedValue（信封密文），明文永不直接持久化；
//  2. 列表和普通详情出参必须使用 MaskPreview，禁止泄漏明文或密文内容；
//  3. RevealSecretResp 仅供特定解密端点使用。
package model

import (
	"regexp"
	"time"
)

const (
	SecretTypeGeneric     = "GENERIC"
	SecretTypeApiKey      = "API_KEY"
	SecretTypeToken       = "TOKEN"
	SecretTypePassword    = "PASSWORD"
	SecretTypeCertificate = "CERTIFICATE"
	SecretTypeOAuth2      = "OAUTH2"
)

var (
	// SecretKeyRegex 仅允许字母、数字、下划线、中划线和点号，长度 1-64。
	SecretKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,64}$`)
)

// SysSecret 通用密钥存储持久化实体。
type SysSecret struct {
	ID             string    `gorm:"column:id;primaryKey;type:varchar(36)" json:"id"`
	TenantID       string    `gorm:"column:tenant_id;type:varchar(36);not null;default:''" json:"tenant_id"`
	Key            string    `gorm:"column:key;type:varchar(64);not null" json:"key"`
	Name           string    `gorm:"column:name;type:varchar(128);not null" json:"name"`
	SecretType     string    `gorm:"column:secret_type;type:varchar(32);not null;default:'GENERIC'" json:"secret_type"`
	Description    string    `gorm:"column:description;type:varchar(255);default:''" json:"description"`
	EncryptedValue string    `gorm:"column:encrypted_value;type:text;not null" json:"-"`
	MaskPreview    string    `gorm:"column:mask_preview;type:varchar(32);not null;default:'****'" json:"mask_preview"`
	KeyID          string    `gorm:"column:key_id;type:varchar(32);not null;default:''" json:"key_id"`
	CreatedAt      time.Time `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (SysSecret) TableName() string {
	return "sys_secrets"
}

// CreateSecretReq 创建密钥请求参数。
type CreateSecretReq struct {
	Key         string `json:"key" binding:"required"`
	Name        string `json:"name" binding:"required"`
	SecretType  string `json:"secret_type"`
	Description string `json:"description"`
	Value       string `json:"value" binding:"required"`
}

// UpdateSecretReq 更新密钥请求参数。
type UpdateSecretReq struct {
	Name        string `json:"name"`
	SecretType  string `json:"secret_type"`
	Description string `json:"description"`
	Value       string `json:"value"` // 可选，非空时更新并重新加密
}

// SecretListReq 列表分页与检索参数。
type SecretListReq struct {
	Page       int    `json:"page" form:"page"`
	PageSize   int    `json:"page_size" form:"page_size"`
	Query      string `json:"query" form:"query"`
	SecretType string `json:"secret_type" form:"secret_type"`
}

// SecretResp 安全脱敏出参。
type SecretResp struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	SecretType  string `json:"secret_type"`
	Description string `json:"description"`
	MaskPreview string `json:"mask_preview"`
	KeyID       string `json:"key_id"`
	NeedsReseal bool   `json:"needs_reseal"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// RevealSecretResp 明文解密出参（仅限安全审计解密端点）。
type RevealSecretResp struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SecretPageResult 密钥列表分页响应。
type SecretPageResult struct {
	List  []SecretResp `json:"list"`
	Total int64        `json:"total"`
}

