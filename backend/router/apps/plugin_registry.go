// 文件用途：注册插件管理路由（PHASE-D-D9）。
// 核心逻辑：登记 CRUD/启停/凭证轮换/下行命令；鉴权由 v1 组中间件 + casbin 保证。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// PluginRegistry 插件管理路由组。
type PluginRegistry struct{}

func (*PluginRegistry) InitPluginRegistry(Router *gin.RouterGroup) {
	plugins := Router.Group("plugins")
	{
		pluginApi := api.Controllers.PluginRegistryApi
		plugins.POST("", pluginApi.HandleCreatePlugin)
		plugins.GET("", pluginApi.HandleListPlugins)
		plugins.DELETE(":id", pluginApi.HandleDeletePlugin)
		plugins.PUT(":id/enable", pluginApi.HandleEnablePlugin)
		plugins.PUT(":id/disable", pluginApi.HandleDisablePlugin)
		plugins.PUT(":id/token", pluginApi.HandleRotatePluginToken)
		plugins.POST(":id/downlink", pluginApi.HandlePluginDownlink)
	}
}
