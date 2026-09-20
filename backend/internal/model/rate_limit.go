package model

import (
	"time"
)

// TargetType 常量定义。
const (
	TargetTypeTenant = "tenant"
	TargetTypeDevice = "device"
)

// LimitType 常量定义（对标 ThingsBoard 的三层限流）。
const (
	LimitTypeAPI             = "api"              // REST API 调用限流
	LimitTypeTenantTransport = "tenant_transport" // 租户全量上行消息限流
	LimitTypeDeviceTransport = "device_transport" // 单设备上行消息限流
)

// TenantRateLimit 数据库模型（表 tenant_rate_limits）。
type TenantRateLimit struct {
	ID          string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;size:36;not null;index" json:"tenant_id"`
	TargetType  string    `gorm:"column:target_type;size:20;not null;index" json:"target_type"`
	TargetID    string    `gorm:"column:target_id;size:64;not null;index" json:"target_id"`
	LimitType   string    `gorm:"column:limit_type;size:20;not null" json:"limit_type"`
	RateLimits  string    `gorm:"column:rate_limits;size:128;not null" json:"rate_limits"`
	Enabled     bool      `gorm:"column:enabled;default:true" json:"enabled"`
	Description string    `gorm:"column:description;size:255" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (TenantRateLimit) TableName() string {
	return "tenant_rate_limits"
}

// SetRateLimitRequest 创建或更新限流规则请求 DTO。
type SetRateLimitRequest struct {
	TenantID    string `json:"tenant_id"`                      // 可选，租户管理员自动填充自身 tenant_id
	TargetType  string `json:"target_type" binding:"required"` // tenant | device
	TargetID    string `json:"target_id"`                      // 可选，当 target_type=tenant 时默认同 tenant_id
	LimitType   string `json:"limit_type" binding:"required"`  // api | tenant_transport | device_transport
	RateLimits  string `json:"rate_limits" binding:"required"` // e.g. "100:1,1000:60"
	Enabled     *bool  `json:"enabled"`
	Description string `json:"description"`
}

// RateLimitConfigResponse 响应全局配置 DTO。
type RateLimitConfigResponse struct {
	Backend               string `json:"backend"`
	DefaultTenantAPILimit string `json:"default_tenant_api_limit"`
	DefaultTenantTxLimit  string `json:"default_tenant_transport_limit"`
	DefaultDeviceTxLimit  string `json:"default_device_transport_limit"`
}

// RateLimitMetricsResponse 响应度量指标 DTO。
type RateLimitMetricsResponse struct {
	TotalChecked  int64            `json:"total_checked"`
	TotalAllowed  int64            `json:"total_allowed"`
	TotalBlocked  int64            `json:"total_blocked"`
	BlockedByScope map[string]int64 `json:"blocked_by_scope"`
}
