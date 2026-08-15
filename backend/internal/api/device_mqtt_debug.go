// 文件用途：提供设备级 MQTT 调试会话的连接、订阅、发布、日志和关闭入口。
// 核心逻辑：handler 只绑定路径/请求，权限和 broker 会话隔离由 service/mqttdebug 模块负责。
// 关键注意事项：所有路由都同时携带 device_id 与 session_id，不能把 session 当作跨设备能力。
package api

import (
	"strings"

	"aetherlink-iot/backend/internal/middleware"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func (*DeviceDebugApi) OpenMQTTDebugSession(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}
	middleware.SetOperationLogSafeMetadata(c, map[string]interface{}{
		"operation": "mqtt_debug_open",
		"device_id": deviceID,
	})
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceDebug.OpenMQTTDebugSession(c.Request.Context(), deviceID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*DeviceDebugApi) GetMQTTDebugSession(c *gin.Context) {
	deviceID, sessionID, ok := bindMQTTDebugPath(c)
	if !ok {
		return
	}
	var req model.DeviceMQTTDebugSnapshotReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceDebug.GetMQTTDebugSession(c.Request.Context(), deviceID, sessionID, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*DeviceDebugApi) ApplyMQTTDebugCommand(c *gin.Context) {
	deviceID, sessionID, ok := bindMQTTDebugPath(c)
	if !ok {
		return
	}
	var req model.DeviceMQTTDebugCommandReq
	if !BindAndValidate(c, &req) {
		return
	}
	middleware.SetOperationLogSafeMetadata(c, map[string]interface{}{
		"operation":     "mqtt_debug_command",
		"device_id":     deviceID,
		"session_id":    sessionID,
		"action":        req.Action,
		"topic":         req.Topic,
		"qos":           req.QoS,
		"payload_bytes": len(req.Payload),
	})
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceDebug.ApplyMQTTDebugCommand(c.Request.Context(), deviceID, sessionID, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*DeviceDebugApi) CloseMQTTDebugSession(c *gin.Context) {
	deviceID, sessionID, ok := bindMQTTDebugPath(c)
	if !ok {
		return
	}
	middleware.SetOperationLogSafeMetadata(c, map[string]interface{}{
		"operation":  "mqtt_debug_close",
		"device_id":  deviceID,
		"session_id": sessionID,
	})
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.DeviceDebug.CloseMQTTDebugSession(c.Request.Context(), deviceID, sessionID, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

func bindMQTTDebugPath(c *gin.Context) (string, string, bool) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return "", "", false
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "session_id is required"))
		return "", "", false
	}
	return deviceID, sessionID, true
}
