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
	Handle(c, func(req *model.RegisterEdgeNodeReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.EdgeNode.RegisterEdgeNode(*req, claims)
	})
}

// Heartbeat 心跳触碰并回报健康分类。
// @Summary  边缘节点心跳
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/heartbeat [post]
func (*EdgeNodeApi) Heartbeat(c *gin.Context) {
	// 心跳体可选：绑定失败（非 JSON）不拒绝——心跳的价值在到达本身。
	// 因此走 HandlePathBodyOptional（绑定失败放行），而非 HandlePathBody（其绑定失败即拒绝）。
	HandlePathBodyOptional(c, "node_id", func(nodeID string, req *model.EdgeNodeHeartbeatReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.EdgeNode.HeartbeatEdgeNode(nodeID, *req, claims)
	})
}

// List 列出租户内节点及健康分类。
// @Summary  边缘节点列表
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes [get]
func (*EdgeNodeApi) List(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		// limit 沿用旧解析：缺省/非法/非正数一律按 0（全量）处理，不作为参数错误。
		limit := 0
		if v := c.Query("limit"); v != "" {
			if parsed, perr := strconv.Atoi(v); perr == nil && parsed > 0 {
				limit = parsed
			}
		}
		return service.GroupApp.EdgeNode.ListEdgeNodes(limit, claims)
	})
}

// Reconcile 重连后按版本同步的编排入口。
// @Summary  边缘节点 Reconcile
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/reconcile [post]
func (*EdgeNodeApi) Reconcile(c *gin.Context) {
	HandlePathBody(c, "node_id", func(nodeID string, req *model.EdgeNodeReconcileReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.EdgeNode.ReconcileEdgeNode(nodeID, *req, claims)
	})
}

// IssueCertificate 签发边缘节点客户端证书。
// @Summary  签发边缘节点证书
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/certificate [post]
func (*EdgeNodeApi) IssueCertificate(c *gin.Context) {
	// 请求体可选（validity_days 有缺省值）：绑定失败不拒绝，故走 HandlePathBodyOptional 而非 HandlePathBody。
	HandlePathBodyOptional(c, "node_id", func(nodeID string, req *model.IssueEdgeNodeCertificateReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.EdgeNode.IssueNodeCertificate(nodeID, *req, claims)
	})
}

// GetCertificate 查看边缘节点当前有效证书。
// @Summary  查看边缘节点证书
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/certificate [get]
func (*EdgeNodeApi) GetCertificate(c *gin.Context) {
	HandlePath(c, "node_id", func(nodeID string, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.EdgeNode.GetNodeCertificate(nodeID, claims)
	})
}

// RevokeCertificate 吊销边缘节点证书。
// @Summary  吊销边缘节点证书
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/certificate [delete]
func (*EdgeNodeApi) RevokeCertificate(c *gin.Context) {
	// 成功包络保留旧的 data.message 对象，故用 HandlePath 而非 HandlePathAction（后者成功时 data 为 nil）。
	HandlePath(c, "node_id", func(nodeID string, claims *utils.UserClaims) (interface{}, error) {
		if err := service.GroupApp.EdgeNode.RevokeNodeCertificate(nodeID, claims); err != nil {
			return nil, err
		}
		return gin.H{"message": "edge node certificate revoked"}, nil
	})
}

// Upgrade 远程升级边缘节点。
// @Summary  升级边缘节点
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/upgrade [post]
func (*EdgeNodeApi) Upgrade(c *gin.Context) {
	HandlePathBody(c, "node_id", func(nodeID string, req *model.UpgradeEdgeNodeReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.EdgeNode.UpgradeNode(nodeID, *req, claims)
	})
}

// Rollback 远程回滚边缘节点。
// @Summary  回滚边缘节点
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/rollback [post]
func (*EdgeNodeApi) Rollback(c *gin.Context) {
	HandlePathBody(c, "node_id", func(nodeID string, req *model.RollbackEdgeNodeReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.EdgeNode.RollbackNode(nodeID, *req, claims)
	})
}

// GetUpgradeHistory 查询边缘节点升级历史。
// @Summary  边缘节点升级历史
// @Tags     EdgeNodes
// @Router   /api/v1/edge/nodes/{node_id}/upgrade/history [get]
func (*EdgeNodeApi) GetUpgradeHistory(c *gin.Context) {
	HandlePath(c, "node_id", func(nodeID string, claims *utils.UserClaims) (interface{}, error) {
		// limit 沿用旧解析：缺省/非法/非正数一律按 0 处理，不作为参数错误。
		limit := 0
		if v := c.Query("limit"); v != "" {
			if parsed, perr := strconv.Atoi(v); perr == nil && parsed > 0 {
				limit = parsed
			}
		}
		return service.GroupApp.EdgeNode.ListNodeUpgradeHistory(nodeID, limit, claims)
	})
}
