// 文件用途：注册RDI相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type RDI struct{}

func (*RDI) InitRDI(Router *gin.RouterGroup) {
	rdiapi := Router.Group("rdi")
	{
		rdiapi.GET("thing-model", api.Controllers.RDIApi.ThingModel)
		rdiapi.POST("devices/activate", api.Controllers.RDIApi.ActivateDevice)
		rdiapi.GET("devices/:device_id/config", api.Controllers.RDIApi.DeviceConfig)
		rdiapi.GET("devices/:device_id/history", api.Controllers.RDIApi.DeviceHistory)
		rdiapi.GET("devices/:device_id/latest-firmware", api.Controllers.RDIApi.LatestFirmware)
		rdiapi.PUT("devices/:device_id/config", api.Controllers.RDIApi.UpdateDeviceConfig)
		rdiapi.POST("devices/:device_id/commands", api.Controllers.RDIApi.SendCommand)
		rdiapi.POST("devices/:device_id/share-token", api.Controllers.RDIApi.CreateShareToken)
		rdiapi.DELETE("devices/:device_id/share-tokens/:token", api.Controllers.RDIApi.RevokeShareToken)
		rdiapi.DELETE("devices/:device_id/share-recipients/:user_id", api.Controllers.RDIApi.RevokeSharedDeviceRecipient)
		rdiapi.POST("share-tokens/:token/accept", api.Controllers.RDIApi.AcceptSharedDevice)
		rdiapi.GET("shared-with-me/devices", api.Controllers.RDIApi.SharedDevices)
	}
}
