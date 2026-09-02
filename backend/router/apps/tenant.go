// 文件用途：注册租户客户层级（ROADMAP C2）应用路由。
// 核心逻辑：在 /api/v1/tenant 分组挂载租户树/创建/更新/删除/详情处理器；
//           tenant 前缀与用户模块的 /api/v1/user/tenant/id 无冲突（路径段不同）。
// 关键注意事项：所有方法均在 JWT + Casbin 保护段内挂载；可见范围由 service 守卫决定。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Tenant struct {
}

func (*Tenant) InitTenant(Router *gin.RouterGroup) {
	tenantapi := Router.Group("tenant")
	{
		tenantapi.GET("tree", api.Controllers.TenantApi.GetTenantTree)
		tenantapi.GET(":id", api.Controllers.TenantApi.GetTenantDetail)
		tenantapi.POST("", api.Controllers.TenantApi.CreateTenant)
		tenantapi.PUT(":id", api.Controllers.TenantApi.UpdateTenant)
		tenantapi.DELETE(":id", api.Controllers.TenantApi.DeleteTenant)
	}
}