// 文件用途：注册实体版本控制（ROADMAP C7）相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载版本列表、快照创建、详情与恢复端点。
// 关键注意事项：路由位于 JWT+Casbin 中间件之后的 v1 组内；路径是前后端与自动化契约，
// 新增或调整必须同步 endpoint catalog 与 business capabilities 清单。
// 重构建议：若后续加入自动快照配置端点，挂在同一组内并保持复数资源命名。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// EntityVersion 实体版本控制路由组。
type EntityVersion struct{}

// InitEntityVersion 挂载 /entity_versions 路由。
func (*EntityVersion) InitEntityVersion(Router *gin.RouterGroup) {
	url := Router.Group("entity_versions")
	{
		url.GET("", api.Controllers.EntityVersionApi.HandleGetEntityVersionList)
		url.POST("", api.Controllers.EntityVersionApi.HandleCreateEntityVersion)
		url.GET(":id", api.Controllers.EntityVersionApi.HandleGetEntityVersion)
		url.POST(":id/restore", api.Controllers.EntityVersionApi.HandleRestoreEntityVersion)
	}
}
