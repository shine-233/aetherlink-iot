// 文件用途：租户管理与客户自助开通 HTTP 接口层。
package api

import (
	"strconv"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type TenantApi struct{}

// CreateTenant 创建新租户。
// @Summary  创建新租户
// @Tags     Tenant
// @Router   /api/v1/tenants [post]
func (*TenantApi) CreateTenant(c *gin.Context) {
	var req model.CreateTenantReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.Tenant.CreateTenant(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// ListTenants 租户分页列表。
// @Summary  租户分页列表
// @Tags     Tenant
// @Router   /api/v1/tenants [get]
func (*TenantApi) ListTenants(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	search := c.Query("search")
	claims := c.MustGet("claims").(*utils.UserClaims)
	list, total, err := service.GroupApp.Tenant.ListTenants(c.Request.Context(), page, pageSize, search, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetTenant 获取租户详情。
// @Summary  获取租户详情
// @Tags     Tenant
// @Router   /api/v1/tenants/{id} [get]
func (*TenantApi) GetTenant(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.Tenant.GetTenant(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// UpdateTenant 更新租户基本信息。
// @Summary  更新租户
// @Tags     Tenant
// @Router   /api/v1/tenants/{id} [put]
func (*TenantApi) UpdateTenant(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateTenantReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.Tenant.UpdateTenant(c.Request.Context(), id, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// SelfProvisionTenant 客户自助开通开箱入驻（公开路由，无需认证）。
// @Summary  客户自助开通租户入驻
// @Tags     Tenant
// @Router   /api/v1/tenant/provision [post]
func (*TenantApi) SelfProvisionTenant(c *gin.Context) {
	var req model.SelfProvisionTenantReq
	if !BindAndValidate(c, &req) {
		return
	}
	resp, err := service.GroupApp.Tenant.SelfServiceProvisionTenant(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
