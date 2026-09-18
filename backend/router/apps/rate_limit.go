package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type RateLimitRouter struct{}

func (s *RateLimitRouter) InitRateLimitRouter(Router *gin.RouterGroup) {
	ratelimitRouter := Router.Group("ratelimit")
	rateLimitApi := api.Controllers.RateLimitApi
	{
		ratelimitRouter.GET("config", rateLimitApi.GetConfig)
		ratelimitRouter.GET("metrics", rateLimitApi.GetMetrics)
		ratelimitRouter.GET("overrides", rateLimitApi.ListOverrides)
		ratelimitRouter.POST("override", rateLimitApi.SetOverride)
		ratelimitRouter.DELETE("override/:target_type/:target_id", rateLimitApi.DeleteOverride)
	}
}
