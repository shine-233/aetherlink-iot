// device_config.go 负责设备配置领域的 HTTP 入口。
// 核心链路：
// 1. 绑定并校验请求参数或路径参数。
// 2. 从 Gin 上下文读取 claims、语言头等请求级信息。
// 3. 调用 DeviceConfig service 完成设备配置 CRUD、连接信息与自动化菜单装配。
// 4. 把结果写入统一响应上下文，错误统一交给中间件处理。
// 静态审查建议：
// 1. 当前多个 handler 都重复读取 claims 和语言头，后续可继续抽出更薄的辅助函数。
// 2. 该层已经保持了较薄控制器职责，后续不要把物模型解绑、协议推导等业务逻辑回灌到 API 层。
// 3. `/menu`、`/connect`、`/voucher_type`、自动化下拉菜单这些接口都依赖细粒度契约，改动时要同步前端设备配置编辑页。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceConfigApi struct{}

// CreateDeviceConfig 创建设备配置。
// 请求体承载基础配置、物模型绑定、协议类型和凭证类型等字段；claims 决定租户归属与权限边界。
// @Router   /api/v1/device_config [post]
func (*DeviceConfigApi) CreateDeviceConfig(c *gin.Context) {
	var req model.CreateDeviceConfigReq
	if !BindAndValidate(c, &req) {
		return
	}
	var userClaims = c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceConfig.CreateDeviceConfig(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// UpdateDeviceConfig 更新设备配置。
// 这里仍保持“参数绑定 + claims 注入 + service 调用”的薄入口模式，复杂的物模型解绑和协议副作用都留给 service 处理。
// @Router   /api/v1/device_config [put]
func (*DeviceConfigApi) UpdateDeviceConfig(c *gin.Context) {
	var req model.UpdateDeviceConfigReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceConfig.UpdateDeviceConfig(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// DeleteDeviceConfig 删除设备配置。
// 删除动作会波及物模型绑定与缓存一致性，因此这里只负责把 ID 与 claims 交给 service，不在 API 层做额外业务分支。
// @Router   /api/v1/device_config/{id} [delete]
func (*DeviceConfigApi) DeleteDeviceConfig(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceConfig.DeleteDeviceConfig(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleDeviceConfigById 根据 ID 获取设备配置详情。
// 前端设备配置编辑页会依赖这个接口回填协议配置、物模型选择和其他半结构化字段。
// @Router   /api/v1/device_config/{id} [get]
func (*DeviceConfigApi) HandleDeviceConfigById(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	info, err := service.GroupApp.DeviceConfig.GetDeviceConfigByID(c, id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", info)
}

// HandleDeviceConfigListByPage 设备配置分页查询。
// 该接口既服务设备配置列表，也间接影响市场发布和设备接入入口的可选配置范围。
// @Router   /api/v1/device_config [get]
func (*DeviceConfigApi) HandleDeviceConfigListByPage(c *gin.Context) {
	var req model.GetDeviceConfigListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	deviceconfigList, err := service.GroupApp.DeviceConfig.GetDeviceConfigListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", deviceconfigList)
}

// HandleDeviceConfigListMenu 返回设备配置菜单化列表。
// 主要用于下拉选择或轻量菜单场景，契约通常比完整分页列表更轻。
// @Router   /api/v1/device_config/menu [get]
func (*DeviceConfigApi) HandleDeviceConfigListMenu(c *gin.Context) {
	var req model.GetDeviceConfigListMenuReq
	if !BindAndValidate(c, &req) {
		return
	}

	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	deviceconfigList, err := service.GroupApp.DeviceConfig.GetDeviceConfigListMenu(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", deviceconfigList)
}

// BatchUpdateDeviceConfig 批量绑定设备到网关。
// 该入口属于高影响批处理动作，只做参数绑定与权限透传，具体多级网关关系更新交给 service。
// @Router   /api/v1/device_config/batch [put]
func (*DeviceConfigApi) BatchUpdateDeviceConfig(c *gin.Context) {
	var req model.BatchUpdateDeviceConfigReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceConfig.BatchUpdateDeviceConfig(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// HandleDeviceConfigConnect 返回设备配置对应的接入说明或连接信息。
// 这里额外读取 Accept-Language，是因为连接说明文案会受当前语言影响。
// /api/v1/device_config/connect
func (*DeviceConfigApi) HandleDeviceConfigConnect(c *gin.Context) {
	var param model.DeviceIDReq
	if !BindAndValidate(c, &param) {
		return
	}
	lang := c.GetHeader("Accept-Language")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceConfig.GetDeviceConfigConnect(c, param.DeviceID, lang, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleVoucherType 返回当前设备类型 + 协议类型下的凭证表单定义。
// 前端设备配置编辑页会在切换协议时调用此接口，动态刷新凭证配置方式。
// /api/v1/device_config/voucher_type
func (*DeviceConfigApi) HandleVoucherType(c *gin.Context) {
	var param model.GetVoucherTypeReq
	if !BindAndValidate(c, &param) {
		return
	}
	lang := c.GetHeader("Accept-Language")
	data, err := service.GroupApp.DeviceConfig.GetVoucherTypeForm(param.DeviceType, param.ProtocolType, lang)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleActionByDeviceConfigID 根据设备配置 ID 返回自动化动作下拉项。
// 该接口把设备配置与自动化能力桥接起来，供前端规则编辑器装配动作菜单。
// /api/v1/device_config/metrics/menu [get]
func (*DeviceConfigApi) HandleActionByDeviceConfigID(c *gin.Context) {
	var param model.GetActionByDeviceConfigIDReq
	if !BindAndValidate(c, &param) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	list, err := service.GroupApp.DeviceConfig.GetActionByDeviceConfigID(param.DeviceConfigID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)
}

// HandleConditionByDeviceConfigID 根据设备配置 ID 返回自动化条件下拉项。
// 与动作菜单类似，这里为规则条件编辑器提供配置相关的可选指标；当前继续复用 action 请求结构体作为入参契约。
// /api/v1/device_config/metrics/condition/menu
func (*DeviceConfigApi) HandleConditionByDeviceConfigID(c *gin.Context) {
	var param model.GetActionByDeviceConfigIDReq
	if !BindAndValidate(c, &param) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	list, err := service.GroupApp.DeviceConfig.GetConditionByDeviceConfigID(param.DeviceConfigID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)
}
