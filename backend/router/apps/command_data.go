// Command data application routes.
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type CommandData struct{}

func (*CommandData) InitCommandData(Router *gin.RouterGroup) {
	commandDataApi := Router.Group("command/datas")
	{
		commandDataApi.GET("set/logs", api.Controllers.CommandSetLogApi.ServeSetLogsDataListByPage)

		commandDataApi.POST("pub", api.Controllers.CommandSetLogApi.CommandPutMessage)
		commandDataApi.POST("direct-method", api.Controllers.CommandSetLogApi.InvokeDirectMethod)

		commandDataApi.POST("jobs/preview", api.Controllers.CommandSetLogApi.PreviewFleetCommandJob)
		commandDataApi.POST("jobs/submit", api.Controllers.CommandSetLogApi.SubmitFleetCommandJob)
		commandDataApi.GET("jobs", api.Controllers.CommandSetLogApi.ListFleetCommandJobs)
		commandDataApi.GET("jobs/:job_id/support-bundle", api.Controllers.CommandSetLogApi.GetFleetCommandJobSupportBundle)
		commandDataApi.GET("jobs/:job_id/rows", api.Controllers.CommandSetLogApi.GetFleetCommandJobRows)
		commandDataApi.GET("jobs/:job_id/report", api.Controllers.CommandSetLogApi.GetFleetCommandJobReport)
		commandDataApi.GET("jobs/:job_id", api.Controllers.CommandSetLogApi.GetFleetCommandJob)
		commandDataApi.POST("jobs/:job_id/cancel", api.Controllers.CommandSetLogApi.CancelFleetCommandJob)
		commandDataApi.POST("jobs/:job_id/retry", api.Controllers.CommandSetLogApi.RetryFleetCommandJob)
		// P0.3 批次生命周期：暂停/恢复/回滚/进度消费。
		// 这四个能力此前只在服务层有实现与单测，没有任何 HTTP 入口，等于运维与
		// 网关都调不到——这里补端点使能力可达；Casbin 资源见 94.sql。
		commandDataApi.POST("jobs/:job_id/pause", api.Controllers.CommandSetLogApi.PauseFleetCommandJob)
		commandDataApi.POST("jobs/:job_id/resume", api.Controllers.CommandSetLogApi.ResumeFleetCommandJob)
		commandDataApi.POST("jobs/:job_id/rollback", api.Controllers.CommandSetLogApi.RollbackFleetCommandJob)
		commandDataApi.POST("jobs/:job_id/progress", api.Controllers.CommandSetLogApi.ConsumeFleetCommandJobProgress)

		commandDataApi.POST("saved-filters", api.Controllers.FleetSavedFilterApi.CreateFleetSavedFilter)
		commandDataApi.GET("saved-filters", api.Controllers.FleetSavedFilterApi.ListFleetSavedFilters)
		commandDataApi.PUT("saved-filters/:filter_id", api.Controllers.FleetSavedFilterApi.UpdateFleetSavedFilter)
		commandDataApi.DELETE("saved-filters/:filter_id", api.Controllers.FleetSavedFilterApi.DeleteFleetSavedFilter)

		commandDataApi.GET("delivery/diagnostics/:device_id", api.Controllers.CommandSetLogApi.GetCommandDeliveryDiagnostics)

		commandDataApi.GET(":id", api.Controllers.CommandSetLogApi.HandleCommandList)
	}
}
