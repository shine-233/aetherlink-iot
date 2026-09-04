// 文件用途：OIDC/SSO HTTP 入口（ROADMAP C7 剩余）。
// 边界说明：IdP 配置 CRUD 走 JWT+Casbin 组；start/callback 为公开端点。
package api

import (
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type OidcSsoApi struct{}

// HandleOidcProviderCreate 新建租户 IdP。
// POST /api/v1/oidc/provider
func (*OidcSsoApi) HandleOidcProviderCreate(c *gin.Context) {
	var req service.OidcProviderReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.OidcSso.Create(userClaims, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleOidcProviderList 列出当前租户 IdP。
// GET /api/v1/oidc/provider/list
func (*OidcSsoApi) HandleOidcProviderList(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.OidcSso.List(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleOidcProviderUpdate 更新租户 IdP。
// PUT /api/v1/oidc/provider
func (*OidcSsoApi) HandleOidcProviderUpdate(c *gin.Context) {
	var req service.OidcProviderReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.OidcSso.Update(userClaims, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleOidcProviderDelete 删除租户 IdP。
// DELETE /api/v1/oidc/provider/:id
func (*OidcSsoApi) HandleOidcProviderDelete(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.OidcSso.Delete(userClaims, id); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

// HandleOidcPublicProviders 登录页 SSO 提供方发现（公开，仅平台级）。
// GET /api/v1/sso/providers
func (*OidcSsoApi) HandleOidcPublicProviders(c *gin.Context) {
	resp, err := service.GroupApp.OidcSso.ListPublic()
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleSSOStart SSO 入口：302 到 IdP（公开）。
// GET /api/v1/sso/:id/start
func (*OidcSsoApi) HandleSSOStart(c *gin.Context) {
	service.GroupApp.OidcSso.HandleSSO(c)
}

// HandleSSOCallback IdP 回调：验签并落地本地会话（公开）。
// GET /api/v1/sso/:id/callback
func (*OidcSsoApi) HandleSSOCallback(c *gin.Context) {
	service.GroupApp.OidcSso.HandleSSO(c)
}
