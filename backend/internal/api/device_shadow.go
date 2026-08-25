// 文件用途：设备影子消息域的 HTTP 入口编排（ROADMAP A3 离线命令缓存）。
// 核心职责：提供影子消息列表查询、设置（在线直发/离线缓存）和取消接口。
// 边界说明：设备归属与租户边界由 service 层的设备访问守卫校验，本层只做绑定与错误出口。
package api

import (
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceShadowApi struct{}

// HandleShadowMessageList 查询设备影子消息列表。
// GET /api/v1/device/shadow/:deviceId?status=pending
func (*DeviceShadowApi) HandleShadowMessageList(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}
	status := c.Query("status")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceShadow.GetShadowMessages(deviceId, status, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// SetShadowMessage 设置影子消息：设备在线直接下发，离线写入缓存队列。
// POST /api/v1/device/shadow/:deviceId
func (*DeviceShadowApi) SetShadowMessage(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}
	var req service.SetDeviceShadowMessageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceShadow.SetShadowMessage(deviceId, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// CancelShadowMessage 取消指定的待投递影子消息。
// DELETE /api/v1/device/shadow/:deviceId/:msgId
func (*DeviceShadowApi) CancelShadowMessage(c *gin.Context) {
	deviceId := c.Param("deviceId")
	msgId := c.Param("msgId")
	if deviceId == "" || msgId == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id and msg_id are required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.DeviceShadow.CancelShadowMessage(deviceId, msgId, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}
