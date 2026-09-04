// 文件用途：注册资产相关路由（ROADMAP C2）。
// 核心逻辑：租户内 CRUD + 树查询；鉴权由 v1 组中间件保证。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Asset struct{}

func (*Asset) InitAsset(Router *gin.RouterGroup) {
	assetApi := api.Controllers.AssetApi
	assets := Router.Group("asset")
	{
		assets.POST("", assetApi.HandleAssetCreate)
		assets.PUT("", assetApi.HandleAssetUpdate)
		assets.DELETE(":id", assetApi.HandleAssetDelete)
		assets.GET("list", assetApi.HandleAssetList)
		assets.GET("tree", assetApi.HandleAssetTree)
		assets.GET(":id", assetApi.HandleAssetGet)
	}
}
