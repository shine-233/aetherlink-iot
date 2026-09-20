// 文件用途：注册 SCADA / Widget 基础层的应用路由（ROADMAP P1.3）。
// 核心逻辑：在 Gin 路由组上挂载项目、画布文档与控制命令的 URL、方法和处理器。
// 关键注意事项：路由路径与方法构成前端与自动化接口的契约，改动需同步更新调用方；
// 控制相关路由即使服务未接线也保留注册，由处理器统一 fail closed，
// 这样"能力缺失"会以明确错误暴露，而不是以 404 让人以为路径拼错了。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Scada struct{}

func (*Scada) Init(Router *gin.RouterGroup) {
	url := Router.Group("scada")
	{
		// 项目（此前无"项目"这一层，项目 CRUD 只能返回 unsupported）
		url.POST("projects", api.Controllers.ScadaApi.CreateScadaProject)
		url.GET("projects", api.Controllers.ScadaApi.ListScadaProjects)
		url.GET("projects/:id", api.Controllers.ScadaApi.GetScadaProject)
		url.DELETE("projects/:id", api.Controllers.ScadaApi.DeleteScadaProject)

		// 画布文档。项目级子资源必须沿用与 projects/:id 相同的通配符名 :id：
		// Gin 不允许同一层级出现两个不同名通配符（:projectId 会与 :id 冲突），
		// 冲突时注册阶段直接 panic，等于后端启动即崩。
		url.POST("projects/:id/documents", api.Controllers.ScadaApi.CreateScadaDocument)
		url.GET("projects/:id/documents", api.Controllers.ScadaApi.ListScadaDocuments)
		url.GET("documents/:id", api.Controllers.ScadaApi.GetScadaDocument)
		url.PUT("documents/:id", api.Controllers.ScadaApi.SaveScadaDocument)
		url.POST("documents/:id/publish", api.Controllers.ScadaApi.PublishScadaDocument)
		url.POST("documents/:id/rollback", api.Controllers.ScadaApi.RollbackScadaDocument)
		url.POST("documents/:id/archive", api.Controllers.ScadaApi.ArchiveScadaDocument)
		url.GET("documents/:id/versions", api.Controllers.ScadaApi.ListScadaDocumentVersions)
		url.GET("documents/:id/audits", api.Controllers.ScadaApi.ListScadaControlAudits)

		// 实时控制
		url.POST("control/confirm", api.Controllers.ScadaApi.IssueControlConfirmation)
		url.POST("control", api.Controllers.ScadaApi.ExecuteControl)
	}
}
