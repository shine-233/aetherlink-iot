// 文件用途：注册Casbin 权限相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Casbin struct{}

func (*Casbin) Init(Router *gin.RouterGroup) {
	url := Router.Group("casbin")
	{
		//角色-功能
		url.POST("function", api.Controllers.CasbinApi.AddFunctionToRole)
		url.DELETE("function/:id", api.Controllers.CasbinApi.DeleteFunctionFromRole)
		url.PUT("function", api.Controllers.CasbinApi.UpdateFunctionFromRole)
		url.GET("function", api.Controllers.CasbinApi.HandleFunctionFromRole)

		//角色-用户
		url.POST("user", api.Controllers.CasbinApi.AddRoleToUser)
		url.DELETE("user/:id", api.Controllers.CasbinApi.DeleteRolesFromUser)
		url.PUT("user", api.Controllers.CasbinApi.UpdateRolesFromUser)
		url.GET("user", api.Controllers.CasbinApi.HandleRolesFromUser)
	}
}
