package api

import (
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type BillingApi struct{}

// ListPlans 获取可用套餐列表
// @Summary  获取可用套餐列表
// @Tags     Billing
// @Router   /api/v1/billing/plans [get]
func (*BillingApi) ListPlans(c *gin.Context) {
	plans, err := service.GroupApp.Billing.ListPlans()
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", plans)
}

// GetUsage 获取当前租户的用量计量与配额报告
// @Summary  获取租户用量与配额报告
// @Tags     Billing
// @Router   /api/v1/billing/usage [get]
func (*BillingApi) GetUsage(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	targetTenantID := strings.TrimSpace(c.Query("tenant_id"))

	// 权限防护: 非平台超管不能查别的租户用量
	if claims.Authority != "SYS_ADMIN" {
		if targetTenantID != "" && targetTenantID != claims.TenantID {
			c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "permission denied: cannot view usage for other tenants"))
			return
		}
		targetTenantID = claims.TenantID
	} else if targetTenantID == "" {
		targetTenantID = claims.TenantID
	}

	report, err := service.GroupApp.Billing.GetTenantUsage(targetTenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", report)
}

// SubscribePlan 订购或变更套餐
// @Summary  订购或变更套餐
// @Tags     Billing
// @Router   /api/v1/billing/subscriptions [post]
func (*BillingApi) SubscribePlan(c *gin.Context) {
	var req model.SubscribePlanReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)

	sub, err := service.GroupApp.Billing.SubscribePlan(req.TenantID, req.PlanCode, claims.Authority, claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", sub)
}
