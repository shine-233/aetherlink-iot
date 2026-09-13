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

		// P1.5 边缘节点注册表：注册/心跳/列表/Reconcile。
		// 此前注册、健康判定与版本兼容只有纯决策函数，无数据来源也无入口；
		// edge_nodes 表（97.sql）落地后这里是最小可用闭环。
		nodes := g.Group("nodes")
		{
			nodes.POST("", api.Controllers.EdgeNodeApi.Register)
			nodes.GET("", api.Controllers.EdgeNodeApi.List)
			nodes.POST(":node_id/heartbeat", api.Controllers.EdgeNodeApi.Heartbeat)
			nodes.POST(":node_id/reconcile", api.Controllers.EdgeNodeApi.Reconcile)
		}
	}
}
