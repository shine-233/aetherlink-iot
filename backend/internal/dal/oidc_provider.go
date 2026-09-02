// 文件用途：OIDC 提供方 DAL（ROADMAP C7 剩余）。
// 核心逻辑：租户管理员对自有 IdP 配置 CRUD；公开 SSO 入口按 id 读取启用中的配置。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateOidcProvider 新建提供方。
func CreateOidcProvider(p *model.OidcProvider) error {
	return global.DB.Create(p).Error
}

// tenant-scope: reviewed-2026-09-02 all-tenants semantics (public SSO entry); provider→tenant ownership enforced by session issuer user binding.
// GetOidcProviderByID 按 id 读取（供 SSO 公开入口），不限租户但需 enabled。
func GetOidcProviderByID(id string) (*model.OidcProvider, error) {
	var p model.OidcProvider
	err := global.DB.Where("id = ? AND enabled = ?", id, true).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetOidcProviderOwned 租户管理员读取自己名下提供方（含未启用）。
func GetOidcProviderOwned(id, tenantID string) (*model.OidcProvider, error) {
	var p model.OidcProvider
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListOidcProvidersByTenant 列出租户提供方（tenant_id 精确匹配；空=平台级）。
func ListOidcProvidersByTenant(tenantID string) ([]*model.OidcProvider, error) {
	var list []*model.OidcProvider
	err := global.DB.Where("tenant_id = ?", tenantID).Order("created_at ASC").Find(&list).Error
	return list, err
}

// UpdateOidcProvider 更新提供方（租户维度守卫）。
func UpdateOidcProvider(p *model.OidcProvider) (bool, error) {
	res := global.DB.Model(&model.OidcProvider{}).
		Where("id = ? AND tenant_id = ?", p.ID, p.TenantID).
		Updates(map[string]interface{}{
			"name":              p.Name,
			"issuer":            p.Issuer,
			"client_id":         p.ClientID,
			"client_secret":     p.ClientSecret,
			"discovery_url":     p.DiscoveryURL,
			"scopes":            p.Scopes,
			"frontend_redirect": p.FrontendRedirect,
			"enabled":           p.Enabled,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeleteOidcProvider 删除提供方（租户维度守卫）。
func DeleteOidcProvider(id, tenantID string) error {
	res := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.OidcProvider{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
