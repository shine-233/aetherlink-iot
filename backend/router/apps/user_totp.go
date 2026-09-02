// 文件用途：注册 2FA（ROADMAP C7）路由。
// 核心逻辑：绑定/状态管理走 JWT 鉴权组；第二因子登录为公开端点（router_init 注册）。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type UserTOTP struct{}

func (*UserTOTP) InitUserTOTP(Router *gin.RouterGroup) {
	totpApi := api.Controllers.UserTotpApi
	group := Router.Group("user/totp")
	{
		group.GET("setup", totpApi.HandleTotpSetup)
		group.POST("activate", totpApi.HandleTotpActivate)
		group.POST("disable", totpApi.HandleTotpDisable)
		group.GET("status", totpApi.HandleTotpStatus)
	}
}
