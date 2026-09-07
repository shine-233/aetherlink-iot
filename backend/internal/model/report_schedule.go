package model

import "time"

const TableNameReportSchedule = "report_schedules"

// ReportSchedule 定时报表任务（ROADMAP D3）。
// 按 cron 表达式周期性导出租户指定设备/测点的遥测数据为 CSV，并经 D2 邮件渠道投递给收件人。
type ReportSchedule struct {
	ID            string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID      string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	Name          string    `gorm:"column:name;not null" json:"name"`
	CronExpr      string    `gorm:"column:cron_expr;not null" json:"cron_expr"`
	Recipients    string    `gorm:"column:recipients;type:text;not null" json:"recipients"`
	DeviceIDs     []string  `gorm:"column:device_ids;type:jsonb;serializer:json" json:"device_ids"`
	Keys          []string  `gorm:"column:keys;type:jsonb;serializer:json" json:"keys"`
	LookbackHours int       `gorm:"column:lookback_hours;not null;default:24" json:"lookback_hours"`
	Format        string    `gorm:"column:format;not null;default:csv" json:"format"`
	Enabled       bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	LastRunAt     *time.Time `gorm:"column:last_run_at" json:"last_run_at"`
	LastStatus    string    `gorm:"column:last_status" json:"last_status"`
	CreatedAt     time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*ReportSchedule) TableName() string { return TableNameReportSchedule }

// ---- HTTP 请求结构体 ----

// CreateReportScheduleReq 创建定时报表任务。
type CreateReportScheduleReq struct {
	Name          string   `json:"name" validate:"required,max=128"`
	CronExpr      string   `json:"cron_expr" validate:"required"`
	Recipients    string   `json:"recipients" validate:"required"`
	DeviceIDs     []string `json:"device_ids" validate:"required,min=1,dive,max=36"`
	Keys          []string `json:"keys" validate:"required,min=1,dive,max=255"`
	LookbackHours int      `json:"lookback_hours" validate:"omitempty,min=1,max=8760"`
	Format        string   `json:"format" validate:"omitempty,oneof=csv"`
	Enabled       bool     `json:"enabled"`
}

// UpdateReportScheduleReq 更新定时报表任务（部分字段可选）。
type UpdateReportScheduleReq struct {
	ID            string   `json:"id" validate:"required"`
	Name          string   `json:"name" validate:"omitempty,max=128"`
	CronExpr      string   `json:"cron_expr" validate:"omitempty"`
	Recipients    string   `json:"recipients" validate:"omitempty"`
	DeviceIDs     []string `json:"device_ids" validate:"omitempty,min=1,dive,max=36"`
	Keys          []string `json:"keys" validate:"omitempty,min=1,dive,max=255"`
	LookbackHours int      `json:"lookback_hours" validate:"omitempty,min=1,max=8760"`
	Format        string   `json:"format" validate:"omitempty,oneof=csv"`
	Enabled       *bool    `json:"enabled"`
}
