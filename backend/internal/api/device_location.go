// 文件用途：提供设备地理空间定位（TB-13 Geospatial Map Tracking）HTTP 处理器。
// 核心逻辑：负责入参绑定、Claims 提取、Service 层分发与统一 API 响应交付。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// HandleGetLatestDeviceLocations 查询全部设备的最新 GPS 位置与在线状态
// @Router   /api/v1/devices/locations/latest [get]
// @Router   /api/v1/device/locations/latest [get]
func (*DeviceApi) HandleGetLatestDeviceLocations(c *gin.Context) {
	var req model.DeviceLocationLatestReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GetLatestDeviceLocations(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleGetDeviceLocationHistory 查询单设备历史地理运动轨迹
// @Router   /api/v1/device/:id/location/history [get]
// @Router   /api/v1/devices/:device_id/location/history [get]
func (*DeviceApi) HandleGetDeviceLocationHistory(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		deviceID = c.Param("id")
	}
	var req model.DeviceLocationHistoryReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GetDeviceLocationHistory(c.Request.Context(), deviceID, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
