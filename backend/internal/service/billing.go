package service

import (
	"math"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/google/uuid"
)

type BillingService struct{}

// ListPlans 列出系统支持的所有有效套餐
func (s *BillingService) ListPlans() ([]*model.SubscriptionPlan, error) {
	return dal.GetSubscriptionPlans()
}

// GetTenantUsage 获取租户当前套餐用量报告与配额消耗
func (s *BillingService) GetTenantUsage(tenantID string) (*model.BillingUsageReport, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant_id is required")
	}

	// 1. 获取租户当前订阅，若不存在则默认归为 free 免费社区版
	sub, err := dal.GetTenantSubscription(tenantID)
	if err != nil {
		return nil, err
	}

	planCode := "free"
	subStatus := "active"
	periodStart := time.Now().UTC()
	periodEnd := periodStart.Add(30 * 24 * time.Hour)

	if sub != nil {
		planCode = sub.PlanCode
		subStatus = sub.Status
		if !sub.CurrentPeriodStart.IsZero() {
			periodStart = sub.CurrentPeriodStart
		}
		if !sub.CurrentPeriodEnd.IsZero() {
			periodEnd = sub.CurrentPeriodEnd
		}
	}

	// 2. 获取套餐详情
	plan, err := dal.GetSubscriptionPlanByCode(planCode)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		// 容错回退
		plan = &model.SubscriptionPlan{
			Code:               planCode,
			Name:               strings.ToUpper(planCode),
			MaxDevices:         10,
			MaxTenants:         1,
			MaxUsers:           3,
			MaxTelemetryPerDay: 10000,
			Features:           "[]",
		}
	}

	// 3. 统计租户当前各项资源实体消耗
	deviceCount, userCount, subTenantCount, telemetryCount, err := dal.GetTenantUsageCounts(tenantID)
	if err != nil {
		return nil, err
	}

	// 4. 计算百分比（保留一位小数）
	calcPct := func(used int64, maxVal int) float64 {
		if maxVal <= 0 {
			return 0.0
		}
		pct := (float64(used) / float64(maxVal)) * 100.0
		return math.Round(pct*10) / 10
	}

	deviceUsagePct := calcPct(deviceCount, plan.MaxDevices)
	tenantUsagePct := calcPct(subTenantCount, plan.MaxTenants)
	userUsagePct := calcPct(userCount, plan.MaxUsers)
	telemetryUsagePct := calcPct(telemetryCount, plan.MaxTelemetryPerDay)

	// 5. 判定配额状态: normal (<80%), warning (>=80% 且 <100%), exceeded (>=100%)
	quotaStatus := "normal"
	maxPct := math.Max(deviceUsagePct, math.Max(tenantUsagePct, math.Max(userUsagePct, telemetryUsagePct)))
	if maxPct >= 100.0 {
		quotaStatus = "exceeded"
	} else if maxPct >= 80.0 {
		quotaStatus = "warning"
	}

	report := &model.BillingUsageReport{
		TenantID:             tenantID,
		PlanCode:             plan.Code,
		PlanName:             plan.Name,
		Status:               subStatus,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
		DeviceCount:          deviceCount,
		MaxDevices:           plan.MaxDevices,
		DeviceUsagePct:       deviceUsagePct,
		TenantCount:          subTenantCount,
		MaxTenants:           plan.MaxTenants,
		TenantUsagePct:       tenantUsagePct,
		UserCount:            userCount,
		MaxUsers:             plan.MaxUsers,
		UserUsagePct:         userUsagePct,
		TelemetryPointsCount: telemetryCount,
		MaxTelemetryPerDay:   plan.MaxTelemetryPerDay,
		TelemetryUsagePct:    telemetryUsagePct,
		QuotaStatus:          quotaStatus,
		Features:             plan.ParsedFeatures(),
	}

	return report, nil
}

// SubscribePlan 为租户订购或变更套餐
func (s *BillingService) SubscribePlan(targetTenantID, planCode string, operatorRole string, operatorTenantID string) (*model.TenantSubscription, error) {
	targetTenantID = strings.TrimSpace(targetTenantID)
	planCode = strings.TrimSpace(planCode)
	if planCode == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "plan_code is required")
	}

	// 权限约束: 非平台超管 SYS_ADMIN 只能为本租户订购
	if operatorRole != "SYS_ADMIN" {
		if targetTenantID == "" {
			targetTenantID = operatorTenantID
		} else if targetTenantID != operatorTenantID {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "permission denied: cannot subscribe for other tenants")
		}
	} else if targetTenantID == "" {
		targetTenantID = operatorTenantID
	}

	if targetTenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant_id is required")
	}

	// 校验套餐是否存在
	plan, err := dal.GetSubscriptionPlanByCode(planCode)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "subscription plan not found: "+planCode)
	}

	now := time.Now().UTC()
	sub := &model.TenantSubscription{
		ID:                 uuid.New().String(),
		TenantID:           targetTenantID,
		PlanCode:           plan.Code,
		Status:             "active",
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
		CancelAtPeriodEnd:  false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := dal.UpsertTenantSubscription(sub); err != nil {
		return nil, err
	}

	return sub, nil
}
