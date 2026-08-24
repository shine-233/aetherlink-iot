package model

import "time"

type FleetCommandJobReq struct {
	DeviceIDs        []string                     `json:"device_ids" validate:"omitempty"`
	DeviceFilter     *FleetCommandJobDeviceFilter `json:"device_filter" validate:"omitempty"`
	ScopeType        string                       `json:"scope_type"`
	ExpectedTotal    *int64                       `json:"expected_total,omitempty"`
	CurrentPageCount *int                         `json:"current_page_count,omitempty"`
	ScopeSource      *string                      `json:"scope_source,omitempty" validate:"omitempty,max=64"`
	MaxDevices       int                          `json:"max_devices,omitempty" validate:"omitempty,min=1,max=1000"`
	Identify         string                       `json:"identify" validate:"required"`
	Value            *string                      `json:"value"`
	TimeoutSeconds   int                          `json:"timeout_seconds"`
	ScheduledAt      *time.Time                   `json:"scheduled_at,omitempty"`
	PreviewToken     string                       `json:"preview_token,omitempty"`
	SubsetLimit      int                          `json:"subset_limit,omitempty" validate:"omitempty,min=1,max=50"`
	SampleLimit      int                          `json:"sample_limit,omitempty" validate:"omitempty,min=1,max=50"`
}

type FleetCommandJobDeviceFilter struct {
	DeviceNumber       *string `json:"device_number" validate:"omitempty,max=36"`
	IsEnabled          *string `json:"is_enabled" validate:"omitempty,max=36"`
	ProductID          *string `json:"product_id" validate:"omitempty,max=36"`
	Label              *string `json:"label" validate:"omitempty,max=255"`
	Name               *string `json:"name" validate:"omitempty,max=255"`
	CurrentVersion     *string `json:"current_version" validate:"omitempty,max=36"`
	PIDNumber          *string `json:"pid_number" validate:"omitempty,max=36"`
	FirmwareVersion    *string `json:"firmware_version" validate:"omitempty,max=64"`
	Description        *string `json:"description" validate:"omitempty,max=500"`
	SharedStatus       *string `json:"shared_status" validate:"omitempty,max=32"`
	GroupId            *string `json:"group_id" validate:"omitempty,max=36"`
	DeviceConfigId     *string `json:"device_config_id" validate:"omitempty,max=36"`
	DeviceTemplateID   *string `json:"device_template_id" validate:"omitempty,max=36"`
	IsOnline           *int    `json:"is_online" validate:"omitempty,max=36"`
	WarnStatus         *string `json:"warn_status" validate:"omitempty,max=36"`
	Search             *string `json:"search" validate:"omitempty,max=255"`
	AccessWay          *string `json:"access_way" validate:"omitempty,max=36"`
	BatchNumber        *string `json:"batch_number" validate:"omitempty"`
	DeviceType         *string `json:"device_type" validate:"omitempty,oneof=1 2 3"`
	ServiceIdentifier  *string `json:"service_identifier" validate:"omitempty,max=36"`
	ServiceAccessID    *string `json:"service_access_id" validate:"omitempty,max=36"`
	LastReportedAfter  *int64  `json:"last_reported_after" validate:"omitempty,gt=0"`
	LastReportedBefore *int64  `json:"last_reported_before" validate:"omitempty,gt=0"`
	NeverReported      *bool   `json:"never_reported"`
	LifecycleStatus    *string `json:"lifecycle_status" validate:"omitempty,oneof=activated inactive transmitted all"`
}

type FleetCommandJobPreviewRow struct {
	DeviceID              string     `json:"device_id"`
	DeviceNumber          string     `json:"device_number,omitempty"`
	Name                  string     `json:"name,omitempty"`
	Online                bool       `json:"online"`
	Eligible              bool       `json:"eligible"`
	Status                string     `json:"status"`
	RecommendedPath       string     `json:"recommended_path,omitempty"`
	Readiness             []string   `json:"readiness,omitempty"`
	TelemetryCurrentCount int        `json:"telemetry_current_count,omitempty"`
	LatestTelemetryKey    string     `json:"latest_telemetry_key,omitempty"`
	LatestTelemetryAt     *time.Time `json:"latest_telemetry_at,omitempty"`
	Advice                string     `json:"advice,omitempty"`
	Reason                string     `json:"reason,omitempty"`
}

type FleetCommandJobPreviewPathCounts struct {
	Immediate int `json:"immediate"`
	Jobs      int `json:"jobs"`
	Blocked   int `json:"blocked"`
	Telemetry int `json:"telemetry"`
}

type FleetCommandJobPreviewBlocker struct {
	Reason string `json:"reason"`
	Advice string `json:"advice,omitempty"`
	Count  int    `json:"count"`
}

type FleetCommandJobPreviewResult struct {
	JobType        string                            `json:"job_type"`
	ScopeType      string                            `json:"scope_type"`
	PreviewToken   string                            `json:"preview_token"`
	TotalMatched   int64                             `json:"total_matched,omitempty"`
	RequestedCount int                               `json:"requested_count"`
	EligibleCount  int                               `json:"eligible_count"`
	BlockedCount   int                               `json:"blocked_count"`
	TimeoutSeconds int                               `json:"timeout_seconds"`
	Rows           []FleetCommandJobPreviewRow       `json:"rows"`
	PreviewDevices []GetDeviceListByPageRsp          `json:"preview_devices,omitempty"`
	SampleDevices  []GetDeviceListByPageRsp          `json:"sample_devices,omitempty"`
	Warnings       []string                          `json:"warnings,omitempty"`
	ScopeLimits    []string                          `json:"scope_limits,omitempty"`
	PathCounts     FleetCommandJobPreviewPathCounts  `json:"path_counts"`
	Blockers       []FleetCommandJobPreviewBlocker   `json:"blockers,omitempty"`
	NextAction     string                            `json:"next_action,omitempty"`
	Governance     *FleetCommandJobGovernanceSummary `json:"governance_summary,omitempty"`
}

type FleetCommandJobSubmitRow struct {
	DetailID              string     `json:"detail_id,omitempty"`
	DeviceID              string     `json:"device_id"`
	DeviceNumber          string     `json:"device_number,omitempty"`
	Name                  string     `json:"name,omitempty"`
	Eligible              bool       `json:"eligible"`
	Status                string     `json:"status"`
	Readiness             []string   `json:"readiness,omitempty"`
	MessageID             string     `json:"message_id,omitempty"`
	DispatchAttempts      int        `json:"dispatch_attempts,omitempty"`
	MaxDispatchAttempts   int        `json:"max_dispatch_attempts,omitempty"`
	RetryState            string     `json:"retry_state,omitempty"`
	LastDispatchStartedAt *time.Time `json:"last_dispatch_started_at,omitempty"`
	NextRetryAfter        *time.Time `json:"next_retry_after,omitempty"`
	ResponseRecorded      bool       `json:"response_recorded,omitempty"`
	ResponseStatus        string     `json:"response_status,omitempty"`
	ResponseStatusLabel   string     `json:"response_status_label,omitempty"`
	ResponseData          string     `json:"response_data,omitempty"`
	ResponseError         string     `json:"response_error,omitempty"`
	CommandLogCreatedAt   *time.Time `json:"command_log_created_at,omitempty"`
	LogRecorded           bool       `json:"log_recorded,omitempty"`
	Reason                string     `json:"reason,omitempty"`
	Advice                string     `json:"advice,omitempty"`
	CanRetry              bool       `json:"can_retry"`
	RecommendedPath       string     `json:"recommended_path,omitempty"`
	TelemetryCurrentCount int        `json:"telemetry_current_count,omitempty"`
	LatestTelemetryKey    string     `json:"latest_telemetry_key,omitempty"`
	LatestTelemetryAt     *time.Time `json:"latest_telemetry_at,omitempty"`
	SubmittedAt           *time.Time `json:"submitted_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
}

type FleetCommandJobSubmitResult struct {
	JobID               string                            `json:"job_id"`
	JobType             string                            `json:"job_type"`
	ScopeType           string                            `json:"scope_type"`
	Identify            string                            `json:"identify"`
	PreviewToken        string                            `json:"preview_token,omitempty"`
	Status              string                            `json:"status"`
	AuditRemark         *string                           `json:"audit_remark,omitempty"`
	RequestedCount      int                               `json:"requested_count"`
	EligibleCount       int                               `json:"eligible_count"`
	BlockedCount        int                               `json:"blocked_count"`
	SubmittedCount      int                               `json:"submitted_count"`
	FailedCount         int                               `json:"failed_count"`
	RetryableCount      int                               `json:"retryable_count"`
	RetryReadyCount     int                               `json:"retry_ready_count"`
	RetryWaitingCount   int                               `json:"retry_waiting_count"`
	RetryExhaustedCount int                               `json:"retry_exhausted_count"`
	LogMissingCount     int                               `json:"log_missing_count"`
	TimeoutSeconds      int                               `json:"timeout_seconds"`
	CanCancel           bool                              `json:"can_cancel"`
	CanRetryFailed      bool                              `json:"can_retry_failed"`
	CreatedAt           *time.Time                        `json:"created_at,omitempty"`
	UpdatedAt           *time.Time                        `json:"updated_at,omitempty"`
	ScheduledAt         *time.Time                        `json:"scheduled_at,omitempty"`
	NextDispatchAt      *time.Time                        `json:"next_dispatch_at,omitempty"`
	TimeoutAt           *time.Time                        `json:"timeout_at,omitempty"`
	Rows                []FleetCommandJobSubmitRow        `json:"rows"`
	RowsTotal           int                               `json:"rows_total,omitempty"`
	RowsTruncated       bool                              `json:"rows_truncated,omitempty"`
	Events              []FleetCommandJobEvent            `json:"events,omitempty"`
	StatusCounts        map[string]int                    `json:"status_counts,omitempty"`
	ProgressHealth      *FleetCommandJobProgressHealth    `json:"progress_health,omitempty"`
	HandoffSummary      string                            `json:"handoff_summary,omitempty"`
	AuditSummary        *FleetCommandJobAuditSummary      `json:"audit_summary,omitempty"`
	ExecutionSummary    *FleetCommandJobExecutionSummary  `json:"execution_summary,omitempty"`
	Governance          *FleetCommandJobGovernanceSummary `json:"governance_summary,omitempty"`
	Warnings            []string                          `json:"warnings,omitempty"`
	ScopeLimits         []string                          `json:"scope_limits,omitempty"`
}

type FleetCommandJobProgressHealth struct {
	State                   string `json:"state"`
	PendingCount            int    `json:"pending_count"`
	TerminalCount           int    `json:"terminal_count"`
	ElapsedSeconds          int64  `json:"elapsed_seconds"`
	TimeoutRemainingSeconds int64  `json:"timeout_remaining_seconds"`
	NextAction              string `json:"next_action"`
}

type FleetCommandJobAuditSummary struct {
	EventCount      int        `json:"event_count"`
	LatestEventType string     `json:"latest_event_type,omitempty"`
	LatestEventAt   *time.Time `json:"latest_event_at,omitempty"`
	LatestMessage   string     `json:"latest_message,omitempty"`
	NextAction      string     `json:"next_action"`
}

type FleetCommandJobExecutionSummary struct {
	PathType      string                                  `json:"path_type"`
	PathLabel     string                                  `json:"path_label"`
	Decision      string                                  `json:"decision"`
	CanClose      bool                                    `json:"can_close"`
	CloseBlockers []string                                `json:"close_blockers,omitempty"`
	NextAction    string                                  `json:"next_action"`
	Evidence      []string                                `json:"evidence,omitempty"`
	Checklist     []FleetCommandJobExecutionChecklistItem `json:"checklist,omitempty"`
}

type FleetCommandJobExecutionChecklistItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

type FleetCommandJobGovernanceSummary struct {
	Level      string                          `json:"level"`
	Title      string                          `json:"title"`
	Summary    string                          `json:"summary"`
	NextAction string                          `json:"next_action"`
	Items      []FleetCommandJobGovernanceItem `json:"items,omitempty"`
}

type FleetCommandJobGovernanceItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

type FleetCommandJobRowsReq struct {
	Page         int    `json:"page" form:"page"`
	PageSize     int    `json:"page_size" form:"page_size"`
	StatusFilter string `json:"status_filter" form:"status_filter"`
	Search       string `json:"search" form:"search"`
}

type FleetCommandJobRowsResult struct {
	Total         int64                      `json:"total"`
	Page          int                        `json:"page"`
	PageSize      int                        `json:"page_size"`
	StatusFilter  string                     `json:"status_filter,omitempty"`
	Search        string                     `json:"search,omitempty"`
	Rows          []FleetCommandJobSubmitRow `json:"rows"`
	RowsTruncated bool                       `json:"rows_truncated"`
}

type FleetCommandJobEvent struct {
	ID        string     `json:"id"`
	EventType string     `json:"event_type"`
	DetailID  string     `json:"detail_id,omitempty"`
	DeviceID  string     `json:"device_id,omitempty"`
	Message   string     `json:"message,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

type FleetCommandJobSupportDevice struct {
	DetailID            string                            `json:"detail_id,omitempty"`
	DeviceID            string                            `json:"device_id"`
	DeviceNumber        string                            `json:"device_number,omitempty"`
	Name                string                            `json:"name,omitempty"`
	Status              string                            `json:"status"`
	Readiness           []string                          `json:"readiness,omitempty"`
	MessageID           string                            `json:"message_id,omitempty"`
	DispatchAttempts    int                               `json:"dispatch_attempts,omitempty"`
	MaxDispatchAttempts int                               `json:"max_dispatch_attempts,omitempty"`
	RetryState          string                            `json:"retry_state,omitempty"`
	NextRetryAfter      *time.Time                        `json:"next_retry_after,omitempty"`
	ResponseStatus      string                            `json:"response_status,omitempty"`
	ResponseStatusLabel string                            `json:"response_status_label,omitempty"`
	ResponseData        string                            `json:"response_data,omitempty"`
	ResponseError       string                            `json:"response_error,omitempty"`
	ResponseAt          *time.Time                        `json:"response_at,omitempty"`
	Reason              string                            `json:"reason,omitempty"`
	Advice              string                            `json:"advice,omitempty"`
	ReadyCheckURL       string                            `json:"ready_check_url,omitempty"`
	JobDetailURL        string                            `json:"job_detail_url,omitempty"`
	DiagnosticSummary   *FleetCommandJobSupportDiagnostic `json:"diagnostic_summary,omitempty"`
}

type FleetCommandJobSupportDiagnostic struct {
	Level       string   `json:"level"`
	Code        string   `json:"code"`
	Summary     string   `json:"summary"`
	Evidence    []string `json:"evidence,omitempty"`
	NextActions []string `json:"next_actions,omitempty"`
}

type FleetCommandJobSupportBundle struct {
	JobID               string                         `json:"job_id"`
	JobType             string                         `json:"job_type"`
	ScopeType           string                         `json:"scope_type"`
	Identify            string                         `json:"identify"`
	Status              string                         `json:"status"`
	ScheduledAt         *time.Time                     `json:"scheduled_at,omitempty"`
	NextDispatchAt      *time.Time                     `json:"next_dispatch_at,omitempty"`
	AuditRemark         *string                        `json:"audit_remark,omitempty"`
	RequestedCount      int                            `json:"requested_count"`
	EligibleCount       int                            `json:"eligible_count"`
	BlockedCount        int                            `json:"blocked_count"`
	SubmittedCount      int                            `json:"submitted_count"`
	FailedCount         int                            `json:"failed_count"`
	RetryableCount      int                            `json:"retryable_count"`
	RetryReadyCount     int                            `json:"retry_ready_count"`
	RetryWaitingCount   int                            `json:"retry_waiting_count"`
	RetryExhaustedCount int                            `json:"retry_exhausted_count"`
	LogMissingCount     int                            `json:"log_missing_count"`
	StatusCounts        map[string]int                 `json:"status_counts,omitempty"`
	RetryableDeviceIDs  []string                       `json:"retryable_device_ids,omitempty"`
	MissingLogDeviceIDs []string                       `json:"missing_log_device_ids,omitempty"`
	FailedDevices       []FleetCommandJobSupportDevice `json:"failed_devices,omitempty"`
	// RowsTruncated 表示证据行触达单次内联上限，支持包可能缺少部分设备行。
	RowsTruncated    bool                              `json:"rows_truncated,omitempty"`
	Events           []FleetCommandJobEvent            `json:"events,omitempty"`
	ExecutionSummary *FleetCommandJobExecutionSummary  `json:"execution_summary,omitempty"`
	Governance       *FleetCommandJobGovernanceSummary `json:"governance_summary,omitempty"`
	NextActions      []string                          `json:"next_actions"`
	GeneratedAt      time.Time                         `json:"generated_at"`
	ShareHint        string                            `json:"share_hint"`
}

type FleetCommandJobListReq struct {
	Page            int    `json:"page" form:"page"`
	PageSize        int    `json:"page_size" form:"page_size"`
	Status          string `json:"status" form:"status"`
	AttentionFilter string `json:"attention_filter" form:"attention_filter"`
	Search          string `json:"search" form:"search"`
}

type FleetCommandJobListItem struct {
	JobID                    string     `json:"job_id"`
	JobType                  string     `json:"job_type"`
	ScopeType                string     `json:"scope_type"`
	Identify                 string     `json:"identify"`
	CommandValue             *string    `json:"command_value,omitempty"`
	TimeoutSeconds           int        `json:"timeout_seconds"`
	Status                   string     `json:"status"`
	AuditRemark              *string    `json:"audit_remark,omitempty"`
	RequestedCount           int        `json:"requested_count"`
	EligibleCount            int        `json:"eligible_count"`
	BlockedCount             int        `json:"blocked_count"`
	SubmittedCount           int        `json:"submitted_count"`
	FailedCount              int        `json:"failed_count"`
	RetryableCount           int        `json:"retryable_count"`
	RetryReadyCount          int        `json:"retry_ready_count"`
	RetryWaitingCount        int        `json:"retry_waiting_count"`
	RetryExhaustedCount      int        `json:"retry_exhausted_count"`
	LogMissingCount          int        `json:"log_missing_count"`
	DeviceAckFailedCount     int        `json:"device_ack_failed_count"`
	NeedsOperatorAction      bool       `json:"needs_operator_action"`
	NeedsOperatorActionCount int        `json:"needs_operator_action_count"`
	CanCancel                bool       `json:"can_cancel"`
	CanRetryFailed           bool       `json:"can_retry_failed"`
	CreatedAt                *time.Time `json:"created_at,omitempty"`
	UpdatedAt                *time.Time `json:"updated_at,omitempty"`
	ScheduledAt              *time.Time `json:"scheduled_at,omitempty"`
	NextDispatchAt           *time.Time `json:"next_dispatch_at,omitempty"`
	TimeoutAt                *time.Time `json:"timeout_at,omitempty"`
}

type FleetCommandJobListAttentionCounts struct {
	RetryableCount           int `json:"retryable_count"`
	RetryReadyCount          int `json:"retry_ready_count"`
	RetryWaitingCount        int `json:"retry_waiting_count"`
	RetryExhaustedCount      int `json:"retry_exhausted_count"`
	LogMissingCount          int `json:"log_missing_count"`
	DeviceAckFailedCount     int `json:"device_ack_failed_count"`
	BlockedCount             int `json:"blocked_count"`
	NeedsOperatorActionCount int `json:"needs_operator_action_count"`
}

type FleetCommandJobListResult struct {
	Total           int64                              `json:"total"`
	Page            int                                `json:"page"`
	PageSize        int                                `json:"page_size"`
	Search          string                             `json:"search,omitempty"`
	AttentionFilter string                             `json:"attention_filter,omitempty"`
	AttentionCounts FleetCommandJobListAttentionCounts `json:"attention_counts"`
	List            []FleetCommandJobListItem          `json:"list"`
}
