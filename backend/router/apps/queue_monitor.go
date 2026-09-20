package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type QueueMonitorRouter struct{}

func (s *QueueMonitorRouter) InitQueueMonitorRouter(Router *gin.RouterGroup) {
	queueRouter := Router.Group("queue")
	queueMonitorApi := api.Controllers.QueueMonitorApi
	{
		queueRouter.GET("stats", queueMonitorApi.GetStats)
		queueRouter.GET("config", queueMonitorApi.GetConfig)
	}
}
