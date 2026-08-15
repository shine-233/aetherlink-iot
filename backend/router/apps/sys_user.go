// 文件用途：注册系统用户相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type User struct {
}

func (*User) InitUser(Router *gin.RouterGroup) {
	userapi := Router.Group("user")
	{
		// 个人信息管理
		userapi.GET("detail", api.Controllers.UserApi.HandleUserDetail)
		userapi.PUT("update", api.Controllers.UserApi.UpdateUsers)
		userapi.POST("change-email", api.Controllers.UserApi.ChangeEmail)
		userapi.GET("warning-email", api.Controllers.UserApi.GetWarningEmails)
		userapi.PUT("warning-email", api.Controllers.UserApi.UpdateWarningEmails)
		userapi.POST("prefer-lang", api.Controllers.UserApi.UpdatePreferredLanguage)
		userapi.PUT("prefer-lang", api.Controllers.UserApi.UpdatePreferredLanguage)
		userapi.GET("logout", api.Controllers.UserApi.Logout)
		userapi.GET("refresh", api.Controllers.UserApi.RefreshToken)

		// 用户管理
		userapi.GET("", api.Controllers.UserApi.HandleUserListByPage)
		userapi.POST("", api.Controllers.UserApi.CreateUser)
		userapi.PUT("", api.Controllers.UserApi.UpdateUser)
		userapi.DELETE(":id", api.Controllers.UserApi.DeleteUser)
		userapi.GET(":id", api.Controllers.UserApi.HandleUser)
		userapi.POST("transform", api.Controllers.UserApi.TransformUser)

		// 用户地址管理
		userapi.PUT("address/:id", api.Controllers.UserApi.UpdateUserAddress)

		// 获取租户ID
		userapi.GET("/tenant/id", api.Controllers.UserApi.GetTenantID)

		// 用户选择器
		userapi.GET("/selector", api.Controllers.UserApi.GetUserSelector)

	}
}
