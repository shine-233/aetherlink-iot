// 文件用途：注册设备 Modbus 点表相关路由（ROADMAP B1 前端配置界面配套）。
// 核心逻辑：UI 按 deviceId 读写；插件按 deviceNumber 拉取（x-api-key 鉴权由 v1 组中间件处理）。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type DeviceModbusProfile struct{}

func (*DeviceModbusProfile) InitDeviceModbusProfile(Router *gin.RouterGroup) {
	modbusProfile := Router.Group("device/modbus/profile")
	{
		profileApi := api.Controllers.DeviceModbusProfileApi
		modbusProfile.GET(":deviceId", profileApi.HandleGetModbusProfile)
		modbusProfile.PUT(":deviceId", profileApi.HandleSaveModbusProfile)
		modbusProfile.GET("number/:deviceNumber", profileApi.HandleGetModbusProfileByNumber)
	}
}
