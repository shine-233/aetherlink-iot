// 文件用途：提供设备诊断与连接诊断相关的 HTTP 接口处理器。
// 核心逻辑：统一解析设备路径 ID、可选日志条数和用户 claims，再调用设备诊断 service。
// 关键注意事项：本层只做参数与响应编排，不读取调试存储或推断设备在线状态。
package api

import (
	"strconv"

	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// GetDeviceDiagnostics 获取设备诊断数据。
// @Router   /api/v1/devices/{device_id}/diagnostics [get]
func (*DeviceApi) GetDeviceDiagnostics(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "device_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GetDiagnostics(deviceID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// GetDeviceConnectionDiagnostics returns read-only connection evidence for the
// device onboarding guide without requiring the client to call debug and
// collector endpoints separately.
// @Summary Get device connection diagnostics
// @Description Returns read-only connection evidence (debug logs, connection metadata) for the device onboarding guide without requiring the client to call debug and collector endpoints separately.
// @Tags DeviceOnboarding
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_id path string true "Device ID"
// @Param debug_log_limit query int false "Optional limit on the number of debug log entries included"
// @Success 200 {object} object "Connection diagnostics payload"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/device/{device_id}/connection/diagnostics [get]
func (*DeviceApi) GetDeviceConnectionDiagnostics(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "device_id is required"))
		return
	}

	var debugLogLimit int64
	if rawLimit := c.Query("debug_log_limit"); rawLimit != "" {
		parsed, err := strconv.ParseInt(rawLimit, 10, 64)
		if err != nil {
			c.Error(errcode.WithData(errcode.CodeParamError, "debug_log_limit must be an integer"))
			return
		}
		debugLogLimit = parsed
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GetConnectionDiagnostics(c.Request.Context(), service.DeviceConnectionDiagnosticsReq{
		DeviceID:      deviceID,
		DebugLogLimit: debugLogLimit,
	}, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}
