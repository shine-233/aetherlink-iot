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
		commandDataApi.GET("jobs/:job_id", api.Controllers.CommandSetLogApi.GetFleetCommandJob)
		commandDataApi.POST("jobs/:job_id/cancel", api.Controllers.CommandSetLogApi.CancelFleetCommandJob)
		commandDataApi.POST("jobs/:job_id/retry", api.Controllers.CommandSetLogApi.RetryFleetCommandJob)

		commandDataApi.POST("saved-filters", api.Controllers.FleetSavedFilterApi.CreateFleetSavedFilter)
		commandDataApi.GET("saved-filters", api.Controllers.FleetSavedFilterApi.ListFleetSavedFilters)
		commandDataApi.PUT("saved-filters/:filter_id", api.Controllers.FleetSavedFilterApi.UpdateFleetSavedFilter)
		commandDataApi.DELETE("saved-filters/:filter_id", api.Controllers.FleetSavedFilterApi.DeleteFleetSavedFilter)

		commandDataApi.GET("delivery/diagnostics/:device_id", api.Controllers.CommandSetLogApi.GetCommandDeliveryDiagnostics)

		commandDataApi.GET(":id", api.Controllers.CommandSetLogApi.HandleCommandList)
	}
}
