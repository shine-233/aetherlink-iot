package dal

import (
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

var (
	errBillingDBNotInitialized = errors.New("database not initialized for billing")
)

// GetSubscriptionPlans 获取所有启用的套餐列表，按价格升序排序。
// tenant-scope: system-table —— 套餐是平台全局商业目录，无租户维度，所有租户共享同一份。
func GetSubscriptionPlans() ([]*model.SubscriptionPlan, error) {
	if global.DB == nil {
		return nil, errBillingDBNotInitialized
	}
	var plans []*model.SubscriptionPlan
	err := global.DB.Table("subscription_plans").
		Where("enabled = ?", 1).
		Order("price_monthly ASC").
		Find(&plans).Error
	return plans, err
}

// GetSubscriptionPlanByCode 根据套餐代码查询套餐。
// tenant-scope: system-table —— 同上，套餐目录平台级共享；code 唯一不随租户变化。
func GetSubscriptionPlanByCode(code string) (*model.SubscriptionPlan, error) {
	if global.DB == nil {
		return nil, errBillingDBNotInitialized
	}
	var plan model.SubscriptionPlan
	err := global.DB.Table("subscription_plans").
		Where("code = ?", strings.TrimSpace(code)).
		Take(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

// GetTenantSubscription 获取租户当前的订阅记录
func GetTenantSubscription(tenantID string) (*model.TenantSubscription, error) {
	if global.DB == nil {
		return nil, errBillingDBNotInitialized
	}
	var sub model.TenantSubscription
	err := global.DB.Table("tenant_subscriptions").
		Where("tenant_id = ?", strings.TrimSpace(tenantID)).
		Take(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

// UpsertTenantSubscription 创建或更新租户订阅
func UpsertTenantSubscription(sub *model.TenantSubscription) error {
	if global.DB == nil {
		return errBillingDBNotInitialized
	}
	sub.UpdatedAt = time.Now().UTC()
	var existing model.TenantSubscription
	err := global.DB.Table("tenant_subscriptions").Where("tenant_id = ?", sub.TenantID).Take(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if sub.CreatedAt.IsZero() {
				sub.CreatedAt = time.Now().UTC()
			}
			return global.DB.Table("tenant_subscriptions").Create(sub).Error
		}
		return err
	}
	// 更新现有订阅
	updates := map[string]interface{}{
		"plan_code":             sub.PlanCode,
		"status":                sub.Status,
		"current_period_start":  sub.CurrentPeriodStart,
		"current_period_end":    sub.CurrentPeriodEnd,
		"cancel_at_period_end": sub.CancelAtPeriodEnd,
		"updated_at":            sub.UpdatedAt,
	}
	return global.DB.Table("tenant_subscriptions").Where("tenant_id = ?", sub.TenantID).Updates(updates).Error
}

// GetTenantUsageCounts 统计租户当前指标消耗
func GetTenantUsageCounts(tenantID string) (deviceCount, userCount, subTenantCount, telemetryCount int64, err error) {
	if global.DB == nil {
		return 0, 0, 0, 0, errBillingDBNotInitialized
	}
	tenantID = strings.TrimSpace(tenantID)

	// 1. 设备数
	if err := global.DB.Table(model.TableNameDevice).Where("tenant_id = ?", tenantID).Count(&deviceCount).Error; err != nil {
		return 0, 0, 0, 0, err
	}

	// 2. 用户数
	if err := global.DB.Table(model.TableNameUser).Where("tenant_id = ?", tenantID).Count(&userCount).Error; err != nil {
		return 0, 0, 0, 0, err
	}

	// 3. 子租户数
	if err := global.DB.Table(model.TableNameTenant).Where("parent_tenant_id = ?", tenantID).Count(&subTenantCount).Error; err != nil {
		return 0, 0, 0, 0, err
	}

	// 4. 遥测点数 (通过关联设备统计)
	if err := global.DB.Table("telemetry_datas").
		Joins("JOIN devices ON devices.id = telemetry_datas.device_id").
		Where("devices.tenant_id = ?", tenantID).
		Count(&telemetryCount).Error; err != nil {
		return 0, 0, 0, 0, err
	}

	return deviceCount, userCount, subTenantCount, telemetryCount, nil
}
