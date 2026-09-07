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
		g.PUT("", api.Controllers.ReportScheduleApi.Update)
		g.DELETE(":id", api.Controllers.ReportScheduleApi.Delete)
		g.GET("", api.Controllers.ReportScheduleApi.List)
		g.GET(":id", api.Controllers.ReportScheduleApi.Get)
		g.POST(":id/run", api.Controllers.ReportScheduleApi.RunNow)
	}
}
