// 文件用途：边缘计算 2.0（ROADMAP D6）HTTP 入口。
// 边界说明：租户边界在 service 层处理；本层只做绑定、claims 提取与错误出口。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

type EdgeSyncApi struct{}

// Create 创建看板/规则链边缘同步任务并立即投递。
// POST /api/v1/edge/sync
func (*EdgeSyncApi) Create(c *gin.Context) {
	var req model.CreateEdgeSyncReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeSync.CreateEdgeSync(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// List 列边缘同步任务（过滤 resource_type/gateway_device_id/status）。
// GET /api/v1/edge/sync
func (*EdgeSyncApi) List(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeSync.ListEdgeSyncTasks(
		c.Query("resource_type"), c.Query("gateway_device_id"), c.Query("status"), 0, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Get 单条任务详情。
// GET /api/v1/edge/sync/:id
func (*EdgeSyncApi) Get(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeSync.GetEdgeSyncTask(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Retry 重试投递（重放任务行内快照）。
// POST /api/v1/edge/sync/:id/retry
func (*EdgeSyncApi) Retry(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeSync.RetryEdgeSync(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// DistributeOta OTA 经边分发（网关 + 升级包 + 目标设备集合）。
// POST /api/v1/edge/ota/distribute
func (*EdgeSyncApi) DistributeOta(c *gin.Context) {
	var req model.EdgeOtaDistributeReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.EdgeSync.DistributeEdgeOTA(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
