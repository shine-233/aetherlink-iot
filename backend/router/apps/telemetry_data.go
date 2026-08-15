package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type TelemetryData struct{}

func (*TelemetryData) InitTelemetryData(Router *gin.RouterGroup) {
	telemetrydataapi := Router.Group("telemetry/datas")
	{
		telemetrydataapi.GET("current/:id", api.Controllers.TelemetryDataApi.HandleCurrentData)
		telemetrydataapi.GET("/current/keys", api.Controllers.TelemetryDataApi.HandleCurrentDataKeys)
		telemetrydataapi.GET("current/detail/:id", api.Controllers.TelemetryDataApi.ServeCurrentDetailData)

		telemetrydataapi.GET("history", api.Controllers.TelemetryDataApi.ServeHistoryData)
		telemetrydataapi.GET("history/pagination", api.Controllers.TelemetryDataApi.ServeHistoryDataByPage)
		telemetrydataapi.GET("history/page", api.Controllers.TelemetryDataApi.ServeHistoryDataByPage)
		telemetrydataapi.DELETE("", api.Controllers.TelemetryDataApi.DeleteData)

		telemetrydataapi.GET("statistic", api.Controllers.TelemetryDataApi.ServeStatisticData)
		telemetrydataapi.GET("statistic/batch", api.Controllers.TelemetryDataApi.ServeStatisticDataByDeviceId)
		telemetrydataapi.GET("set/logs", api.Controllers.TelemetryDataApi.ServeSetLogsDataListByPage)

		telemetrydataapi.GET("dead-letters", api.Controllers.TelemetryDataApi.ServeDeadLetterList)
		telemetrydataapi.POST("dead-letters/drain", api.Controllers.TelemetryDataApi.DrainDeadLetters)
		telemetrydataapi.PATCH("dead-letters/:id/status", api.Controllers.TelemetryDataApi.UpdateDeadLetterStatus)

		telemetrydataapi.GET("uplink-dead-letters", api.Controllers.TelemetryDataApi.ServeAttributeEventDeadLetterList)
		telemetrydataapi.POST("uplink-dead-letters/drain", api.Controllers.TelemetryDataApi.DrainAttributeEventDeadLetters)
		telemetrydataapi.PATCH("uplink-dead-letters/:id/status", api.Controllers.TelemetryDataApi.UpdateAttributeEventDeadLetterStatus)

		telemetrydataapi.POST("pub", api.Controllers.TelemetryDataApi.TelemetryPutMessage)
		telemetrydataapi.GET("simulation", api.Controllers.TelemetryDataApi.ServeEchoData)
		telemetrydataapi.POST("simulation", api.Controllers.TelemetryDataApi.SimulationTelemetryData)
		telemetrydataapi.GET("simulation/init", api.Controllers.TelemetryDataApi.GetSimulationInit)
		telemetrydataapi.POST("simulation/send", api.Controllers.TelemetryDataApi.SimulationSend)
		telemetrydataapi.GET("msg/count", api.Controllers.TelemetryDataApi.ServeMsgCountByTenant)
	}
}
