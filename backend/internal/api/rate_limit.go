package api

import (
	"errors"
	"net/http"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/ratelimit"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RateLimitApi struct{}

// GetConfig 获取当前集群/系统生效的限流配置。
func (a *RateLimitApi) GetConfig(c *gin.Context) {
	svc := ratelimit.GetDefaultService()
	cfg := svc.GetConfig()
	c.Set("data", cfg)
}

// GetMetrics 获取限流拦截计数与度量指标。
func (a *RateLimitApi) GetMetrics(c *gin.Context) {
	svc := ratelimit.GetDefaultService()
	metrics := svc.GetMetrics()
	c.Set("data", metrics)
}

// ListOverrides 获取当前租户下（或超管全量）的自定义限流配额。
func (a *RateLimitApi) ListOverrides(c *gin.Context) {
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.New(errcode.CodeUnauthorized))
		return
	}
	claims, ok := claimsVal.(*utils.UserClaims)
	if !ok || claims == nil {
		c.Error(errcode.New(errcode.CodeUnauthorized))
		return
	}

	tenantID := claims.TenantID
	if claims.Authority == "SYS_ADMIN" && c.Query("tenant_id") != "" {
		tenantID = c.Query("tenant_id")
	}

	svc := ratelimit.GetDefaultService()
	list, err := svc.ListOverrides(c.Request.Context(), tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", list)
}

// SetOverride 创建或更新租户/设备限流配额规则。
func (a *RateLimitApi) SetOverride(c *gin.Context) {
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.New(errcode.CodeUnauthorized))
		return
	}
	claims, ok := claimsVal.(*utils.UserClaims)
	if !ok || claims == nil {
		c.Error(errcode.New(errcode.CodeUnauthorized))
		return
	}

	var req model.SetRateLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
		return
	}

	// 租户管理员只能设置本租户下的规则
	if claims.Authority != "SYS_ADMIN" {
		if req.TenantID != "" && req.TenantID != claims.TenantID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    errcode.CodeNoPermission,
				"message": "forbidden: cannot modify quota for another tenant",
			})
			return
		}
		req.TenantID = claims.TenantID
	} else if req.TenantID == "" {
		req.TenantID = claims.TenantID
	}

	if req.TargetType == model.TargetTypeTenant && (req.TargetID == "" || req.TargetID == "self") {
		req.TargetID = req.TenantID
	}

	// 校验规则语法 (例如: "100:1,1000:60")
	if err := ratelimit.ValidateRateLimitString(req.RateLimits); err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "invalid rate_limits format: "+err.Error()))
		return
	}

	// 校验 target_type 与 limit_type 合法性
	if req.TargetType != model.TargetTypeTenant && req.TargetType != model.TargetTypeDevice {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "target_type must be 'tenant' or 'device'"))
		return
	}
	if req.LimitType != model.LimitTypeAPI && req.LimitType != model.LimitTypeTenantTransport && req.LimitType != model.LimitTypeDeviceTransport {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "limit_type must be 'api', 'tenant_transport', or 'device_transport'"))
		return
	}

	svc := ratelimit.GetDefaultService()
	if err := svc.SetOverride(c.Request.Context(), &req); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"status": "ok"})
}

// DeleteOverride 删除自定义限流配额，回退至系统默认配置。
func (a *RateLimitApi) DeleteOverride(c *gin.Context) {
	claimsVal, exists := c.Get("claims")
	if !exists {
		c.Error(errcode.New(errcode.CodeUnauthorized))
		return
	}
	claims, ok := claimsVal.(*utils.UserClaims)
	if !ok || claims == nil {
		c.Error(errcode.New(errcode.CodeUnauthorized))
		return
	}

	targetType := c.Param("target_type")
	targetID := c.Param("target_id")
	limitType := c.Query("limit_type")
	if limitType == "" {
		limitType = model.LimitTypeAPI
	}

	tenantID := claims.TenantID
	if claims.Authority == "SYS_ADMIN" && c.Query("tenant_id") != "" {
		tenantID = c.Query("tenant_id")
	} else if claims.Authority != "SYS_ADMIN" && targetType == model.TargetTypeTenant {
		targetID = claims.TenantID
	}

	svc := ratelimit.GetDefaultService()
	if err := svc.DeleteOverride(c.Request.Context(), tenantID, targetType, targetID, limitType); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(errcode.New(errcode.CodeNotFound))
			return
		}
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"status": "ok"})
}
