// device_debug.go 提供设备调试开关、状态和日志查询入口。
// 核心链路：
// 1. 从路径中读取 device_id，并校验不能为空。
// 2. 绑定调试请求或日志查询参数。
// 3. 注入 claims 后调用 DeviceDebug service 完成调试开关控制、状态查询或日志回读。
// 静态审查建议：
// 1. 设备调试属于高影响运维能力，后续若要扩展，应优先统一审计日志和权限边界，而不是先加更多页面按钮。
// 2. 三个 handler 都重复了 device_id 判空和 claims 读取，后续可抽取小 helper 进一步保持薄控制器。
// 3. 调试日志查询会直接影响问题排查效率，修改日志分页或过滤契约时要同步前端调试页。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceDebugApi struct{}

// SetDeviceDebug 开启或关闭设备调试日志。
// 该入口只负责参数和上下文透传，具体开关语义、缓存与日志后端联动由 service 处理。
// @Summary Enable or disable device debug logs
// @Description Enables or disables debug capture for a device and updates the debug config (duration, item limits, payload bytes). Setting enabled=false removes the debug config.
// @Tags DeviceDebug
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_id path string true "Device ID"
// @Param request body model.SetDeviceDebugReq true "Debug config payload"
// @Success 200 {object} service.DeviceDebugStatus "Debug status after apply"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/device/{device_id}/debug [post]
func (*DeviceDebugApi) SetDeviceDebug(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}

	var req model.SetDeviceDebugReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceDebug.SetDeviceDebug(c.Request.Context(), deviceID, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetDeviceDebugStatus 查询设备调试状态。
// 前端通常用它决定调试页当前是“开启”还是“关闭”态。
// @Summary Get device debug status
// @Description Returns the current debug config for a device so the client can render enabled/disabled state and remaining TTL.
// @Tags DeviceDebug
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_id path string true "Device ID"
// @Success 200 {object} service.DeviceDebugStatus "Current debug status"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/device/{device_id}/debug/status [get]
func (*DeviceDebugApi) GetDeviceDebugStatus(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceDebug.GetDeviceDebugStatus(c.Request.Context(), deviceID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetDeviceDebugLogs 查询设备调试日志。
// 该接口会携带分页或筛选参数进入 service，是调试问题回放的重要入口。
// @Summary List device debug logs
// @Description Returns a paged slice of the device debug log ring buffer collected by gmqtt for the specified device.
// @Tags DeviceDebug
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_id path string true "Device ID"
// @Param offset query int false "Offset into the debug log buffer (>=0)"
// @Param limit query int false "Number of entries to return (1-500)"
// @Success 200 {object} service.DeviceDebugLogsResp "Paged debug log entries"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/device/{device_id}/debug/logs [get]
func (*DeviceDebugApi) GetDeviceDebugLogs(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}

	var req model.GetDeviceDebugLogsReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceDebug.GetDeviceDebugLogs(c.Request.Context(), deviceID, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
