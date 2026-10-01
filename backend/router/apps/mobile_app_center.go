// 文件用途：移动应用中心（mobile_app_bundles，ROADMAP TB-23）路由定义。
// 核心逻辑：挂载 /api/v1/mobile/app_bundles 路由组并关联 MobileAppBundleApi：
//
//	upload（上传登记）、列表/详情/更新/删除（CRUD）、publish/archive（状态机）、download（文件流）。
//
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约；
//
//	新增路由必须同步在迁移（136.sql）做 Casbin g2/p 登记，否则启动审计 fail-fast；
//	路由组挂在 CasbinRBAC 中间件之后（router_init.go），路由面受资源表保护。
//
// 重构建议：uniapp 对接阶段若开放终端拉取面（published 专用下载），另立子组并
//
//	单独评审授权，不在本管理面路由上加角色分支。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// MobileAppBundleRouter 移动应用中心路由组。
type MobileAppBundleRouter struct{}

// InitMobileAppBundle 注册移动应用中心管理路由。
func (*MobileAppBundleRouter) InitMobileAppBundle(Router *gin.RouterGroup) {
	r := Router.Group("mobile/app_bundles")
	{
		r.POST("upload", api.Controllers.MobileAppBundleApi.UploadAppBundle)
		r.GET("", api.Controllers.MobileAppBundleApi.ListAppBundles)
		r.GET(":id", api.Controllers.MobileAppBundleApi.GetAppBundle)
		r.PUT(":id", api.Controllers.MobileAppBundleApi.UpdateAppBundle)
		r.DELETE(":id", api.Controllers.MobileAppBundleApi.DeleteAppBundle)
		r.POST(":id/publish", api.Controllers.MobileAppBundleApi.PublishAppBundle)
		r.POST(":id/archive", api.Controllers.MobileAppBundleApi.ArchiveAppBundle)
		r.GET(":id/download", api.Controllers.MobileAppBundleApi.DownloadAppBundle)
	}
}
