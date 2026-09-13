// 文件用途：P1.5 边缘节点注册/心跳/列表/Reconcile 的 HTTP 入口。
// 边界说明：租户边界与判定都在 service 层；本层只做绑定、claims 提取与错误出口。
package api

import (
	"strconv"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type EdgeNodeApi struct{}

// Register 注册/重注册边缘节点（幂等；跨租户抢注会被拒绝）。
// @Summary  注册边缘节点
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes [post]
func (*EdgeNodeApi) Register(c *gin.Context) {
	var req model.RegisterEdgeNodeReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeNode.RegisterEdgeNode(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Heartbeat 心跳触碰并回报健康分类。
// @Summary  边缘节点心跳
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/heartbeat [post]
func (*EdgeNodeApi) Heartbeat(c *gin.Context) {
	nodeID := c.Param("node_id")
	var req model.EdgeNodeHeartbeatReq
	// 心跳体可选：绑定失败（非 JSON）不拒绝——心跳的价值在到达本身。
	_ = c.ShouldBindJSON(&req)
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeNode.HeartbeatEdgeNode(nodeID, req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// List 列出租户内节点及健康分类。
// @Summary  边缘节点列表
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes [get]
func (*EdgeNodeApi) List(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	limit := 0
	if v := c.Query("limit"); v != "" {
		if parsed, perr := strconv.Atoi(v); perr == nil && parsed > 0 {
			limit = parsed
		}
	}
	resp, err := service.GroupApp.EdgeNode.ListEdgeNodes(limit, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Reconcile 重连后按版本同步的编排入口。
// @Summary  边缘节点 Reconcile
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/reconcile [post]
func (*EdgeNodeApi) Reconcile(c *gin.Context) {
	nodeID := c.Param("node_id")
	var req model.EdgeNodeReconcileReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeNode.ReconcileEdgeNode(nodeID, req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
