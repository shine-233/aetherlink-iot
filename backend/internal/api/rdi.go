// 文件用途：
//
//	提供 RDI 设备相关 HTTP 接口，覆盖设备激活、配置读取与更新、历史查询、指令下发、分享与共享接入等入口。
//
// 核心链路：
//
//	handler 负责请求参数绑定、读取 claims 和路径参数，再委托 service.GroupApp.RDI 执行设备侧业务，最后通过 c.Set("data", ...) 交给统一响应中间件输出。
//
// 使用注意：
//  1. 本文件大量依赖 c.MustGet("claims") 和 c.Param("device_id"/"token")，路由与鉴权中间件必须保证上下文键存在。
//  2. 设备写操作统一透传 request context，后续扩展时不要绕过上下文取消、超时和审计链路。
//
// 静态审查建议：
//  1. 重点核对每个 device_id、token 接口是否在 service 层完成租户隔离、设备归属和分享权限校验。
//  2. 审查写接口的幂等性、错误码一致性，以及共享链路是否暴露超范围设备信息。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type RDIApi struct{}

// ThingModel 返回 RDI 物模型定义。
// 核心链路：直接从 service 层读取静态或聚合后的物模型数据，并交由统一响应层输出。
// 审查重点：关注物模型是否包含仅限内部使用的字段，以及多版本模型切换时的兼容性。
func (*RDIApi) ThingModel(c *gin.Context) {
	c.Set("data", service.GroupApp.RDI.ThingModel())
}

// ActivateDevice 激活指定产品下的 RDI 设备。
// 核心链路：绑定激活参数后读取当前用户 claims，再由 service 完成产品绑定、设备归属和激活态切换。
// 使用注意：API 层不做兜底授权判断，静态审查时要确认 service 端校验了产品、租户和用户关系。
func (*RDIApi) ActivateDevice(c *gin.Context) {
	var req model.ActivateRDIDeviceReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.RDI.ActivateDeviceByPID(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeviceConfig 获取设备配置。
// 核心链路：从路径读取 device_id，结合 claims 调用 service 返回当前用户可见的设备配置。
// 审查重点：确认敏感配置字段是否需要脱敏，以及非法 device_id 是否统一返回预期错误码。
func (*RDIApi) DeviceConfig(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	data, err := service.GroupApp.RDI.DeviceConfig(deviceID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeviceHistory 查询设备历史数据。
// 核心链路：绑定分页/筛选参数，结合 device_id 与 claims 调用 service 执行历史查询。
// 审查重点：关注时间范围、分页上限、跨租户历史数据泄露和空结果响应一致性。
func (*RDIApi) DeviceHistory(c *gin.Context) {
	var req model.RDIHistoryReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	data, err := service.GroupApp.RDI.DeviceHistory(deviceID, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// LatestFirmware 返回设备可升级的最新固件包信息。
// 审查重点：确认 service 是否按设备型号、批次和权限过滤可见固件，避免错误升级指引。
func (*RDIApi) LatestFirmware(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	data, err := service.GroupApp.RDI.LatestFirmwarePackage(deviceID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateDeviceConfig 更新设备配置。
// 核心链路：绑定更新参数后透传 request context，交由 service 执行配置写入与可能的下行同步。
// 使用注意：这类写接口对副作用敏感，静态审查时要确认 service 侧有权限校验、字段白名单和失败回滚策略。
func (*RDIApi) UpdateDeviceConfig(c *gin.Context) {
	var req model.UpdateRDIConfigReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	data, err := service.GroupApp.RDI.UpdateDeviceConfig(c.Request.Context(), deviceID, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// SendCommand 下发设备指令。
// 核心链路：完成参数绑定后，由 service 根据 device_id、claims 和请求上下文执行命令投递。
// 审查重点：确认高风险指令是否有限流、审计和幂等控制，并检查超时/离线设备错误是否可区分。
func (*RDIApi) SendCommand(c *gin.Context) {
	var req model.RDICommandReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	data, err := service.GroupApp.RDI.SendCommand(c.Request.Context(), deviceID, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateShareToken 创建设备分享令牌。
// 核心链路：基于当前用户、设备和分享请求参数，由 service 生成受控分享凭证。
// 审查重点：确认 token 过期时间、可见范围、重复创建策略与撤销能力在 service 层已覆盖。
func (*RDIApi) CreateShareToken(c *gin.Context) {
	var req model.RDIShareTokenReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	data, err := service.GroupApp.RDI.CreateShareToken(deviceID, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// AcceptSharedDevice 接受设备分享。
// 核心链路：读取分享 token 与当前用户 claims，委托 service 完成分享校验、设备挂接和结果返回。
// 审查重点：确认 token 一次性使用、租户边界和重复接受行为是否符合预期。
func (*RDIApi) AcceptSharedDevice(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	token := c.Param("token")
	data, err := service.GroupApp.RDI.AcceptSharedDevice(token, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// SharedDevices 查询共享设备列表。
// 审查重点：检查分页条件、分享状态过滤和返回字段最小化，避免将原始拥有者侧隐私数据带出。
func (*RDIApi) SharedDevices(c *gin.Context) {
	var req model.RDISharedDeviceListReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.RDI.SharedDevices(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// RevokeShareToken 撤销设备分享令牌。
// 核心链路：从路径读取 device_id 与 token，由 service 在设备行锁内移除该 token 及其接收人。
// 审查重点：确认仅设备归属者与管理员可撤销，且被撤销后接收人立即失去读权限。
// @Summary 撤销设备分享令牌
// @Description 设备归属者主动作废一个分享令牌；通过该令牌接受分享的接收人会同时失去访问权限。
// @Tags RDI设备
// @Accept json
// @Produce json
// @Param device_id path string true "设备ID"
// @Param token path string true "分享令牌"
// @Success 200 {object} model.RDIRevokeShareResponse
// @Router /api/v1/rdi/devices/{device_id}/share-tokens/{token} [delete]
func (*RDIApi) RevokeShareToken(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	token := c.Param("token")
	data, err := service.GroupApp.RDI.RevokeShareToken(deviceID, token, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// RevokeSharedDeviceRecipient 撤销指定接收人的设备访问权限。
// 核心链路：从路径读取 device_id 与 user_id，由 service 在设备行锁内移除该接收人记录。
// 审查重点：确认仅设备归属者与管理员可撤销，接收人本身不得撤销他人或自身以外的记录。
// @Summary 撤销指定接收人的设备分享
// @Description 设备归属者主动收回某个接收人的共享访问权限；该接收人随后不再能读取设备数据。
// @Tags RDI设备
// @Accept json
// @Produce json
// @Param device_id path string true "设备ID"
// @Param user_id path string true "接收人用户ID"
// @Success 200 {object} model.RDIRevokeShareResponse
// @Router /api/v1/rdi/devices/{device_id}/share-recipients/{user_id} [delete]
func (*RDIApi) RevokeSharedDeviceRecipient(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	deviceID := devicePathID(c)
	userID := c.Param("user_id")
	data, err := service.GroupApp.RDI.RevokeSharedDeviceRecipient(deviceID, userID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// SharedDeviceConfig 获取分享设备配置。
// 核心链路：通过分享 token 查询共享视角下允许暴露的设备配置。
// 审查重点：确认该接口不会绕过登录态或 token 范围约束泄露完整配置。
func (*RDIApi) SharedDeviceConfig(c *gin.Context) {
	token := c.Param("token")
	data, err := service.GroupApp.RDI.SharedDeviceConfig(token)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
