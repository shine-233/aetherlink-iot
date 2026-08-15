// 文件用途：注册通知服务配置相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type NotificationServicesConfig struct{}

func (*NotificationServicesConfig) Init(Router *gin.RouterGroup) {
	url := Router.Group("notification/services/config")
	{
		// 创建/修改
		url.POST("", api.Controllers.NotificationServicesConfigApi.SaveNotificationServicesConfig)

		// 查询
		url.GET(":type", api.Controllers.NotificationServicesConfigApi.HandleNotificationServicesConfig)

		// 调试
		url.POST("e-mail/test", api.Controllers.NotificationServicesConfigApi.SendTestEmail)
	}

	templates := Router.Group("notification/e-mail/templates")
	{
		templates.GET("", api.Controllers.NotificationServicesConfigApi.ListEmailTemplates)
		templates.POST("", api.Controllers.NotificationServicesConfigApi.CreateEmailTemplate)
		templates.POST("preview", api.Controllers.NotificationServicesConfigApi.PreviewEmailTemplate)
		templates.PUT(":id", api.Controllers.NotificationServicesConfigApi.UpdateEmailTemplate)
		templates.DELETE(":id", api.Controllers.NotificationServicesConfigApi.DeleteEmailTemplate)
		templates.POST(":id/default", api.Controllers.NotificationServicesConfigApi.SetDefaultEmailTemplate)
	}
}
