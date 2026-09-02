// 文件用途：注册 OIDC/SSO 路由（ROADMAP C7 剩余）。
// 核心逻辑：提供方 CRUD 走 JWT 鉴权组；/sso/:id/start 与 /callback 为公开入口（router_init 注册）。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type OidcSso struct{}

func (*OidcSso) InitOidcProvider(Router *gin.RouterGroup) {
	oidcApi := api.Controllers.OidcSsoApi
	group := Router.Group("oidc/provider")
	{
		group.POST("", oidcApi.HandleOidcProviderCreate)
		group.GET("list", oidcApi.HandleOidcProviderList)
		group.PUT("", oidcApi.HandleOidcProviderUpdate)
		group.DELETE(":id", oidcApi.HandleOidcProviderDelete)
	}
}
