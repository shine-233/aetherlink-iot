// 文件用途：注册协议插件相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type ProtocolPlugin struct{}

func (*ProtocolPlugin) InitProtocolPlugin(Router *gin.RouterGroup) {
	protocolPluginApi := Router.Group("protocol_plugin")
	{
		// 根据协议类型和设备类型获取设备配置的配置表单
		protocolPluginApi.GET("config_form", api.Controllers.ProtocolPluginApi.HandleProtocolPluginFormByProtocolType)
	}
}
