package roadmap

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by the deliberately inert reference services.
// Replace these constructors with production adapters only after contracts and
// persistence/idempotency behavior have been reviewed.
var ErrNotImplemented = errors.New("roadmap framework: not implemented")

type UnwiredRelations struct{}

func (UnwiredRelations) Create(context.Context, Relation) error      { return ErrNotImplemented }
func (UnwiredRelations) Delete(context.Context, RelationQuery) error { return ErrNotImplemented }
func (UnwiredRelations) List(context.Context, RelationQuery) ([]Relation, error) {
	return nil, ErrNotImplemented
}

type UnwiredRuleRuntime struct{}

func (UnwiredRuleRuntime) Execute(context.Context, EntityID, []byte, RetryPolicy) error {
	return ErrNotImplemented
}
func (UnwiredRuleRuntime) Replay(context.Context, string) error { return ErrNotImplemented }

type UnwiredTraceSink struct{}

func (UnwiredTraceSink) Record(context.Context, ExecutionTrace) error { return ErrNotImplemented }

type UnwiredDeadLetterStore struct{}

func (UnwiredDeadLetterStore) Put(context.Context, DeadLetterMessage) error { return ErrNotImplemented }
func (UnwiredDeadLetterStore) List(context.Context, EntityID, int) ([]DeadLetterMessage, error) {
	return nil, ErrNotImplemented
}
func (UnwiredDeadLetterStore) Remove(context.Context, string) error { return ErrNotImplemented }

type UnwiredOTAService struct{}

func (UnwiredOTAService) RecordProgress(context.Context, OTAProgress) error { return ErrNotImplemented }
func (UnwiredOTAService) Retry(context.Context, EntityID, EntityID) error   { return ErrNotImplemented }
func (UnwiredOTAService) Rollback(context.Context, EntityID, EntityID) error {
	return ErrNotImplemented
}

type UnwiredEdgeOperations struct{}

func (UnwiredEdgeOperations) Register(context.Context, EdgeNode) error { return ErrNotImplemented }
func (UnwiredEdgeOperations) Heartbeat(context.Context, EntityID, EdgeNode) error {
	return ErrNotImplemented
}
func (UnwiredEdgeOperations) PublishConfig(context.Context, EntityID, string, []byte) error {
	return ErrNotImplemented
}
func (UnwiredEdgeOperations) Reconcile(context.Context, EntityID) error { return ErrNotImplemented }

type UnwiredAnalytics struct{}

func (UnwiredAnalytics) Query(context.Context, AnalyticsQuery) ([]AnalyticsRow, error) {
	return nil, ErrNotImplemented
}
func (UnwiredAnalytics) Export(context.Context, AnalyticsQuery, string) ([]byte, error) {
	return nil, ErrNotImplemented
}
func (UnwiredAnalytics) RenderReport(context.Context, ReportSpec) ([]byte, error) {
	return nil, ErrNotImplemented
}

type UnwiredNotificationProvider struct{}

func (UnwiredNotificationProvider) Name() string { return "unwired" }
func (UnwiredNotificationProvider) Send(context.Context, Notification) (NotificationResult, error) {
	return NotificationResult{}, ErrNotImplemented
}
func (UnwiredNotificationProvider) Health(context.Context) error { return ErrNotImplemented }

type UnwiredProtocolAdapter struct{}

func (UnwiredProtocolAdapter) Name() string { return "unwired" }
func (UnwiredProtocolAdapter) Validate(context.Context, AdapterConfig) error {
	return ErrNotImplemented
}
func (UnwiredProtocolAdapter) Connect(context.Context, AdapterConfig) error { return ErrNotImplemented }
func (UnwiredProtocolAdapter) Discover(context.Context) ([]EntityID, error) {
	return nil, ErrNotImplemented
}
func (UnwiredProtocolAdapter) ReadTelemetry(context.Context, EntityID, []string) ([]TelemetryPoint, error) {
	return nil, ErrNotImplemented
}
func (UnwiredProtocolAdapter) WriteCommand(context.Context, EntityID, string, []byte) error {
	return ErrNotImplemented
}
func (UnwiredProtocolAdapter) Health(context.Context) error { return ErrNotImplemented }
func (UnwiredProtocolAdapter) Close(context.Context) error  { return ErrNotImplemented }
