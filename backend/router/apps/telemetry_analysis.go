// Telemetry analytics routes (ROADMAP P2.2).
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type TelemetryAnalysis struct{}

func (*TelemetryAnalysis) InitTelemetryAnalysis(Router *gin.RouterGroup) {
	url := Router.Group("telemetry/analysis")
	{
		url.POST("", api.Controllers.TelemetryAnalysisApi.AnalyzeTelemetry)
		url.POST("export", api.Controllers.TelemetryAnalysisApi.ExportTelemetryAnalysis)
		// P2.2 基础异常检测（bounds / deviation）。
		url.POST("anomaly", api.Controllers.TelemetryAnalysisApi.DetectTelemetryAnomalies)
	}
}
