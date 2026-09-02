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
	var req service.AiTelemetryQueryReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.AiQuery.QueryTelemetry(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// AnalyzeAlarm 告警根因分析（ROADMAP C4：AI 告警分析）。
// POST /api/v1/ai/alarm/analysis
func (*AiQueryApi) AnalyzeAlarm(c *gin.Context) {
	var req service.AiAlarmAnalysisReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.AiQuery.AnalyzeAlarm(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
