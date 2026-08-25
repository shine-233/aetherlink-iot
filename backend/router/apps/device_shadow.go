// 文件用途：注册设备影子消息相关的应用路由（ROADMAP A3）。
// 核心逻辑：在 Gin 路由组上挂载影子消息的查询、设置与取消端点。
// 关键注意事项：路径以 deviceId 为资源锚点，鉴权由 v1 组中间件保证。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type DeviceShadow struct{}

func (*DeviceShadow) InitDeviceShadow(Router *gin.RouterGroup) {
	shadowApi := Router.Group("device/shadow")
	{
		shadowApi.GET(":deviceId", api.Controllers.DeviceShadowApi.HandleShadowMessageList)
		shadowApi.POST(":deviceId", api.Controllers.DeviceShadowApi.SetShadowMessage)
		shadowApi.DELETE(":deviceId/:msgId", api.Controllers.DeviceShadowApi.CancelShadowMessage)
	}
}
