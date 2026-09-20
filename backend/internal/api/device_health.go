// 文件用途：设备综合健康度评估（Device Health Score）API 处理器。
// 核心逻辑：HTTP 参数提取、租户 Claims 注入、服务分发与响应包装。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

type DeviceHealthApi struct{}

// GetHealthSummary 获取租户设备健康汇总大盘
// @Router   /api/v1/devices/health/summary [get]
func (*DeviceHealthApi) GetHealthSummary(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceHealth.GetTenantHealthSummary(c.Request.Context(), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetDeviceHealth 查询单设备多维健康评分诊断详情
// @Router   /api/v1/devices/:device_id/health [get]
// @Router   /api/v1/device/:id/health [get]
func (*DeviceHealthApi) GetDeviceHealth(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		deviceID = c.Param("id")
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceHealth.GetDeviceHealthDetail(c.Request.Context(), deviceID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// EvaluateDeviceHealth 触发单设备或租户全量健康度诊断评估
// @Router   /api/v1/devices/health/evaluate [post]
// @Router   /api/v1/devices/:device_id/health/evaluate [post]
// @Router   /api/v1/device/:id/health/evaluate [post]
func (*DeviceHealthApi) EvaluateDeviceHealth(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		deviceID = c.Param("id")
	}

	var req model.EvaluateDeviceHealthReq
	_ = c.ShouldBindJSON(&req)
	if deviceID == "" && req.DeviceID != nil && strings.TrimSpace(*req.DeviceID) != "" {
		deviceID = strings.TrimSpace(*req.DeviceID)
	}

	claims := c.MustGet("claims").(*utils.UserClaims)

	if deviceID != "" {
		data, err := service.GroupApp.DeviceHealth.EvaluateDeviceHealth(c.Request.Context(), deviceID, claims)
		if err != nil {
			c.Error(err)
			return
		}
		c.Set("data", data)
		return
	}

	// 全量租户设备评估
	data, err := service.GroupApp.DeviceHealth.EvaluateTenantDeviceHealth(c.Request.Context(), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
