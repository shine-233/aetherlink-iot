// protocol_plugin.go 提供协议插件相关的 HTTP 入口。
// 核心链路：
// 1. 对后台页面提供协议插件动态配置表单查询。
// 2. 对插件接入侧提供按凭证、设备号或协议标识符查询设备配置与设备列表的能力。
// 3. 对插件访问入口额外叠加 OpenAPIKey 鉴权和限流保护。
// 静态审查建议：
// 1. 该文件同时服务后台配置页和插件接入面，后续若接口继续增加，建议按“后台管理 / 插件接入”拆分。
// 2. 设备配置插件入口里的限流键构造直接绑定哈希后的 voucher/明文 device_number，调整鉴权策略时要同步检查这里。
// 3. 插件侧接口都依赖 claims 注入与 OpenAPIKeyAuth，中间件约定变化会直接影响接入侧可用性。
package api

import (
	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/middleware"
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ProtocolPluginApi struct{}

// HandleProtocolPluginFormByProtocolType 根据协议类型和设备类型返回动态配置表单。
// 前端设备配置编辑页会依赖这个接口渲染协议插件的动态字段。
// @Router   /api/v1/protocol_plugin/config_form [get]
func (*ProtocolPluginApi) HandleProtocolPluginFormByProtocolType(c *gin.Context) {
	var req model.GetProtocolPluginFormByProtocolType
	if !BindAndValidate(c, &req) {
		return
	}
	data, err := service.GroupApp.ServicePlugin.GetProtocolPluginFormByProtocolType(req.ProtocolType, req.DeviceType)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleDeviceConfigForProtocolPlugin 为协议插件返回设备配置。
// 该接口会先经过 OpenAPIKey 鉴权，再按 voucher 或 device_number 做接入侧限流。
// /api/v1/plugin/device/config
func (*ProtocolPluginApi) HandleDeviceConfigForProtocolPlugin(c *gin.Context) {
	var req model.GetDeviceConfigReq
	if !BindAndValidate(c, &req) {
		return
	}

	// 限流检查：只对 voucher 和 device_number 两类高频接入标识做限流。
	if !middleware.OpenAPIKeyAuth(c) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	var limitKey string
	if req.Voucher != "" {
		// 限流键使用 sha256 摘要，避免明文 voucher 落入 Redis keyspace；空值分支保持原判断顺序。
		limitKey = "device_auth_voucher:" + utils.VoucherCacheKey(req.Voucher)
	} else if req.DeviceNumber != "" {
		limitKey = "device_auth_device_number:" + req.DeviceNumber
	}

	if limitKey != "" {
		limiter := initialize.NewDeviceAuthLimiter()
		if !limiter.Allow(limitKey) {
			c.Error(errcode.WithData(errcode.CodeRateLimit, map[string]interface{}{
				"error": "Request rate limit exceeded, please try again later",
			}))
			return
		}
	}

	data, err := service.GroupApp.ProtocolPlugin.GetDeviceConfig(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleDeviceConfigForProtocolPluginByProtocolType 按协议标识符返回设备配置与设备列表。
// 主要服务协议插件批量感知设备侧配置或设备清单的场景。
// /api/v1/plugin/devices
func (*ProtocolPluginApi) HandleDeviceConfigForProtocolPluginByProtocolType(c *gin.Context) {
	var req model.GetDevicesByProtocolPluginReq
	if !BindAndValidate(c, &req) {
		return
	}

	if !middleware.OpenAPIKeyAuth(c) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.ProtocolPlugin.GetDevicesByProtocolPlugin(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
