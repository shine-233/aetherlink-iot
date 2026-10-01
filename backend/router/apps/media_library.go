// 文件用途：媒体库（media_files，ROADMAP TB-41）路由定义。
// 核心逻辑：挂载 /api/v1/media/files 路由组并关联 MediaLibraryApi；
//
//	列表与详情/删除共用 files 前缀，:id 为参数段。
//
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约；
//
//	新增路由必须同步在迁移（129.sql）做 Casbin g2/p 登记，否则启动审计 fail-fast。
//
// 重构建议：若后续加引用明细/恢复站接口，按子资源再分组，不在本文件堆平铺路由。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// MediaLibraryRouter 媒体库路由组。
type MediaLibraryRouter struct{}

// InitMediaLibrary 注册媒体库管理路由。
func (*MediaLibraryRouter) InitMediaLibrary(Router *gin.RouterGroup) {
	r := Router.Group("media/files")
	{
		r.GET("", api.Controllers.MediaLibraryApi.ListMediaFiles)
		r.GET(":id", api.Controllers.MediaLibraryApi.GetMediaFileDetail)
		r.DELETE(":id", api.Controllers.MediaLibraryApi.DeleteMediaFile)
	}
}
