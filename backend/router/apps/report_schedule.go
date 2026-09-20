package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// ReportSchedule 定时报表路由组（ROADMAP D3）。
type ReportSchedule struct{}

func (*ReportSchedule) InitReportSchedule(Router *gin.RouterGroup) {
	g := Router.Group("report/schedules")
	{
		g.POST("", api.Controllers.ReportScheduleApi.Create)
		g.GET("", api.Controllers.ReportScheduleApi.List)
		g.GET(":id", api.Controllers.ReportScheduleApi.Get)
		g.PUT(":id", api.Controllers.ReportScheduleApi.Update)
		g.DELETE(":id", api.Controllers.ReportScheduleApi.Delete)
		g.POST(":id/run", api.Controllers.ReportScheduleApi.RunNow)
		g.GET(":id/runs", api.Controllers.ReportScheduleApi.ListRuns)
		g.GET(":id/runs/:run_id", api.Controllers.ReportScheduleApi.GetRun)
		g.POST(":id/runs/:run_id/retry", api.Controllers.ReportScheduleApi.RetryRun)
	}
}
