# AetherLink IoT MCP Tool Contract

Status: design-only. There is no live MCP server, transport, auth layer, or deployment wiring in this repository yet.

## Tool Contract Template

Each future MCP tool must document:

- name: stable tool id
- purpose: one user task, not a broad admin surface
- risk: read-only, write-confirmed, or blocked-high-risk
- auth: tenant scope, role scope, and audit identity
- source: backend API route or archived report path
- evidence_kind: business, boundary, catalog, preflight, runtime-blocked
- freshness: generated_at, command, and artifact path
- redaction: which fields are hidden or summarized
- tests: schema test, API mapping test, E2E/business mapping if applicable

## Initial Safe Tools

| Tool | Risk | Source | Evidence kind |
| --- | --- | --- | --- |
| get_coverage_artifact_index | read-only | references/coverage-artifact-index.md | catalog |
| get_api_e2e_preflight_status | read-only | automation preflight output/archive | preflight |
| get_first_device_status | read-only | backend device/onboarding APIs | business only after live API/E2E evidence |
| get_plugin_runtime_surface | read-only | references/plugin-runtime-surface.md | catalog |

## First MCP Slice: Canonical Tool Matrix

This matrix is the current implementation-first MCP runbook. It is still
design-only and must not be counted as delivered runtime MCP capability until a
real server, auth layer, audit trail, and tool tests exist.

| Tool | Scope now | Risk / auth | Canonical source | Freshness / evidence | Redaction | Minimum verification |
| --- | --- | --- | --- | --- | --- | --- |
| get_first_device_status | In scope first | read-only; tenant-scoped operator or above; audit identity required | `GET /api/v1/device/:device_id/onboarding/connection-guide`, device onboarding/debug routes summarized in `references/api-guide.md` | `business` only after live API/E2E evidence; otherwise `boundary` or `runtime-blocked`; must return `generated_at` and source route list | hide credentials, JWTs, broker secrets, and raw `sample_*` compatibility fields; prefer `test_*` wording in output | schema test, API-route mapping test, and one user-visible first-device E2E mapping before it can count as trusted MCP evidence |
| get_first_device_connection_params | In scope first | read-only; tenant-scoped operator or above; audit identity required | device onboarding / connection-guide APIs documented in `references/api-guide.md` | fresh if returned from live backend at request time; otherwise `runtime-blocked`; must report credential mode, transport, and whether evidence is live or archived | redact secrets by default; only expose masked credential ids, endpoint, port, TLS mode, topic names, sample commands, and last connection error | schema test plus source mapping to onboarding API wrapper and one redaction test proving secrets are not exposed |
| create_first_device_closeout | Deferred until read path is stable | write-confirmed; tenant-scoped operator or above; explicit confirmation and audit required | closeout manifest/report inputs only; no direct DB reads; source must be declared per artifact path | may summarize `business`, `boundary`, or `preflight` evidence, but must label every artifact with freshness and provenance | redact device credentials, tokens, DB URLs, and support-bundle sensitive payloads; summarize instead of dumping raw artifacts | schema test, artifact provenance test, confirmation flow test, and at least one end-to-end audit log assertion before any runtime claim |
| get_api_e2e_preflight_status | In scope first | read-only; platform operator / release operator scope | `automation_tests` preflight output, release env gate, archived summary artifacts | `preflight`; must explicitly report `missing proxy`, missing env, and whether the result is current-run or archived | never expose real usernames, passwords, tokens, or full URLs with embedded secrets | schema test, preflight command mapping test, and stale-artifact handling test |
| get_plugin_runtime_surface | In scope first | read-only; platform operator scope | `references/plugin-runtime-surface.md` only, until a live broker report exists | `catalog` until a live MCP tool can cite broker runtime artifacts; must not overstate runtime closure | no DB credentials, broker secrets, or raw plugin config secrets | schema test plus catalog-source mapping test; do not count as broker runtime evidence |

## Source Rules For First Slice

- MCP tools must read backend APIs or archived reports, not the database
  directly.
- Every response must include source route(s) or artifact path(s), `generated_at`
  or archive timestamp, evidence kind, and freshness.
- A tool is not "implemented" just because its schema exists in docs. Runtime
  implementation requires a live server process, auth, audit logging, and tests
  proving the tool calls the intended backend/report source.
- Prefer stable customer wording such as `test_payload`, `test_topic`, and
  `subset_limit` in external tool output, while keeping legacy compatibility
  fields internal and redacted.

## External Safety Anchors

- MCP tools must be designed with explicit authorization, threat modeling, and
  server/operator responsibilities before they are exposed:
  https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices
- API/tool evidence should distinguish generated descriptions from runtime
  behavior. OpenAPI descriptions help clients discover an HTTP API surface, but
  runtime proof still requires tests or archived execution evidence:
  https://spec.openapis.org/oas/v3.2.0.html

## Blocked Until Runtime Exists

Do not count MCP as delivered until there is a server process, authentication, deployment config, audit logging, tool schemas, and tests that prove tools call the intended backend APIs.
