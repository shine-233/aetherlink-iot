# Roadmap backend framework

This directory is an isolated, compileable contract scaffold for the next
AetherLink roadmap. It does **not** register routes, migrations, workers,
providers, or `service.GroupApp` dependencies. Existing production behavior is
unchanged.

The contracts are intentionally implementation-neutral:

- `Relations`: generic device/asset/user/tenant relations beyond an asset tree.
- `RuleRuntime`, `TraceSink`, `DeadLetterStore`: retry, trace, replay, and DLQ.
- `OTAService`: progress state transitions, retry, and rollback hooks.
- `EdgeOperations`: node registration, heartbeat, config reconciliation.
- `NotificationProvider`: retry-safe provider adapter for external channels.
- `ProtocolAdapter`: validation, discovery, telemetry, commands, health, close.
- `AnalyticsService`: time-series query, export, and report rendering.

`Unwired*` types intentionally return `ErrNotImplemented`; they are placeholders
for another model or implementation pass. Before wiring them into production,
add tenant authorization, idempotency keys, persistence, observability,
back-pressure, and contract/integration tests.
