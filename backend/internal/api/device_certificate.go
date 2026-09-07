// 文件用途：接入安全 X.509（ROADMAP D5）HTTP 入口。
// 边界说明：租户边界在 service 层处理；本层只做绑定、claims 提取与错误出口。
package api

import (
	"strconv"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

type DeviceCertificateApi struct{}

// Issue 为设备签发 X.509 证书（私钥仅本响应返回一次）。
// POST /api/v1/device-certificates/issue
func (*DeviceCertificateApi) Issue(c *gin.Context) {
	var req model.IssueDeviceCertificateReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceCertificate.IssueDeviceCertificate(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// List 租户内列设备证书，可按 device_id 过滤。
// GET /api/v1/device-certificates?device_id=&limit=
func (*DeviceCertificateApi) List(c *gin.Context) {
	deviceID := c.Query("device_id")
	limit := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceCertificate.ListDeviceCertificates(deviceID, limit, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Get 证书详情。
// GET /api/v1/device-certificates/:id
func (*DeviceCertificateApi) Get(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceCertificate.GetDeviceCertificate(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Revoke 吊销证书。
// POST /api/v1/device-certificates/:id/revoke
func (*DeviceCertificateApi) Revoke(c *gin.Context) {
	var req struct {
		Reason string `json:"reason" validate:"omitempty,max=255"`
	}
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	body := &model.RevokeDeviceCertificateReq{ID: c.Param("id"), Reason: req.Reason}
	resp, err := service.GroupApp.DeviceCertificate.RevokeDeviceCertificate(body, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Renew 轮换证书：签发新证书并吊销旧证书。
// POST /api/v1/device-certificates/:id/renew
func (*DeviceCertificateApi) Renew(c *gin.Context) {
	var req struct {
		ValidityDays int `json:"validity_days" validate:"omitempty,min=1,max=3650"`
	}
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	body := &model.RenewDeviceCertificateReq{ID: c.Param("id"), ValidityDays: req.ValidityDays}
	resp, err := service.GroupApp.DeviceCertificate.RenewDeviceCertificate(body, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Verify 校验证书链与吊销状态（broker mTLS 认证后端的人工复核入口）。
// POST /api/v1/device-certificates/verify
func (*DeviceCertificateApi) Verify(c *gin.Context) {
	var req model.VerifyDeviceCertificateReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.DeviceCertificate.VerifyDeviceCertificate(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
