// 文件用途：注册租户管理（P3 商业化）应用路由。
// 核心逻辑：挂载租户创建/列表/详情/更新路由。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Tenant struct{}

func (*Tenant) InitTenant(Router *gin.RouterGroup) {
	tenant := Router.Group("tenants")
	{
		tenant.POST("", api.Controllers.TenantApi.CreateTenant)
		tenant.GET("", api.Controllers.TenantApi.ListTenants)
		tenant.GET(":id", api.Controllers.TenantApi.GetTenant)
		tenant.PUT(":id", api.Controllers.TenantApi.UpdateTenant)
	}
}
