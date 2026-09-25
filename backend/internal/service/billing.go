package service

import (
	"context"
	"math"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/quota"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
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

// GetTenantAPIQuota 获取租户今日 API 配额报告（TB-17）：今日调用数、套餐限额与剩余量。
// 限额与状态复用 internal/quota 的执法判定纯函数，保证展示口径与 429 执法口径一致；
// 计量读取 fail-open——读取失败按 0 展示而非报错（查询端点不承载执法职责）。
func (s *BillingService) GetTenantAPIQuota(ctx context.Context, tenantID string) (*model.APIQuotaReport, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "tenant_id is required")
	}

	// 1. 订阅套餐：无订阅归 free（与 GetTenantUsage 同一口径）
	sub, err := dal.GetTenantSubscription(tenantID)
	if err != nil {
		return nil, err
	}
	planCode := "free"
	if sub != nil && sub.PlanCode != "" {
		planCode = sub.PlanCode
	}
	plan, err := dal.GetSubscriptionPlanByCode(planCode)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		// 容错回退：套餐目录缺失时按"未配置阈值"展示（不限量），不阻断查询。
		plan = &model.SubscriptionPlan{Code: planCode, Name: strings.ToUpper(planCode)}
	}

	// 2. 今日已用调用数（Redis 权威值 / 进程镜像 / DB 快照三级回落；失败按 0 展示）
	used, usageDate, err := quota.Default().TodayUsage(ctx, tenantID)
	if err != nil {
		logrus.WithError(err).Warnf("billing: read today api usage failed for tenant %s, report 0", tenantID)
		usageDate = quota.UsageDate(time.Now())
	}

	// 3. 限额与状态判定复用执法纯函数（展示与执法同口径）
	decision := quota.DecideDailyQuota(int64(plan.MaxApiCallsPerDay), used, time.Now())

	return &model.APIQuotaReport{
		TenantID:          tenantID,
		Date:              usageDate,
		PlanCode:          plan.Code,
		APICallsToday:     used,
		MaxAPICallsPerDay: int64(plan.MaxApiCallsPerDay),
		Remaining:         decision.Remaining,
		UsagePct:          decision.UsagePct,
		QuotaStatus:       decision.Status,
	}, nil
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
