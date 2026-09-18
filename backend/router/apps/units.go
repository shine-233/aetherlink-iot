package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type UnitsRouter struct{}

func (s *UnitsRouter) InitUnitsRouter(Router *gin.RouterGroup) {
	unitsRouter := Router.Group("units")
	unitsApi := api.Controllers.UnitsApi
	{
		unitsRouter.GET("registry", unitsApi.GetRegistry)
		unitsRouter.POST("convert", unitsApi.Convert)
	}
}
