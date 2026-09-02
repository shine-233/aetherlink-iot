// 文件用途：注册 AI 集成相关的应用路由（ROADMAP C4：遥测查询 + 告警分析）。
// 核心逻辑：在 Gin 路由组上挂载自然语言遥测查询与告警根因分析端点，鉴权由 v1 组中间件保证。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type AiQuery struct{}

func (*AiQuery) InitAiQuery(Router *gin.RouterGroup) {
	aiApi := Router.Group("ai")
	{
		aiApi.POST("telemetry/query", api.Controllers.AiQueryApi.QueryTelemetryByQuestion)
		aiApi.POST("alarm/analysis", api.Controllers.AiQueryApi.AnalyzeAlarm)
	}
}
