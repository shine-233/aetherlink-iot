# AetherLink IoT Coverage Artifact Index

Updated: 2026-07-09

This file indexes coverage and automation artifacts by what they prove. It is not a release certificate.

| Artifact | Current evidence | Trust boundary |
| --- | --- | --- |
| automation test catalog | `npm.cmd run test:list` lists 38 API modules and 17 E2E modules | Catalog only; not runtime success |
| harness trust pack | `npm.cmd run test:spec -- tests/00_coverage_contract.test.js tests/00_endpoint_coverage.test.js tests/00_oracle_contract.test.js tests/00_runtime_config_env.test.js tests/00_preflight_api_e2e.test.js` first caught an RDI `toBeTruthy()` weak assertion, then passed with 51 passing after strengthening it | Harness consistency and fake-coverage resistance only |
| API/E2E preflight | fails closed due missing release accounts, `PREVIEW_URL`, `API_TARGET`, 9725 proxy settings | Correctly blocks release claim |
| backend service profile | existing `backend/coverage/service`, `go tool cover -func=coverage/service` total 24.8% | Existing profile, not fresh rerun |
| backend router profile | existing `backend/coverage/router` historically around 3.4% | Narrow router package proof only |
| frontend focused coverage | latest attempted focused Vitest coverage timed out | No fresh frontend coverage claim this round |
| Playwright E2E reports | no fresh release E2E run this round | Missing runtime evidence |
| frontend chart-config focused Vitest | `pnpm.cmd exec vitest run src/views/device/template/components/step/__tests__/app-chart-config.test.ts src/views/device/template/components/step/__tests__/web-chart-config.test.ts` passed, 2 files / 6 tests after field-merge and `smartDeepClone` save changes | Focused component behavior only |
| command delivery diagnostics seeded API entry | `tests/25_seeded_command_jobs.test.js` now directly calls `/command/datas/delivery/diagnostics/:device_id`; this round only ran `node --check` for the changed JS files | Direct coverage entry exists; not runtime proof until executed against a seeded backend |
| device list seeded row assertion | `tests/02_device.test.js` now requires `/device` list results to include the seeded device id and concrete `id/name/pid_number` row fields; harness remains 51 passing | API test assertion strength only; not a fresh backend runtime run |
| device group tree row assertion | `tests/02_device.test.js` now creates a group, recursively validates `/device/group/tree`, and requires the created group by `id/name`; harness remains 51 passing | API test assertion strength only; not a fresh backend runtime run |
| broker plugin util coverage | `go test ./plugin/aetherlink/util` passed and reported 97.0% statements | Utility topic validation only; not full plugin runtime |
| broker plugin focused coverage | `go test ./plugin/aetherlink -run ...` timed out while producing a profile file | No complete package pass claim |

## Next Action Index

| Artifact / gap | Next command or action | Prerequisites | Success artifact | Still cannot claim |
| --- | --- | --- | --- | --- |
| Release API/E2E | `npm.cmd run preflight:api-e2e`, then release API/E2E run only after preflight passes | Real release accounts, `PREVIEW_URL`, `API_TARGET`, 9725 proxy, `PLAYWRIGHT_USE_PREVIEW_PROXY=1`, `PLAYWRIGHT_REUSE_EXISTING_SERVER=0` | Preflight pass plus archived API/E2E JSON/HTML report | Full business closure unless seeded API and Playwright flows pass |
| Weak API assertions | Continue replacing conditional-empty and nullable helper assertions in `automation_tests/tests` | Existing automation deps | Harness trust pack remains green and weak counts drop | Runtime success for the touched APIs |
| Backend service/router coverage | Re-read or regenerate focused profiles with package-scoped `go test`, not `go test ./...` | Go toolchain, backend deps, enough timeout | Fresh `coverage/*.out` profile plus `go tool cover -func` output | API/E2E or DB/broker behavior |
| Plugin runtime | Execute broker with `aetherlink` plugin and capture auth/ACL/message/lifecycle/debug evidence | Broker config, seeded device credential, backend target | Runtime evidence report with redactions | MCP runtime or release API/E2E |
| MCP integration | Implement read-only schema/unit tests before any write tool | MCP server design, auth model, audit policy | Tool schema tests and sample redacted responses | Live MCP deployment until server/auth/runtime exists |

## Current False-Coverage Corrections

- command delivery diagnostics ownership: `GET /api/v1/command/datas/delivery/diagnostics/:device_id` belongs to command-jobs only; no device-telemetry traceability credit.
- RDI operation interval coverage now uses concrete `value/min/max` props instead of `toBeTruthy()` existence proof.
- Device list coverage now proves the seeded device appears in the tenant list instead of relying on conditional first-row checks.
- Device group tree coverage now creates a group and proves it appears in the returned tree instead of hiding row checks behind a non-empty array branch.
- OTA support archive naming: current API/E2E evidence covers support archive shape and conditional Ready Check handoff fields, not guaranteed failed-device handoff.
- `e2e/20_command_jobs.spec.js` mocked failed-device support handoff is boundary evidence, not real business closure.
- `e2e/21_ready_check_command_draft.spec.js` route-draft preload is boundary/UX evidence, not Ready Check-to-preview-submit closure.
- OTA support archive remains environment/seed blocked when no OTA package/task exists.
- MCP is design-only and has no runtime coverage.
