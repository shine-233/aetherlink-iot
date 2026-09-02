// 文件用途：租户客户层级（ROADMAP C2）HTTP 处理器。
// 核心逻辑：租户树的查询与租户登记的增删改查，全部委托 service.GroupApp.Tenant 完成；
//           本层只做绑定、claims 提取与响应回写，权限边界（可管辖子树）由 service 守卫把关。
// 使用注意：租户管理涉及组织架构数据，修改类接口依赖 JWT + Casbin 用户态；
//           任何返回的租户 ID 都可能被用于后续数据范围查询，务必走统一守卫。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type TenantApi struct{}

// GetTenantTree 返回当前管理员可管辖的租户树。
func (*TenantApi) GetTenantTree(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	tree, err := service.GroupApp.Tenant.GetTenantTree(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", tree)
}

// CreateTenant 创建租户登记并挂入层级。
func (*TenantApi) CreateTenant(c *gin.Context) {
	var req model.CreateTenantReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	created, err := service.GroupApp.Tenant.CreateTenant(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", created)
}

// UpdateTenant 更新租户名称/编码/状态/备注。
func (*TenantApi) UpdateTenant(c *gin.Context) {
	var req model.UpdateTenantReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.Tenant.UpdateTenant(c.Param("id"), &req, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"id": c.Param("id")})
}

// DeleteTenant 删除叶子租户登记。
func (*TenantApi) DeleteTenant(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.Tenant.DeleteTenant(c.Param("id"), userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"id": c.Param("id")})
}

// GetTenantDetail 返回租户详情（含父租户名称与子租户数）。
func (*TenantApi) GetTenantDetail(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	detail, err := service.GroupApp.Tenant.GetTenantDetail(c.Param("id"), userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", detail)
}