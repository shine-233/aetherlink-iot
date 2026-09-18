// File purpose: TB-12 设备认领 HTTP 层——签发/列表/撤销/赎回四个端点。
// Core logic: 绑定校验 + claims 提取 + 委派 service；租户上下文只来自 claims，
// 认领方不需要也不能传 tenant_id（防越权指认）。
// Key notes: 明文 claim_key 只在签发响应体里出现一次，任何列表接口都不回显。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceClaimApi struct{}

// IssueDeviceClaimToken 签发一次性认领令牌。
// @Summary  签发设备认领令牌
// @Tags     DeviceClaim
// @Router   /api/v1/device/claim-tokens [post]
func (*DeviceClaimApi) IssueDeviceClaimToken(c *gin.Context) {
	var req model.IssueDeviceClaimTokenReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceClaim.IssueClaimToken(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// ListDeviceClaimTokens 签发方回查某设备的令牌历史（无明文无哈希）。
// @Summary  查询设备认领令牌历史
// @Tags     DeviceClaim
// @Router   /api/v1/device/claim-tokens [get]
func (*DeviceClaimApi) ListDeviceClaimTokens(c *gin.Context) {
	deviceID := c.Query("device_id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceClaim.ListClaimTokens(c.Request.Context(), deviceID, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// RevokeDeviceClaimToken 撤销 active 令牌。
// @Summary  撤销设备认领令牌
// @Tags     DeviceClaim
// @Router   /api/v1/device/claim-tokens/{token_id} [delete]
func (*DeviceClaimApi) RevokeDeviceClaimToken(c *gin.Context) {
	tokenID := c.Param("token_id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.DeviceClaim.RevokeClaimToken(c.Request.Context(), tokenID, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"revoked": true})
}

// RedeemDeviceClaim 认领设备（设备从签发租户转移到认领租户）。
// @Summary  认领设备
// @Tags     DeviceClaim
// @Router   /api/v1/device/claim-tokens/redeem [post]
func (*DeviceClaimApi) RedeemDeviceClaim(c *gin.Context) {
	var req model.RedeemDeviceClaimReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceClaim.RedeemClaim(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
