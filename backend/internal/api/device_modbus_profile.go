// 文件用途：设备 Modbus 点表域的 HTTP 入口（ROADMAP B1 前端配置界面配套）。
// 边界说明：租户与设备访问守卫在 service 层；本层只做绑定、claims 提取和错误出口。
package api

import (
	"encoding/json"
	"io"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceModbusProfileApi struct{}

type saveModbusProfileBody struct {
	Profile json.RawMessage `json:"profile" validate:"required"`
}

// HandleGetModbusProfile 前端读取点表。
// GET /api/v1/device/modbus/profile/:deviceId
func (*DeviceModbusProfileApi) HandleGetModbusProfile(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceModbusProfile.GetProfileForUser(deviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleSaveModbusProfile 前端保存点表。body: {"profile": {...}}
// PUT /api/v1/device/modbus/profile/:deviceId
func (*DeviceModbusProfileApi) HandleSaveModbusProfile(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_id is required"))
		return
	}
	var body saveModbusProfileBody
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 128*1024))
	if err != nil || len(raw) == 0 {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "request body is required"))
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil || len(body.Profile) == 0 {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "body must be {\"profile\": {...}}"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceModbusProfile.SaveProfile(deviceId, body.Profile, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleGetModbusProfileByNumber 插件按设备编号拉取点表（x-api-key 鉴权）。
// GET /api/v1/device/modbus/profile/number/:deviceNumber
func (*DeviceModbusProfileApi) HandleGetModbusProfileByNumber(c *gin.Context) {
	deviceNumber := c.Param("deviceNumber")
	if deviceNumber == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "device_number is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceModbusProfile.GetProfileByDeviceNumber(deviceNumber, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
