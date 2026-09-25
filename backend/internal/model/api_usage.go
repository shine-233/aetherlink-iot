// 文件用途：TB-17 租户 API 日配额的数据模型——api_usage_daily 计量表与配额查询报告 DTO。
// 核心逻辑：ApiUsageDaily 按 (tenant_id, usage_date) 唯一落每日累计调用数；APIQuotaReport 是
// GET /api/v1/billing/api-quota 的响应体（今日调用数/限额/剩余/百分比/状态）。
// 关键注意事项：日窗口以 UTC 日期切分（与 internal/quota 的 UsageDate 同口径）；
// Remaining = -1 表示套餐未设置执法阈值（不限量），前端据此显示"不限量"而非数字。
// 重构建议：若后续扩展 transport 维度计量，可在此文件平级新增 transport_usage_daily 而非复用本表。
package model

import "time"

// ApiUsageDaily 租户 API 日调用计量（TB-17，对标 TB PE per-tenant API quotas）。
type ApiUsageDaily struct {
	TenantID  string    `gorm:"column:tenant_id;primaryKey" json:"tenant_id"`
	UsageDate time.Time `gorm:"column:usage_date;primaryKey" json:"usage_date"`
	APICalls  int64     `gorm:"column:api_calls" json:"api_calls"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (*ApiUsageDaily) TableName() string {
	return "api_usage_daily"
}

// 配额状态常量（与 billing 的 quota_status 语义对齐，另增 unlimited 表示未设阈值）。
const (
	QuotaStatusNormal    = "normal"    // <80%
	QuotaStatusWarning   = "warning"   // >=80% 且 <100%
	QuotaStatusExceeded  = "exceeded"  // >=100%
	QuotaStatusUnlimited = "unlimited" // 限额 <=0：未配置执法阈值，不执法
)

// APIQuotaReport GET /api/v1/billing/api-quota 响应：当前租户今日 API 调用数、限额与剩余量。
type APIQuotaReport struct {
	TenantID          string  `json:"tenant_id"`
	Date              string  `json:"date"`                  // 计量日（UTC，YYYY-MM-DD）
	PlanCode          string  `json:"plan_code"`             // 订阅套餐代码（无订阅归 free）
	APICallsToday     int64   `json:"api_calls_today"`       // 今日已调用次数
	MaxAPICallsPerDay int64   `json:"max_api_calls_per_day"` // 套餐日调用限额；<=0 表示不限量
	Remaining         int64   `json:"remaining"`             // 剩余额度；-1 表示不限量
	UsagePct          float64 `json:"usage_pct"`             // 已用百分比（一位小数；不限量为 0）
	QuotaStatus       string  `json:"quota_status"`          // normal | warning | exceeded | unlimited
}
