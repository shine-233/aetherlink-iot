// 文件用途：2FA（ROADMAP C7）HTTP 入口——绑定管理 + 登录第二因子。
// 边界说明：secret/恢复码只出现在响应一次；防重放与加密在 service 层完成。
package api

import (
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type UserTotpApi struct{}

type TotpBindReq struct {
	Code string `json:"code" binding:"required"`
}

// HandleTotpSetup 生成一次性绑定材料（otpauth URI）。
// GET /api/v1/user/totp/setup
func (*UserTotpApi) HandleTotpSetup(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.UserTotp.Setup(userClaims.ID, userClaims.Email)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleTotpActivate 用验证码激活 2FA，返回一次性恢复码。
// POST /api/v1/user/totp/activate
func (*UserTotpApi) HandleTotpActivate(c *gin.Context) {
	var req TotpBindReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.UserTotp.Activate(userClaims.ID, userClaims.Email, req.Code)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleTotpDisable 解绑 2FA（需当前 TOTP 或恢复码）。
// POST /api/v1/user/totp/disable
func (*UserTotpApi) HandleTotpDisable(c *gin.Context) {
	var req TotpBindReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.UserTotp.Disable(userClaims.ID, req.Code); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

// HandleTotpStatus 查询当前用户 2FA 状态。
// GET /api/v1/user/totp/status
func (*UserTotpApi) HandleTotpStatus(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	enabled, err := service.GroupApp.UserTotp.Status(userClaims.ID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"enabled": enabled})
}

// LoginWithTotp 登录第二因子（公开端点，ticket 由第一步 /login 下发）。
// POST /api/v1/login/totp
func (*UserApi) LoginWithTotp(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket" binding:"required"`
		Code   string `json:"code" binding:"required"`
	}
	if !BindAndValidate(c, &req) {
		return
	}
	loginRsp, err := service.GroupApp.UserTotp.LoginWithSecondFactor(req.Ticket, req.Code)
	if err != nil {
		c.Error(err)
		return
	}
	setAuthCookieForLoginResponse(c, loginRsp)
	c.Set("data", loginRsp)
}

// handleTotpChallengeError 当 /login 返回 CodeTotpRequired 时，把挑战下发为正常响应而非失败。
func handleTotpChallengeError(c *gin.Context, e *errcode.Error) bool {
	if e == nil || e.Code != errcode.CodeTotpRequired {
		return false
	}
	payload := map[string]interface{}{"step": "totp"}
	if ticket, ok := e.Data.(map[string]interface{})["ticket"]; ok {
		payload["ticket"] = ticket
	}
	c.Set("data", payload)
	return true
}
