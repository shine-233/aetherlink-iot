// 文件用途：注册资源中心（TP-5）应用路由。
// 核心逻辑：挂载分类目录、综合列表、统一资源包导出/导入与模板应用路由。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type ResourceCenter struct{}

func (*ResourceCenter) InitResourceCenter(Router *gin.RouterGroup) {
	rc := Router.Group("resource/center")
	{
		// 分类目录
		rc.GET("catalog", api.Controllers.ResourceCenterApi.HandleResourceCenterCatalog)

		// 跨类型综合检索
		rc.GET("list", api.Controllers.ResourceCenterApi.HandleResourceCenterList)

		// 统一打包导出
		rc.GET("bundle", api.Controllers.ResourceCenterApi.HandleExportResourceBundle)

		// 统一资源包导入与冲突预览
		rc.POST("bundle/import", api.Controllers.ResourceCenterApi.HandleImportResourceBundle)

		// 一键应用安装
		rc.POST("apply", api.Controllers.ResourceCenterApi.HandleApplyResource)
	}
}
