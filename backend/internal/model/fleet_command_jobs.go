package model

import "time"

const (
	TableNameCommandJob              = "command_jobs"
	TableNameCommandJobDetail        = "command_job_details"
	TableNameCommandJobEvent         = "command_job_events"
	TableNameCommandJobDispatchQuota = "command_job_dispatch_quotas"
)

type CommandJob struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID        string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	OperatorID      string     `gorm:"column:operator_id;not null" json:"operator_id"`
	JobType         string     `gorm:"column:job_type;not null" json:"job_type"`
	ScopeType       string     `gorm:"column:scope_type;not null" json:"scope_type"`
	Identify        string     `gorm:"column:identify;not null" json:"identify"`
	CommandValue    *string    `gorm:"column:command_value" json:"command_value"`
	TimeoutSeconds  int        `gorm:"column:timeout_seconds;not null" json:"timeout_seconds"`
	Status          string     `gorm:"column:status;not null" json:"status"`
	RequestedCount  int        `gorm:"column:requested_count;not null" json:"requested_count"`
	EligibleCount   int        `gorm:"column:eligible_count;not null" json:"eligible_count"`
	BlockedCount    int        `gorm:"column:blocked_count;not null" json:"blocked_count"`
	SubmittedCount  int        `gorm:"column:submitted_count;not null" json:"submitted_count"`
	FailedCount     int        `gorm:"column:failed_count;not null" json:"failed_count"`
	CanCancel       bool       `gorm:"column:can_cancel;not null" json:"can_cancel"`
	CanRetryFailed  bool       `gorm:"column:can_retry_failed;not null" json:"can_retry_failed"`
	ScopeSnapshot   *string    `gorm:"column:scope_snapshot;type:jsonb" json:"scope_snapshot"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
	ScheduledAt     *time.Time `gorm:"column:scheduled_at" json:"scheduled_at"`
	NextDispatchAt  *time.Time `gorm:"column:next_dispatch_at" json:"next_dispatch_at"`
	TimeoutAt       *time.Time `gorm:"column:timeout_at" json:"timeout_at"`
	LastSubmittedAt *time.Time `gorm:"column:last_submitted_at" json:"last_submitted_at"`
	Remark          *string    `gorm:"column:remark" json:"remark"`
}

func (*CommandJob) TableName() string {
	return TableNameCommandJob
}

type CommandJobDetail struct {
	ID                    string     `gorm:"column:id;primaryKey" json:"id"`
	CommandJobID          string     `gorm:"column:command_job_id;not null" json:"command_job_id"`
	TenantID              string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	DeviceID              string     `gorm:"column:device_id;not null" json:"device_id"`
	DeviceNumber          string     `gorm:"column:device_number" json:"device_number"`
	Name                  string     `gorm:"column:name" json:"name"`
	Online                bool       `gorm:"column:online;not null" json:"online"`
	Eligible              bool       `gorm:"column:eligible;not null" json:"eligible"`
	Status                string     `gorm:"column:status;not null" json:"status"`
	RecommendedPath       string     `gorm:"column:recommended_path" json:"recommended_path"`
	MessageID             *string    `gorm:"column:message_id" json:"message_id"`
	ResponseStatus        *string    `gorm:"column:response_status" json:"response_status"`
	ResponsePayload       *string    `gorm:"column:response_payload" json:"response_payload"`
	ResponseError         *string    `gorm:"column:response_error" json:"response_error"`
	ResponseAt            *time.Time `gorm:"column:response_at" json:"response_at"`
	DispatchAttempts      int        `gorm:"column:dispatch_attempts;not null" json:"dispatch_attempts"`
	DispatchLeaseToken    *string    `gorm:"column:dispatch_lease_token" json:"dispatch_lease_token"`
	DispatchLeaseUntil    *time.Time `gorm:"column:dispatch_lease_until" json:"dispatch_lease_until"`
	LastDispatchStartedAt *time.Time `gorm:"column:last_dispatch_started_at" json:"last_dispatch_started_at"`
	NextRetryAfter        *time.Time `gorm:"column:next_retry_after" json:"next_retry_after"`
	LogRecorded           bool       `gorm:"column:log_recorded;not null" json:"log_recorded"`
	Reason                *string    `gorm:"column:reason" json:"reason"`
	Advice                *string    `gorm:"column:advice" json:"advice"`
	CanRetry              bool       `gorm:"column:can_retry;not null" json:"can_retry"`
	TelemetryCurrentCount int        `gorm:"column:telemetry_current_count;not null" json:"telemetry_current_count"`
	LatestTelemetryKey    string     `gorm:"column:latest_telemetry_key" json:"latest_telemetry_key"`
	LatestTelemetryAt     *time.Time `gorm:"column:latest_telemetry_at" json:"latest_telemetry_at"`
	Readiness             *string    `gorm:"column:readiness;type:jsonb" json:"readiness"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
	SubmittedAt           *time.Time `gorm:"column:submitted_at" json:"submitted_at"`
	CompletedAt           *time.Time `gorm:"column:completed_at" json:"completed_at"`
}

func (*CommandJobDetail) TableName() string {
	return TableNameCommandJobDetail
}

type CommandJobEvent struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	CommandJobID string    `gorm:"column:command_job_id;not null" json:"command_job_id"`
	TenantID     string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	DetailID     *string   `gorm:"column:detail_id" json:"detail_id"`
	DeviceID     *string   `gorm:"column:device_id" json:"device_id"`
	EventType    string    `gorm:"column:event_type;not null" json:"event_type"`
	Message      string    `gorm:"column:message" json:"message"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*CommandJobEvent) TableName() string {
	return TableNameCommandJobEvent
}

type CommandJobDispatchQuota struct {
	ScopeType      string    `gorm:"column:scope_type;primaryKey" json:"scope_type"`
	ScopeID        string    `gorm:"column:scope_id;primaryKey" json:"scope_id"`
	NextDispatchAt time.Time `gorm:"column:next_dispatch_at;not null" json:"next_dispatch_at"`
	MaxConcurrent  int       `gorm:"column:max_concurrent;not null" json:"max_concurrent"`
	RatePerSecond  float64   `gorm:"column:rate_per_second;not null" json:"rate_per_second"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*CommandJobDispatchQuota) TableName() string {
	return TableNameCommandJobDispatchQuota
}
