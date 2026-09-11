package model

import "time"

const TableNameReportSchedule = "report_schedules"

// ReportSchedule is the mutable scheduling definition. Executions are captured
// separately as immutable ReportScheduleRun snapshots.
type ReportSchedule struct {
	ID                string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID          string     `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	Name              string     `gorm:"column:name;not null" json:"name"`
	CronExpr          string     `gorm:"column:cron_expr;not null" json:"cron_expr"`
	Timezone          string     `gorm:"column:timezone;not null;default:UTC" json:"timezone"`
	Recipients        string     `gorm:"column:recipients;type:text;not null" json:"recipients"`
	DeviceIDs         []string   `gorm:"column:device_ids;type:jsonb;serializer:json" json:"device_ids"`
	Keys              []string   `gorm:"column:keys;type:jsonb;serializer:json" json:"keys"`
	LookbackHours     int        `gorm:"column:lookback_hours;not null;default:24" json:"lookback_hours"`
	Format            string     `gorm:"column:format;not null;default:csv" json:"format"`
	Enabled           bool       `gorm:"column:enabled;not null" json:"enabled"`
	NextRunAt         *time.Time `gorm:"column:next_run_at" json:"next_run_at"`
	Revision          int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	LastRunID         *string    `gorm:"column:last_run_id" json:"last_run_id"`
	LastRunAt         *time.Time `gorm:"column:last_run_at" json:"last_run_at"`
	LastStatus        string     `gorm:"column:last_status" json:"last_status"`
	ScheduleErrorCode *string    `gorm:"column:schedule_error_code" json:"schedule_error_code"`
	DeletedAt         *time.Time `gorm:"column:deleted_at;index" json:"-"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*ReportSchedule) TableName() string { return TableNameReportSchedule }

// Legacy request names remain temporarily so the pre-v83 service package still
// compiles. New code should use CreateReportScheduleRequest and
// UpdateReportScheduleRequest, whose update identity comes from the route.
type CreateReportScheduleReq struct {
	Name          string   `json:"name" validate:"required,max=128"`
	CronExpr      string   `json:"cron_expr" validate:"required"`
	Timezone      string   `json:"timezone" validate:"omitempty,max=64"`
	Recipients    string   `json:"recipients" validate:"required"`
	DeviceIDs     []string `json:"device_ids" validate:"required,min=1,dive,max=36"`
	Keys          []string `json:"keys" validate:"required,min=1,dive,max=255"`
	LookbackHours int      `json:"lookback_hours" validate:"omitempty,min=1,max=8760"`
	Format        string   `json:"format" validate:"omitempty,oneof=csv"`
	Enabled       bool     `json:"enabled"`
}

type UpdateReportScheduleReq struct {
	ID            string   `json:"-" validate:"-"`
	Revision      int64    `json:"revision" validate:"omitempty,min=1"`
	Name          string   `json:"name" validate:"omitempty,max=128"`
	CronExpr      string   `json:"cron_expr" validate:"omitempty"`
	Timezone      string   `json:"timezone" validate:"omitempty,max=64"`
	Recipients    string   `json:"recipients" validate:"omitempty"`
	DeviceIDs     []string `json:"device_ids" validate:"omitempty,min=1,dive,max=36"`
	Keys          []string `json:"keys" validate:"omitempty,min=1,dive,max=255"`
	LookbackHours int      `json:"lookback_hours" validate:"omitempty,min=1,max=8760"`
	Format        string   `json:"format" validate:"omitempty,oneof=csv"`
	Enabled       *bool    `json:"enabled"`
}
