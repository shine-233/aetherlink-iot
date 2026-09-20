# Frontend Contracts Framework

This directory is an implementation scaffold inside the frontend source tree for the next AetherLink roadmap phase. It has no runtime wiring or production imports yet, so another model can implement one contract at a time without changing current behavior.

## Scope

- `widget-sdk.ts`: versioned widget registry, data sources, command and lifecycle contracts.
- `dashboard-scada.ts`: dashboard persistence, draft/publish/rollback, bindings, and SCADA runtime.
- `mobile.ts`: mobile capability discovery and device/alarm/shadow/command/push contracts.
- `edge-ops.ts`: edge node health, deployment task queue, retry/cancel, and event streaming.
- `rulechain.ts`: execution trace, replay, and retry-policy vocabulary for rule-chain UI.
- `types.ts`: shared entity, telemetry, relation, and error DTOs.

## Suggested implementation order

1. Replace each interface method with adapters over existing AetherLink REST/WebSocket APIs.
2. Add schema validation (for example Zod or Valibot) at API boundaries.
3. Implement `WidgetRegistry` and one read-only telemetry widget before command widgets.
4. Add repository optimistic-concurrency tests for dashboard publish/rollback.
5. Add mobile and edge contract tests against mocked API fixtures, then real E2E tests.
6. Import from this directory only after the corresponding backend endpoint and acceptance test exist.

## Non-goals

These files do not claim that the capabilities are implemented. They are typed seams for another model or engineer to implement incrementally. Any unsupported backend operation should remain explicit in the adapter rather than silently returning fake success.
