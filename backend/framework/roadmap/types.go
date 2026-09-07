package roadmap

import "time"

// EntityID identifies any tenant-scoped platform entity.
type EntityID string

type EntityType string

const (
	EntityDevice EntityType = "device"
	EntityAsset  EntityType = "asset"
	EntityUser   EntityType = "user"
	EntityTenant EntityType = "tenant"
)

// Relation is a typed, directed edge between platform entities. This is more
// expressive than a parent/child asset tree and supports gateway, ownership,
// and arbitrary domain relationships.
type Relation struct {
	TenantID     EntityID       `json:"tenant_id"`
	FromType     EntityType     `json:"from_type"`
	FromID       EntityID       `json:"from_id"`
	RelationType string         `json:"relation_type"`
	ToType       EntityType     `json:"to_type"`
	ToID         EntityID       `json:"to_id"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type RelationQuery struct {
	TenantID     EntityID
	FromType     EntityType
	FromID       EntityID
	RelationType string
	ToType       EntityType
	ToID         EntityID
}

type RetryPolicy struct {
	MaxAttempts    uint          `json:"max_attempts"`
	InitialBackoff time.Duration `json:"initial_backoff"`
	MaxBackoff     time.Duration `json:"max_backoff"`
	Multiplier     float64       `json:"multiplier"`
}

type ExecutionTrace struct {
	TraceID    string         `json:"trace_id"`
	RuleID     EntityID       `json:"rule_id"`
	MessageID  string         `json:"message_id"`
	NodeID     string         `json:"node_id"`
	Attempt    uint           `json:"attempt"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type DeadLetterMessage struct {
	MessageID string    `json:"message_id"`
	RuleID    EntityID  `json:"rule_id"`
	Payload   []byte    `json:"payload"`
	Reason    string    `json:"reason"`
	Attempts  uint      `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
}

type OTAStatus string

const (
	OTAStatusPending    OTAStatus = "pending"
	OTAStatusRunning    OTAStatus = "running"
	OTAStatusSuccess    OTAStatus = "success"
	OTAStatusFailed     OTAStatus = "failed"
	OTAStatusTimeout    OTAStatus = "timeout"
	OTAStatusCanceled   OTAStatus = "canceled"
	OTAStatusRolledBack OTAStatus = "rolled_back"
)

type OTAProgress struct {
	TaskID     EntityID  `json:"task_id"`
	DeviceID   EntityID  `json:"device_id"`
	Status     OTAStatus `json:"status"`
	Percent    float32   `json:"percent"`
	Version    string    `json:"version"`
	Error      string    `json:"error,omitempty"`
	ReportedAt time.Time `json:"reported_at"`
}

type EdgeNode struct {
	ID             EntityID  `json:"id"`
	TenantID       EntityID  `json:"tenant_id"`
	Version        string    `json:"version"`
	Status         string    `json:"status"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	ConfigRevision string    `json:"config_revision"`
}

type Notification struct {
	ID       string         `json:"id"`
	TenantID EntityID       `json:"tenant_id"`
	Channel  string         `json:"channel"`
	To       []string       `json:"to"`
	Subject  string         `json:"subject,omitempty"`
	Body     string         `json:"body"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type NotificationResult struct {
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	Accepted          bool   `json:"accepted"`
}

type AdapterConfig struct {
	DeviceID EntityID       `json:"device_id"`
	Values   map[string]any `json:"values,omitempty"`
}

type TelemetryPoint struct {
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

type AnalyticsQuery struct {
	TenantID   EntityID
	EntityIDs  []EntityID
	MetricKeys []string
	From       time.Time
	To         time.Time
	Interval   time.Duration
}

type AnalyticsRow struct {
	EntityID  EntityID       `json:"entity_id"`
	Timestamp time.Time      `json:"timestamp"`
	Values    map[string]any `json:"values"`
}

type ReportSpec struct {
	Name       string
	Query      AnalyticsQuery
	Format     string
	Recipients []string
}
