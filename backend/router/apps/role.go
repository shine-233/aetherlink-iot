// 文件用途：注册角色相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Role struct {
}

func (*Role) Init(Router *gin.RouterGroup) {
	Router.GET("permissions", api.Controllers.RoleApi.ListPermissions)

	role := Router.Group("role")
	{
		// 增
		role.POST("", api.Controllers.RoleApi.CreateRole)

		// 删
		role.DELETE(":id", api.Controllers.RoleApi.DeleteRole)

		// 改
		role.PUT("", api.Controllers.RoleApi.UpdateRole)

		// 查
		role.GET("", api.Controllers.RoleApi.HandleRoleListByPage)

		// 权限分配与用户关联
		role.GET(":id/permissions", api.Controllers.RoleApi.GetRolePermissions)
		role.POST(":id/permissions", api.Controllers.RoleApi.AssignRolePermissions)
		role.GET(":id/users", api.Controllers.RoleApi.GetRoleUsers)
		role.POST(":id/users", api.Controllers.RoleApi.AssignRoleUsers)
	}

	roles := Router.Group("roles")
	{
		roles.GET(":id/permissions", api.Controllers.RoleApi.GetRolePermissions)
		roles.POST(":id/permissions", api.Controllers.RoleApi.AssignRolePermissions)
		roles.GET(":id/users", api.Controllers.RoleApi.GetRoleUsers)
		roles.POST(":id/users", api.Controllers.RoleApi.AssignRoleUsers)
	}
}
