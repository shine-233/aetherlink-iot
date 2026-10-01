// 文件用途：AI 集成域的 HTTP 入口（ROADMAP C4：自然语言查询遥测）。
// 边界说明：租户边界与意图钳制在 service 层处理，本层只做绑定、claims 提取和错误出口。
package api

import (
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type AiQueryApi struct{}

// QueryTelemetryByQuestion 自然语言查询设备遥测。
// POST /api/v1/ai/telemetry/query
func (*AiQueryApi) QueryTelemetryByQuestion(c *gin.Context) {
	Handle(c, func(req *service.AiTelemetryQueryReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.AiQuery.QueryTelemetry(c.Request.Context(), req, userClaims)
	})
}

// AnalyzeAlarm 告警根因分析（ROADMAP C4：AI 告警分析）。
// POST /api/v1/ai/alarm/analysis
func (*AiQueryApi) AnalyzeAlarm(c *gin.Context) {
	Handle(c, func(req *service.AiAlarmAnalysisReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.AiQuery.AnalyzeAlarm(c.Request.Context(), req, userClaims)
	})
}
