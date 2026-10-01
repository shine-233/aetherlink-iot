// 文件用途：部件库（widget_bundles，ROADMAP TB-04）路由定义。
// 核心逻辑：挂载 /api/v1/widget-bundles 路由组并关联 WidgetBundleApi；
//
//	builtin/seed 为静态段，gin 静态路由优先于 :id 参数段，二者可安全共存。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type WidgetBundleRouter struct{}

func (*WidgetBundleRouter) InitWidgetBundle(Router *gin.RouterGroup) {
	r := Router.Group("widget-bundles")
	{
		r.POST("", api.Controllers.WidgetBundleApi.CreateWidgetBundle)
		r.PUT("", api.Controllers.WidgetBundleApi.UpdateWidgetBundle)
		r.GET("", api.Controllers.WidgetBundleApi.ListWidgetBundles)
		r.GET(":id", api.Controllers.WidgetBundleApi.GetWidgetBundleByID)
		r.DELETE(":id", api.Controllers.WidgetBundleApi.DeleteWidgetBundle)
		r.GET("builtin", api.Controllers.WidgetBundleApi.GetBuiltinWidgetBundle)
		r.POST("seed", api.Controllers.WidgetBundleApi.SeedBuiltinWidgetBundle)
	}
}
