package model

// CreateReportScheduleRequest is the public create body. Enabled is a pointer
// so an omitted value can default to true without conflating it with false.
type CreateReportScheduleRequest struct {
	Name          string   `json:"name" validate:"required,max=128"`
	CronExpr      string   `json:"cron_expr" validate:"required,max=128"`
	Timezone      string   `json:"timezone" validate:"required,max=64"`
	Recipients    string   `json:"recipients" validate:"required"`
	DeviceIDs     []string `json:"device_ids" validate:"required,min=1,dive,max=36"`
	Keys          []string `json:"keys" validate:"required,min=1,dive,max=255"`
	LookbackHours int      `json:"lookback_hours" validate:"omitempty,min=1,max=8760"`
	Format        string   `json:"format" validate:"omitempty,oneof=csv"`
	Enabled       *bool    `json:"enabled"`
}

func (r CreateReportScheduleRequest) EnabledOrDefault() bool {
	return r.Enabled == nil || *r.Enabled
}

// UpdateReportScheduleRequest deliberately has no ID. The upper layer supplies
// route identity separately and revision provides optimistic concurrency.
type UpdateReportScheduleRequest struct {
	Revision      int64     `json:"revision" validate:"required,min=1"`
	Name          *string   `json:"name" validate:"omitempty,max=128"`
	CronExpr      *string   `json:"cron_expr" validate:"omitempty,max=128"`
	Timezone      *string   `json:"timezone" validate:"omitempty,max=64"`
	Recipients    *string   `json:"recipients"`
	DeviceIDs     *[]string `json:"device_ids" validate:"omitempty,min=1,dive,max=36"`
	Keys          *[]string `json:"keys" validate:"omitempty,min=1,dive,max=255"`
	LookbackHours *int      `json:"lookback_hours" validate:"omitempty,min=1,max=8760"`
	Format        *string   `json:"format" validate:"omitempty,oneof=csv"`
	Enabled       *bool     `json:"enabled"`
}

type ReportScheduleListRequest struct {
	Page     int    `form:"page" json:"page" validate:"omitempty,min=1"`
	PageSize int    `form:"page_size" json:"page_size" validate:"omitempty,min=1,max=100"`
	Search   string `form:"search" json:"search" validate:"omitempty,max=128"`
	Enabled  *bool  `form:"enabled" json:"enabled"`
}

type ReportScheduleListResponse struct {
	List     []*ReportSchedule `json:"list"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type ReportRunListRequest struct {
	Page     int `form:"page" json:"page" validate:"omitempty,min=1"`
	PageSize int `form:"page_size" json:"page_size" validate:"omitempty,min=1,max=100"`
}

type ReportRunListResponse struct {
	List     []*ReportRunResponse `json:"list"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

type ReportRunResponse struct {
	RunID                 string  `json:"run_id"`
	ScheduleID            string  `json:"schedule_id"`
	Trigger               string  `json:"trigger"`
	ParentRunID           *string `json:"parent_run_id,omitempty"`
	OverallStatus         string  `json:"overall_status"`
	GenerationStatus      string  `json:"generation_status"`
	DeliveryStatus        string  `json:"delivery_status"`
	WindowStartAt         string  `json:"window_start_at"`
	WindowEndAt           string  `json:"window_end_at"`
	GenerationAttempts    int     `json:"generation_attempts"`
	DeliveryAttempts      int     `json:"delivery_attempts"`
	GenerationErrorCode   string  `json:"generation_error_code,omitempty"`
	DeliveryErrorCode     string  `json:"delivery_error_code,omitempty"`
	DuplicateDeliveryRisk bool    `json:"duplicate_delivery_risk"`
	CreatedAt             string  `json:"created_at"`
	GenerationCompletedAt *string `json:"generation_completed_at,omitempty"`
	DeliveryCompletedAt   *string `json:"delivery_completed_at,omitempty"`
}

type ReportRunActionResponse struct {
	RunID                 string `json:"run_id"`
	ScheduleID            string `json:"schedule_id"`
	Status                string `json:"status"`
	StatusURL             string `json:"status_url"`
	IdempotentReplay      bool   `json:"idempotent_replay"`
	DuplicateDeliveryRisk bool   `json:"duplicate_delivery_risk"`
}
