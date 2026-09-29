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

// GetAPIQuota 获取当前租户今日 API 配额（TB-17）：今日调用数 / 套餐限额 / 剩余量。
// @Summary  获取租户今日 API 配额
// @Tags     Billing
// @Router   /api/v1/billing/api-quota [get]
func (*BillingApi) GetAPIQuota(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	targetTenantID := strings.TrimSpace(c.Query("tenant_id"))

	// 权限防护: 非平台超管不能查别的租户配额（与 GetUsage 同一防线）
	if claims.Authority != "SYS_ADMIN" {
		if targetTenantID != "" && targetTenantID != claims.TenantID {
			c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "permission denied: cannot view api quota for other tenants"))
			return
		}
		targetTenantID = claims.TenantID
	} else if targetTenantID == "" {
		targetTenantID = claims.TenantID
	}

	report, err := service.GroupApp.Billing.GetTenantAPIQuota(c.Request.Context(), targetTenantID)
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
	Handle(c, func(req *model.SubscribePlanReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Billing.SubscribePlan(req.TenantID, req.PlanCode, claims.Authority, claims.TenantID)
	})
}
