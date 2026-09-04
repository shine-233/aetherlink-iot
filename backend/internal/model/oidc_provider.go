// 文件用途：OIDC/SSO（ROADMAP C7 剩余）模型——租户级 IdP 配置。
package model

import "time"

// OidcProvider 租户级外部 IdP 配置。tenant_id 空串=平台级提供方。
type OidcProvider struct {
	ID               string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID         string     `gorm:"column:tenant_id;not null;default:''" json:"tenant_id"`
	Name             string     `gorm:"column:name;not null" json:"name"`
	Issuer           string     `gorm:"column:issuer;not null" json:"issuer"`
	ClientID         string     `gorm:"column:client_id;not null" json:"client_id"`
	ClientSecret     string     `gorm:"column:client_secret;not null" json:"-"`
	DiscoveryURL     string     `gorm:"column:discovery_url;not null" json:"discovery_url"`
	Scopes           string     `gorm:"column:scopes;not null" json:"scopes"`
	FrontendRedirect string     `gorm:"column:frontend_redirect;not null" json:"frontend_redirect"`
	Enabled          bool       `gorm:"column:enabled;not null" json:"enabled"`
	CreatedAt        *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName OidcProvider's table name
func (*OidcProvider) TableName() string { return "tenant_oidc_providers" }
