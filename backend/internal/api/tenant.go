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
	Handle(c, func(req *model.CreateTenantReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Tenant.CreateTenant(c.Request.Context(), req, claims)
	})
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
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Tenant.GetTenant(c.Request.Context(), id, claims)
	})
}

// UpdateTenant 更新租户基本信息。
// @Summary  更新租户
// @Tags     Tenant
// @Router   /api/v1/tenants/{id} [put]
func (*TenantApi) UpdateTenant(c *gin.Context) {
	HandlePathBody(c, "id", func(id string, req *model.UpdateTenantReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Tenant.UpdateTenant(c.Request.Context(), id, req, claims)
	})
}

// SelfProvisionTenant 客户自助开通开箱入驻（公开路由，无需认证）。
// @Summary  客户自助开通租户入驻
// @Tags     Tenant
// @Router   /api/v1/tenant/provision [post]
func (*TenantApi) SelfProvisionTenant(c *gin.Context) {
	HandlePublic(c, func(req *model.SelfProvisionTenantReq) (interface{}, error) {
		return service.GroupApp.Tenant.SelfServiceProvisionTenant(c.Request.Context(), req)
	})
}
