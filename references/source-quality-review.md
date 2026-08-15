# AetherLink IoT source quality review

Updated: 2026-07-06 02:15 +08:00

Scope: multi-agent static review of frontend and backend hand-written source,
tests, tooling, and publication boundaries. The detailed per-file inventory was
an internal historical snapshot and is intentionally not part of the public
source package; this file records the actionable quality findings and what was
fixed in the current pass.

## Fixed in this pass

- `backend/internal/uplink/device_online.go`
  - Added a shared helper for telemetry, attribute, and event uplinks so device
    online SSE, status automation, and delayed expected-data sending stay
    aligned.
- `backend/internal/uplink/{attribute,event,telemetry}.go`
  - Repaired mojibake comment/code line merges that swallowed real Go code.
  - Removed the broken half-deleted telemetry forwarding block.
  - Verified with `gofmt` and focused `go test ./internal/uplink`.
- `backend/internal/dal/open_api_keys.go`
  - Rewrote the file with stable comments after mojibake had swallowed the
    delete statement.
  - Fixed OpenAPI key cache invalidation to delete the actual verifier keys:
    `apikey:{api_key}` and `apikey:createdid:{api_key}`.
  - Update and delete now invalidate by API key instead of a non-existent
    `openapi:key:{id}` key.
- `backend/internal/middleware/apikey.go`
  - Collapsed the legacy standalone API-key validator into a compatibility
    wrapper over `dal.VerifyOpenAPIKey` and `dal.InvalidateOpenAPIKeyCache`.
  - Websocket telemetry API-key auth and HTTP `OpenAPIKeyAuth` now share the
    same verifier/cache behavior; focused real-path tests are still pending.
- `backend/internal/dal/open_api_keys_test.go`
  - Added focused tests for cache-key construction and nil-Redis no-op
    invalidation.
- `backend/internal/dal/devices.go` and `backend/internal/dal/devices_test.go`
  - Removed full device voucher values from not-found errors.
  - Added a focused test that keeps the error compatible with
    `errors.Is(err, gorm.ErrRecordNotFound)` without leaking the credential.
- `frontend/src/components/thingsvis/ThingsVisAppFrame.test.ts`
  - Confirmed host-save final failure and iframe source/origin checks already
    have focused coverage.
  - Removed unnecessary DOM attachment so happy-dom does not fetch the embedded
    studio URL during unit tests.
- `frontend/src/core/data-architecture/executors/DataItemFetcher.ts` and test
  - WebSocket data sources now return an explicit unsupported result instead of
    silently returning `{}`.
- `frontend/src/core/data-architecture/VisualEditorBridge.ts` and test
  - Placeholder base/component property reads now report the unsupported path
    instead of silently returning `undefined`.
- `frontend/src/core/script-engine/sandbox.ts` and
  `frontend/src/core/script-engine/sandbox.spec.ts`
  - Rejected direct `globalThis`/`this` host-object access and obvious
    same-thread infinite loops before execution.
  - Added tests for host global references and dead-loop rejection.
  - Verified with focused Vitest: 8 tests passed.

## High priority remaining risks

| Area                       | Evidence                                                                                   | Risk                                                                                                                                           | Suggested next patch                                                                                                       |
| -------------------------- | ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Script sandbox             | `frontend/src/core/script-engine/sandbox.ts`, `executor.ts`                                | User/config scripts still run in the main JS thread; regex/static checks reduce known escapes but cannot provide true preemption or isolation. | Move toward Worker/AST whitelist execution before treating user scripts as isolated.                                       |
| OpenAPI key auth tests     | `backend/internal/middleware/apikey.go`, `jwt_auth.go`, `internal/dal/open_api_keys.go`    | The implementation now shares `dal.VerifyOpenAPIKey`, but focused tests still need to cover the real HTTP and websocket auth paths.            | Add real-path tests for `OpenAPIKeyAuth`, websocket API-key auth, disabled keys, cache invalidation, and claims injection. |
| WebSocket lifecycle        | `backend/internal/api/telemetry_data.go`, `device_status_ws.go`, `pkg/global/WSManager.go` | Duplicated WS session logic and unclear channel close ownership can double-close or leak listeners.                                            | Extract a shared WS session module with one close owner and tests.                                                         |
| App lifecycle              | `internal/app/*`, `pkg/global/*`, `pkg/metrics`, diagnostics                               | Background goroutines and listeners lack cancellation/Stop coverage.                                                                           | Move Redis listeners, metrics, MQTT, diagnostics under the service manager.                                                |
| Upload/public file serving | `backend/router/router_init.go`                                                            | `/files/*filepath` direct file serving still deserves path-clean/root-boundary review.                                                         | Add path traversal behavior tests around served files.                                                                     |

## Medium priority refactors

- Split oversized backend modules:
  - `backend/internal/service/device.go`
  - `backend/internal/dal/devices.go`
  - `backend/internal/dal/telemetry_datas.go`
  - `backend/internal/api/telemetry_data.go`
- Split oversized frontend modules:
  - `frontend/src/components/thingsvis/ThingsVisAppFrame.vue`
  - `frontend/src/components/thingsvis/ThingsVisWidget.vue`
  - `frontend/src/core/data-architecture/components/common/DynamicParameterEditor.vue`
  - `frontend/src/core/data-architecture/utils/ConfigurationImportExport.ts`
  - `frontend/src/views/automation/linkage-edit/modules/edit-premise.vue`
- Consolidate duplicate frontend utilities:
  - `hooks/chart/use-echarts.ts` and `hooks/tp-chart/use-tp-echarts.ts`
  - `utils/websocketUtil.ts`, `utils/deviceStatusWebSocket.ts`, and
    ThingsVis-specific WS logic
  - duplicate deep-clone helpers in `utils/common/tool.ts` and
    `utils/deep-clone.ts`
- Clarify publication boundaries:
  - `frontend/build/config` and `frontend/build/plugins` are required build
    source, not generated output; either keep them explicitly tracked or move
    to a less ambiguous config path.
  - `backend/cmd/aetherlink-device-autotest` is an external integration tool,
    not a production command.
  - `backend/initialize/test/*_test.go` should get an integration build tag.

## Current automation/source inventory slice

- Checked `references/source-quality-review.md`,
  `automation_tests/lib/coverage_contract.js`,
  `automation_tests/lib/oracle_contract.js`,
  `automation_tests/tests/00_coverage_contract.test.js`, and
  `automation_tests/tests/00_oracle_contract.test.js`.
- No current source inventory/quality-review wording was found that promotes
  frontend request-wrapper checks, boundary API smoke, page smoke, catalog
  checks, source inventory, or source-string/AST checks into standalone
  business closure.
- This published review is static triage, not proof of a bug; its source
  evidence does not upgrade boundary, catalog, or page smoke checks into
  business closure.
- The 1808 priority scope is not a refreshed full table: it is the 1516-file
  batch plus +12 same-scope additions, -5 same-scope removals, and +285
  expanded-scope files from `mqtt-broker`, `automation_tests`, and
  `references`.
- Added an executable documentation guard:
  `coverageContract.getSourceReviewBoundaryAudit()` plus the focused
  `00_coverage_contract` case keeps the source-review boundary, the
  route/page smoke non-business rule, and the release gate wording together.

## Test evidence boundaries

- Stronger behavior evidence:
  - seeded API automation modules `18_*` through `24_*`
  - E2E login, alarm, automation, write flow, and device-detail app specs
- Partial/local behavior evidence:
  - backend service/API/middleware/storage focused Go tests
  - frontend core/store/router/component tests that assert local state and
    helper behavior
- Not business closure by itself:
  - frontend API wrapper tests that only assert endpoint/method wiring
  - request wrapper/interceptor tests that only prove request construction,
    token/header plumbing, or error-normalization branches
  - route/page smoke specs marked `@page-coverage-only` or
    `@file-page-coverage-only`
  - source inventory rows and source-quality notes
  - AST/contract tests that verify symbols exist

## Release gate still open

Fresh release evidence still requires a local backend/database, local account
preparation, preview-proxy preflight, API automation, Playwright E2E, and
archived reports under `verification/<timestamp>/`.
