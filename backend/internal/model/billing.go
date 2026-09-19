package model

import (
	"encoding/json"
	"time"
)

// SubscriptionPlan 套餐定义
type SubscriptionPlan struct {
	ID                 string    `gorm:"column:id;primaryKey" json:"id"`
	Code               string    `gorm:"column:code;not null;unique" json:"code"`
	Name               string    `gorm:"column:name;not null" json:"name"`
	Description        string    `gorm:"column:description" json:"description"`
	PriceMonthly       float64   `gorm:"column:price_monthly" json:"price_monthly"`
	Currency           string    `gorm:"column:currency" json:"currency"`
	MaxDevices         int       `gorm:"column:max_devices" json:"max_devices"`
	MaxTenants         int       `gorm:"column:max_tenants" json:"max_tenants"`
	MaxUsers           int       `gorm:"column:max_users" json:"max_users"`
	MaxTelemetryPerDay int       `gorm:"column:max_telemetry_per_day" json:"max_telemetry_per_day"`
	MaxApiCallsPerDay  int       `gorm:"column:max_api_calls_per_day" json:"max_api_calls_per_day"`
	Features           string    `gorm:"column:features;type:jsonb" json:"features"`
	Enabled            int       `gorm:"column:enabled" json:"enabled"`
	CreatedAt          time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (*SubscriptionPlan) TableName() string {
	return "subscription_plans"
}

// ParsedFeatures 解析套餐包含的特性列表
func (p *SubscriptionPlan) ParsedFeatures() []string {
	if p.Features == "" {
		return []string{}
	}
	var res []string
	if err := json.Unmarshal([]byte(p.Features), &res); err != nil {
		return []string{}
	}
	return res
}

// TenantSubscription 租户当前订阅关系
type TenantSubscription struct {
	ID                 string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID           string    `gorm:"column:tenant_id;not null;unique" json:"tenant_id"`
	PlanCode           string    `gorm:"column:plan_code;not null" json:"plan_code"`
	Status             string    `gorm:"column:status;not null" json:"status"`
	CurrentPeriodStart time.Time `gorm:"column:current_period_start" json:"current_period_start"`
	CurrentPeriodEnd   time.Time `gorm:"column:current_period_end" json:"current_period_end"`
	CancelAtPeriodEnd  bool      `gorm:"column:cancel_at_period_end" json:"cancel_at_period_end"`
	CreatedAt          time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (*TenantSubscription) TableName() string {
	return "tenant_subscriptions"
}

// BillingUsageReport 租户用量与配额消耗报告
type BillingUsageReport struct {
	TenantID             string    `json:"tenant_id"`
	PlanCode             string    `json:"plan_code"`
	PlanName             string    `json:"plan_name"`
	Status               string    `json:"status"`
	CurrentPeriodStart   time.Time `json:"current_period_start"`
	CurrentPeriodEnd     time.Time `json:"current_period_end"`
	DeviceCount          int64     `json:"device_count"`
	MaxDevices           int       `json:"max_devices"`
	DeviceUsagePct       float64   `json:"device_usage_pct"`
	TenantCount          int64     `json:"tenant_count"`
	MaxTenants           int       `json:"max_tenants"`
	TenantUsagePct       float64   `json:"tenant_usage_pct"`
	UserCount            int64     `json:"user_count"`
	MaxUsers             int       `json:"max_users"`
	UserUsagePct         float64   `json:"user_usage_pct"`
	TelemetryPointsCount int64     `json:"telemetry_points_count"`
	MaxTelemetryPerDay   int       `json:"max_telemetry_per_day"`
	TelemetryUsagePct    float64   `json:"telemetry_usage_pct"`
	QuotaStatus          string    `json:"quota_status"` // normal, warning, exceeded
	Features             []string  `json:"features"`
}

// SubscribePlanReq 订购或变更套餐请求
type SubscribePlanReq struct {
	PlanCode string `json:"plan_code" binding:"required"`
	TenantID string `json:"tenant_id"`
}
