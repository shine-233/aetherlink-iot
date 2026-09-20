package model

import "time"

const (
	TableNameReportScheduleRun      = "report_schedule_runs"
	TableNameReportScheduleDelivery = "report_schedule_deliveries"

	ReportRunTriggerScheduled = "scheduled"
	ReportRunTriggerManual    = "manual"
	ReportRunTriggerRetry     = "retry"

	ReportGenerationStatusPending    = "pending"
	ReportGenerationStatusProcessing = "processing"
	ReportGenerationStatusRetrying   = "retrying"
	ReportGenerationStatusSucceeded  = "succeeded"
	ReportGenerationStatusFailed     = "failed"

	ReportDeliveryStatusPending    = "pending"
	ReportDeliveryStatusProcessing = "processing"
	ReportDeliveryStatusRetrying   = "retrying"
	ReportDeliveryStatusAccepted   = "accepted"
	ReportDeliveryStatusFailed     = "failed"
	ReportDeliveryStatusAmbiguous  = "ambiguous"

	ReportProjectedStatusQueued    = "queued"
	ReportProjectedStatusRunning   = "running"
	ReportProjectedStatusSucceeded = "succeeded"
	ReportProjectedStatusFailed    = "failed"
	ReportProjectedStatusAmbiguous = "ambiguous"
)

type ReportRunConfigSnapshot struct {
	ScheduleName string   `json:"schedule_name"`
	Recipients   string   `json:"recipients"`
	DeviceIDs    []string `json:"device_ids"`
	Keys         []string `json:"keys"`
	Format       string   `json:"format"`
}

type ReportGenerationResult struct {
	PayloadDigest string `json:"payload_digest"`
	PayloadSize   int64  `json:"payload_size"`
	RowCount      int64  `json:"row_count"`
}

// ReportScheduleRun stores the canonical migration-83 generation lifecycle.
type ReportScheduleRun struct {
	ID                 string                  `gorm:"column:id;primaryKey" json:"-"`
	TenantID           string                  `gorm:"column:tenant_id;not null;index" json:"-"`
	ScheduleID         string                  `gorm:"column:schedule_id;index" json:"-"`
	Trigger            string                  `gorm:"column:trigger;not null" json:"-"`
	ScheduledSlot      *time.Time              `gorm:"column:scheduled_slot" json:"-"`
	RetryParentRunID   *string                 `gorm:"column:retry_parent_run_id" json:"-"`
	IdempotencyKeyHash *string                 `gorm:"column:idempotency_key_hash" json:"-"`
	RequestFingerprint *string                 `gorm:"column:request_fingerprint" json:"-"`
	WindowStartAt      time.Time               `gorm:"column:window_start_at;not null" json:"-"`
	WindowEndAt        time.Time               `gorm:"column:window_end_at;not null" json:"-"`
	ConfigSnapshot     ReportRunConfigSnapshot `gorm:"column:config_snapshot;type:jsonb;serializer:json;not null" json:"-"`
	MisfireCount       int                     `gorm:"column:misfire_count;not null;default:0" json:"-"`
	MisfireFirstSlot   *time.Time              `gorm:"column:misfire_first_slot" json:"-"`
	MisfireLastSlot    *time.Time              `gorm:"column:misfire_last_slot" json:"-"`
	GenerationStatus   string                  `gorm:"column:generation_status;not null" json:"-"`
	AttemptCount       int                     `gorm:"column:attempt_count;not null;default:0" json:"-"`
	MaxAttempts        int                     `gorm:"column:max_attempts;not null;default:3" json:"-"`
	NextAttemptAt      *time.Time              `gorm:"column:next_attempt_at" json:"-"`
	ClaimToken         *string                 `gorm:"column:claim_token" json:"-"`
	LeaseUntil         *time.Time              `gorm:"column:lease_until" json:"-"`
	Result             *ReportGenerationResult `gorm:"column:result;type:jsonb;serializer:json" json:"-"`
	ErrorCode          *string                 `gorm:"column:error_code" json:"-"`
	ErrorMessage       *string                 `gorm:"column:error_message" json:"-"`
	CreatedAt          time.Time               `gorm:"column:created_at;not null" json:"-"`
	StartedAt          *time.Time              `gorm:"column:started_at" json:"-"`
	GeneratedAt        *time.Time              `gorm:"column:generated_at" json:"-"`
	CompletedAt        *time.Time              `gorm:"column:completed_at" json:"-"`
	UpdatedAt          time.Time               `gorm:"column:updated_at;not null" json:"-"`
}

func (*ReportScheduleRun) TableName() string { return TableNameReportScheduleRun }

// ReportScheduleDelivery is the immutable SMTP envelope and durable payload outbox.
type ReportScheduleDelivery struct {
	RunID              string     `gorm:"column:run_id;primaryKey" json:"-"`
	TenantID           string     `gorm:"column:tenant_id;not null;index" json:"-"`
	EnvelopeFrom       string     `gorm:"column:envelope_from;not null" json:"-"`
	EnvelopeRecipients []string   `gorm:"column:envelope_recipients;type:jsonb;serializer:json;not null" json:"-"`
	MessageID          string     `gorm:"column:message_id;not null" json:"-"`
	Subject            string     `gorm:"column:subject;not null" json:"-"`
	Payload            []byte     `gorm:"column:payload" json:"-"`
	PayloadDigest      string     `gorm:"column:payload_digest;not null" json:"-"`
	PayloadSize        int64      `gorm:"column:payload_size;not null" json:"-"`
	RowCount           int64      `gorm:"column:row_count;not null" json:"-"`
	Status             string     `gorm:"column:status;not null" json:"-"`
	AttemptCount       int        `gorm:"column:attempt_count;not null;default:0" json:"-"`
	MaxAttempts        int        `gorm:"column:max_attempts;not null;default:3" json:"-"`
	NextAttemptAt      *time.Time `gorm:"column:next_attempt_at" json:"-"`
	ClaimToken         *string    `gorm:"column:claim_token" json:"-"`
	LeaseUntil         *time.Time `gorm:"column:lease_until" json:"-"`
	LastError          *string    `gorm:"column:last_error" json:"-"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null" json:"-"`
	StartedAt          *time.Time `gorm:"column:started_at" json:"-"`
	AcceptedAt         *time.Time `gorm:"column:accepted_at" json:"-"`
	FailedAt           *time.Time `gorm:"column:failed_at" json:"-"`
	AmbiguousAt        *time.Time `gorm:"column:ambiguous_at" json:"-"`
	CompletedAt        *time.Time `gorm:"column:completed_at" json:"-"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null" json:"-"`
}

func (*ReportScheduleDelivery) TableName() string { return TableNameReportScheduleDelivery }

func (r *ReportScheduleRun) ToResponse(delivery *ReportScheduleDelivery) *ReportRunResponse {
	if r == nil {
		return nil
	}
	deliveryStatus, deliveryAttempts := ReportDeliveryStatusPending, 0
	var deliveryCode string
	var deliveryCompleted *string
	duplicateRisk := false
	if delivery != nil {
		deliveryStatus = delivery.Status
		deliveryAttempts = delivery.AttemptCount
		if delivery.LastError != nil {
			deliveryCode = *delivery.LastError
		}
		if delivery.CompletedAt != nil {
			value := delivery.CompletedAt.UTC().Format(time.RFC3339Nano)
			deliveryCompleted = &value
		}
		duplicateRisk = delivery.Status == ReportDeliveryStatusAmbiguous
	}
	generationCode := ""
	if r.ErrorCode != nil {
		generationCode = *r.ErrorCode
	}
	var generationCompleted *string
	if r.CompletedAt != nil {
		value := r.CompletedAt.UTC().Format(time.RFC3339Nano)
		generationCompleted = &value
	}
	return &ReportRunResponse{
		RunID: r.ID, ScheduleID: r.ScheduleID, Trigger: r.Trigger,
		ParentRunID: r.RetryParentRunID, OverallStatus: reportOverallStatus(r.GenerationStatus, deliveryStatus),
		GenerationStatus: r.GenerationStatus, DeliveryStatus: deliveryStatus,
		WindowStartAt: r.WindowStartAt.UTC().Format(time.RFC3339Nano), WindowEndAt: r.WindowEndAt.UTC().Format(time.RFC3339Nano),
		GenerationAttempts: r.AttemptCount, DeliveryAttempts: deliveryAttempts,
		GenerationErrorCode: generationCode, DeliveryErrorCode: deliveryCode,
		DuplicateDeliveryRisk: duplicateRisk, CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano),
		GenerationCompletedAt: generationCompleted, DeliveryCompletedAt: deliveryCompleted,
	}
}

// ProjectReportStatus is the single source of truth for the run projection so the
// API action response and the run detail response can never disagree.
func ProjectReportStatus(generation, delivery string) string {
	return reportOverallStatus(generation, delivery)
}

func reportOverallStatus(generation, delivery string) string {
	if generation == ReportGenerationStatusFailed || delivery == ReportDeliveryStatusFailed {
		return ReportProjectedStatusFailed
	}
	if delivery == ReportDeliveryStatusAmbiguous {
		return ReportProjectedStatusAmbiguous
	}
	if generation == ReportGenerationStatusSucceeded && delivery == ReportDeliveryStatusAccepted {
		return ReportProjectedStatusSucceeded
	}
	if generation == ReportGenerationStatusProcessing || generation == ReportGenerationStatusRetrying ||
		delivery == ReportDeliveryStatusProcessing || delivery == ReportDeliveryStatusRetrying {
		return ReportProjectedStatusRunning
	}
	return ReportProjectedStatusQueued
}
