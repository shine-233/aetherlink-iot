// 文件用途：设备预注册 API 层——批次建档（自动/CSV）、分页查询与导出的 HTTP 入口。
// 核心逻辑：绑定校验请求 → 提取 claims → 调用 service → 交由统一响应中间件封装。
// 关键注意事项：创建响应包含一次性明文 voucher，禁止在任何列表/查询面复现该字段。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// HandleDevicePreRegisterListByPage 分页查询当前租户的预注册设备
// @Router   /api/v1/device/preRegister [get]
func (*DeviceApi) HandleDevicePreRegisterListByPage(c *gin.Context) {
	var req model.GetDevicePreRegisterListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GetDevicePreRegisterListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateDevicePreRegister 按产品+批次批量建档：create_type=1 自动生成，2=CSV 批次文件
// @Router   /api/v1/device/preRegister [post]
func (*DeviceApi) CreateDevicePreRegister(c *gin.Context) {
	var req model.CreateDevicePreRegisterReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.CreateDevicePreRegister(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ExportDevicePreRegister 按产品/批次导出预注册清单（voucher 已脱敏）
// @Router   /api/v1/device/preRegister/export [get]
func (*DeviceApi) ExportDevicePreRegister(c *gin.Context) {
	var req model.ExportPreRegisterReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.ExportDevicePreRegister(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
