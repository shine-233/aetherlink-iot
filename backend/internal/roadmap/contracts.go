package roadmap

import "context"

// Relations provides CRUD/query semantics for generic entity relations.
type Relations interface {
	Create(ctx context.Context, relation Relation) error
	Delete(ctx context.Context, query RelationQuery) error
	List(ctx context.Context, query RelationQuery) ([]Relation, error)
}

// RuleRuntime separates retry/dead-letter/trace policy from the current rule
// chain implementation. Implementations should preserve message idempotency.
type RuleRuntime interface {
	Execute(ctx context.Context, ruleID EntityID, message []byte, policy RetryPolicy) error
	Replay(ctx context.Context, messageID string) error
}

type TraceSink interface {
	Record(ctx context.Context, trace ExecutionTrace) error
}
type DeadLetterStore interface {
	Put(ctx context.Context, message DeadLetterMessage) error
	List(ctx context.Context, ruleID EntityID, limit int) ([]DeadLetterMessage, error)
	Remove(ctx context.Context, messageID string) error
}

type OTAService interface {
	RecordProgress(ctx context.Context, progress OTAProgress) error
	Retry(ctx context.Context, taskID, deviceID EntityID) error
	Rollback(ctx context.Context, taskID, deviceID EntityID) error
}

type EdgeOperations interface {
	Register(ctx context.Context, node EdgeNode) error
	Heartbeat(ctx context.Context, nodeID EntityID, seenAt EdgeNode) error
	PublishConfig(ctx context.Context, nodeID EntityID, revision string, config []byte) error
	Reconcile(ctx context.Context, nodeID EntityID) error
}

// NotificationProvider is the stable adapter contract for email/SMS/webhook/
// enterprise channels. Provider implementations should expose retry-safe sends.
type NotificationProvider interface {
	Name() string
	Send(ctx context.Context, notification Notification) (NotificationResult, error)
	Health(ctx context.Context) error
}

type ProtocolAdapter interface {
	Name() string
	Validate(ctx context.Context, config AdapterConfig) error
	Connect(ctx context.Context, config AdapterConfig) error
	Discover(ctx context.Context) ([]EntityID, error)
	ReadTelemetry(ctx context.Context, deviceID EntityID, keys []string) ([]TelemetryPoint, error)
	WriteCommand(ctx context.Context, deviceID EntityID, command string, payload []byte) error
	Health(ctx context.Context) error
	Close(ctx context.Context) error
}

type AnalyticsService interface {
	Query(ctx context.Context, query AnalyticsQuery) ([]AnalyticsRow, error)
	Export(ctx context.Context, query AnalyticsQuery, format string) ([]byte, error)
	RenderReport(ctx context.Context, spec ReportSpec) ([]byte, error)
}
