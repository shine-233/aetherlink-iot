package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// EdgeSync 边缘计算 2.0 路由组（ROADMAP D6）。
type EdgeSync struct{}

func (*EdgeSync) InitEdgeSync(Router *gin.RouterGroup) {
	g := Router.Group("edge")
	{
		sync := g.Group("sync")
		{
			sync.POST("", api.Controllers.EdgeSyncApi.Create)
			sync.GET("", api.Controllers.EdgeSyncApi.List)
			sync.GET(":id", api.Controllers.EdgeSyncApi.Get)
			sync.POST(":id/retry", api.Controllers.EdgeSyncApi.Retry)
		}
		g.POST("ota/distribute", api.Controllers.EdgeSyncApi.DistributeOta)
	}
}
