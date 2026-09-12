// 文件用途：注册移动端的应用路由（ROADMAP P1.4）。
// 核心逻辑：在 Gin 路由组上挂载能力矩阵、推送登记与幂等命令下发的 URL、方法和处理器。
// 关键注意事项：路由保留注册即使后端能力未接线——未接线由处理器返回明确错误，
// 而 404 会让人误以为路径写错了，把"能力缺失"伪装成"接口不存在"。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Mobile struct{}

func (*Mobile) Init(Router *gin.RouterGroup) {
	url := Router.Group("mobile")
	{
		url.GET("capabilities", api.Controllers.MobileApi.GetMobileCapabilities)
		url.POST("push/subscribe", api.Controllers.MobileApi.SubscribePush)
		url.DELETE("push/:id", api.Controllers.MobileApi.UnsubscribePush)
		url.POST("commands", api.Controllers.MobileApi.SendMobileCommand)

		// 设备 / 告警 / 影子。全部由 claims 推导可见范围，接口不接受 tenant_id 入参——
		// 允许调用方指定租户等于把归属过滤的开关交出去。
		url.GET("devices", api.Controllers.MobileApi.ListMobileDevices)
		url.GET("alarms", api.Controllers.MobileApi.ListMobileAlarms)
		url.POST("alarms/:id/ack", api.Controllers.MobileApi.AcknowledgeMobileAlarm)
		url.GET("devices/:id/shadow", api.Controllers.MobileApi.GetMobileShadow)
		url.PUT("devices/:id/shadow", api.Controllers.MobileApi.UpdateMobileShadow)
		url.GET("devices/:id/ota", api.Controllers.MobileApi.GetMobileOTAStatus)
		url.GET("dashboards", api.Controllers.MobileApi.ListMobileDashboards)
	}
}
