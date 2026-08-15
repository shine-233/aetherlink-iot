## Mature IoT API Anchors 2026-07-09

- Device connection APIs should return enough information to build a customer-facing access packet: endpoint, port, TLS mode, credential mode, MQTT topics, test payload, copyable command examples, and last connection error. Official reference: ThingsBoard MQTT getting-connected docs: https://thingsboard.io/docs/reference/mqtt-api/getting-connected/
- Provisioning APIs, if added, should be modeled separately from existing voucher/manual credential APIs. Official reference: AWS IoT Fleet Provisioning can generate and deliver device certificates on first connection: https://docs.aws.amazon.com/iot/latest/developerguide/provision-wo-cert.html
- Batch command and OTA APIs should model job governance separately from immediate commands: preview, submit, per-device rows, retry/cancel, support bundle, timeout, and rollout status. Official reference: AWS IoT Jobs: https://docs.aws.amazon.com/iot/latest/developerguide/iot-jobs.html
- Job APIs should expose job execution state per target device and aggregate rollout progress. Official reference: AWS IoT Jobs key concepts: https://docs.aws.amazon.com/iot/latest/developerguide/key-concepts-jobs.html
- Immediate device command APIs should stay request-response and online-only in the product language; offline or fleet operations belong to jobs. Official reference: Azure IoT Hub direct methods: https://learn.microsoft.com/en-us/azure/iot-hub/iot-hub-devguide-direct-methods
- Scheduled bulk management APIs should use a jobs model for twin updates and direct methods. Official reference: Azure IoT Hub jobs: https://learn.microsoft.com/en-us/azure/iot-hub/iot-hub-devguide-jobs
- Twin APIs should expose desired/reported state, metadata/version timestamps, and conflict/acknowledgement evidence instead of only current-vs-expected values. Official reference: Azure IoT Hub device twins: https://learn.microsoft.com/en-us/azure/iot-hub/iot-hub-devguide-device-twins
- These are design anchors for future API work. Do not cite them as implemented AetherLink runtime evidence until the matching backend route, frontend UI, automation metadata, and release API/E2E evidence exist.

## Online Direct Method Contract 2026-07-19

- `POST /api/v1/command/datas/direct-method` accepts one `device_id`, command
  `identify`, optional JSON-string `value`, and optional `timeout_seconds`.
  Omitted timeout means 10 seconds; valid explicit values are 1 through 30.
- The route is online-only and tenant/owner write-scoped. It refuses to publish
  unless the existing `command_set_logs` row is created first, then reuses the
  existing downlink topic/message ID and response uplink rather than defining a
  second device protocol. The shared log builder persists the authenticated
  operator ID for manual calls; internal/automatic calls with no actor keep the
  field null.
- Response fields deliberately separate `published` from `device_responded`
  and `device_succeeded`. `outcome` is `device_succeeded`, `device_failed`,
  `delivery_failed`, or `timeout`; raw `status`, `response_payload`,
  `error_message`, `elapsed_ms`, and the audit `message_id` remain visible.
- HTTP request cancellation stops only the short wait. It does not retract a
  command already published or remove its durable log. A timeout also preserves
  the log so a late device response can still appear in normal command history.
- Platform delivery status is conditionally updated only while the log is not
  already status `3/4`; this prevents a fast device response from being
  downgraded by a later publish callback.
- This contract is registered in the endpoint catalog and coverage source, but
  no real API/PostgreSQL/MQTT/device execution has been run in the current lane.

## Device-Scoped MQTT Debug Contract 2026-07-19

- `POST /api/v1/device/:device_id/mqtt-debug/session` opens one short-lived
  tenant/user/device-scoped session. `GET .../session/:session_id` returns the
  bounded snapshot, `POST .../command` accepts `subscribe`, `unsubscribe`, or
  `publish`, and `DELETE .../session/:session_id` closes it. Every call rechecks
  an explicit platform role plus device write permission; the client never
  submits tenant, user, broker username, or broker password.
- Canonical device-specific filters use an isolated broker client and report
  `subscription_details[].mode=broker_subscription`. Shared accepted uplinks
  report `accepted_application_uplink_observer`: they are fan-out copies after
  the production MQTT adapter resolves authoritative device and tenant identity,
  not a per-session global broker subscription and not proof of SUBACK, rejected
  frames, broker QoS, retained, or duplicate flags.
- Publish and subscribe allowlists are separate. Device status and protocol ACK
  topics are read-only; command/control/attribute/OTA downlinks require the
  current `device_number` at the protocol-defined segment, status requires the
  current `device_id`, and `$` control/shared-subscription prefixes are denied.
- Snapshots expose session expiry, `connected` for the isolated debug client's
  broker connection, and `platform_device_online` for the latest online state
  recorded by the platform device row; neither field is presented as the
  other. External `GET` snapshots are limited to four requests per session per
  second, independently from open and command budgets; open/command response
  snapshots do not consume that polling budget. They also expose subscription
  mode, the 200-message ring, per-session safety drops, global accepted-uplink
  observer drops, payload limit and subscription limit. Operation logs are metadata-only
  for these dynamic paths: they retain action/topic/QoS/payload byte count but
  never request/response MQTT payload bodies.
- This is source-level device debugging, not an arbitrary cross-device EMQX
  console. It currently reuses the server's configured broker principal behind
  application allowlists; a dedicated least-privilege debug principal, broker
  ACL/quotas, raw packet semantics and real MQTT/API/UI evidence remain open.

## Alarm Email Template API Contract 2026-07-19

- `/api/v1/notification/e-mail/templates` provides authenticated list/create;
  `/preview` renders a candidate; `/:id` updates or deletes; and
  `/:id/default` selects the default. All six routes are in the endpoint catalog
  and coverage contract, with unexecuted system/tenant lifecycle candidates.
- A tenant default lookup and the system-default fallback use separate GORM
  query instances, so tenant predicates cannot leak into and suppress the
  fallback. This remains source evidence only until migration 32, API and SMTP
  paths run in a real environment.

## Device Twin Metadata and Fleet Last-Report Contract 2026-07-19

- `GET /api/v1/device/twin/:id` row metadata is optional and additive:
  `desired_updated_at`, `desired_expires_at`, `reported_at`,
  `desired_revision`, and `last_write_source` (`desired` or `reported`). Desired
  fields come from persisted expected data; reported time comes from current
  telemetry/attribute timestamps. Equal or missing timestamps omit
  `last_write_source` instead of guessing.
- `GET /api/v1/device` accepts `last_reported_after` (inclusive Unix
  milliseconds), `last_reported_before` (exclusive Unix milliseconds), and
  `never_reported`. Omitting `never_reported` means no report-existence filter;
  `true` selects devices with no telemetry-current row, while `false` requires
  at least one. `never_reported=true` cannot be combined with either time
  bound, and the lower bound must be strictly earlier than the upper bound.
- The same three Fleet keys are accepted by user-scoped saved filters and the
  typed Command Jobs / OTA `device_filter` contracts. Boolean `false` remains a
  real filter value during frontend route and saved-filter normalization; it
  must not be converted to numeric `0` or dropped.
- These contracts are source-level only in the current checkpoint. Swagger was
  not regenerated and no PostgreSQL/API/frontend runtime verification was run.
  `twin_drift` and `ota_failed` are not accepted Fleet keys yet.

## Command Job Scheduled Start Contract 2026-07-19

- Command Job preview and submit accept optional RFC 3339/ISO `scheduled_at`.
  Omission uses the immediate path. An explicit value must be in the future and
  no more than one year ahead; otherwise the request is rejected rather than
  silently becoming immediate. A valid future value persists the Job as
  `scheduled`, is covered by the preview token and scope snapshot, and does not
  start the worker inside the submit request.
- For a future Job, `timeout_at` is calculated from the planned start
  (`scheduled_at + timeout_seconds`), not from creation time. The recovery scan
  selects only due, unexpired Jobs and uses a tenant-scoped conditional update
  from `scheduled` to `running` before dispatch, so competing scanners cannot
  both activate the same Job.
- `scheduled` Jobs can be canceled before activation. Detail/summary/list and
  support-bundle responses expose `scheduled_at`; audit events include
  `scheduled` and `started`.
- The Command Center converts the local datetime picker value to ISO, includes
  it in preview/submit fingerprinting, exposes a `scheduled` history filter and
  planned-start history column, rejects past or more-than-one-year timestamps,
  and begins refresh only when the planned time is due.
- Every ready-row claim now crosses one database quota interface. In a single
  transaction it reads PostgreSQL `clock_timestamp()`, locks the global quota
  row and then the tenant quota row, counts unexpired `dispatching` leases,
  checks both durable leaky-bucket cursors, and only then claims one row with
  `FOR UPDATE SKIP LOCKED`. A denial persists Job `next_dispatch_at`; the
  recovery scan does not resume that Job before the stored time.
- Default/source-config policy is global 16 concurrent and 20 starts/second,
  with per-tenant 4 concurrent and 5 starts/second. Detail, list and support
  responses expose `next_dispatch_at`, and governance summaries expose the
  configured quota. Process timers only optimize short wakeups and cannot
  bypass the database decision.
- Migration `34.sql` adds `command_jobs.scheduled_at` plus the partial due-scan
  index. Migration `36.sql` adds database-locked global/tenant dispatch quotas
  and `next_dispatch_at`; migration `37.sql` adds attribute/event durable
  receipts and dead letters, `38.sql` adds OTA rollout governance, and `39.sql`
  adds fenced attribute/event dead-letter claims, so the backend is now
  `VERSION_NUMBER=39`. Swagger was not regenerated, and no real 33 -> 39
  migration, PostgreSQL concurrency,
  worker, API, frontend test or browser proof ran in this checkpoint.

## Attribute/Event Dead-letter Operator Contract 2026-07-26

- `GET /api/v1/telemetry/datas/uplink-dead-letters` returns paginated metadata
  only. Supported filters are `tenant_id`, `device_id`, `data_type` (`attribute`
  or `event`) and `status`; the canonical raw envelope is not part of the
  response model or the storage select list.
- `PATCH /api/v1/telemetry/datas/uplink-dead-letters/:id/status` accepts
  `retry`, `replay`, `resolve` or `ignore` and requires `expected_status`.
  The update is scoped by that expected value, so a concurrent worker or
  operator action fails with a status conflict instead of being overwritten.
  Replay crosses the same storage
  database-clock, claim-token, lease, envelope-validation and fenced-settlement
  seam as the background worker rather than reimplementing persistence in the
  HTTP layer.
- `POST /api/v1/telemetry/datas/uplink-dead-letters/drain` replays at most 100
  ready rows. Tenant administrators remain tenant-scoped; tenant users are
  owner-scoped in the same candidate/claim query; system administrators may
  optionally filter a tenant. Three failed primary replay attempts make a row
  terminal `dead`. Before selecting candidates, drain also reaps exhausted,
  expired `processing` claims within the caller's tenant/owner scope regardless
  of the candidate `status` filter; it does not mutate another tenant as a side
  effect of an operator request.
- This is source-only evidence. Swagger was not regenerated and no compile,
  PostgreSQL, API, multi-instance, cancellation or three-role permission proof
  ran for these routes.

## Current Trusted API Entry Table 2026-07-09

Use this table when Swagger is stale. It is a source-alignment map, not runtime proof.

| Area | Route source | Handler / service source | Frontend wrapper | Swagger status | Automation evidence |
| --- | --- | --- | --- | --- | --- |
| Scene dry-run | `backend/router/apps/scene.go` registers `POST /api/v1/scene/dry-run` | `backend/internal/api/scene.go` `DryRunScene` | `frontend/src/service/api/automation.ts` | Drift candidate until Swagger is regenerated | Catalog/contract evidence only; not release runtime |
| Scene automation dry-run | `backend/router/apps/scene_automations.go` registers `POST /api/v1/scene_automations/dry-run` | `backend/internal/api/scene_automations.go` dry-run handler | `frontend/src/service/api/automation.ts` | Drift candidate until Swagger is regenerated | Seeded automation-scene evidence exists, but release E2E still blocked |
| OTA preview/support bundle | `backend/router/apps/ota.go` registers preview and support-bundle routes | OTA task API/service files | `frontend/src/service/product/update-ota.ts` | Drift candidate; older Swagger task/detail surface is incomplete | Support archive evidence is conditional, not guaranteed failed-device handoff |
| Command Jobs | `backend/router/apps/command_data.go` registers preview/submit/list/rows/support/cancel/retry | Command data and command-job service files | `frontend/src/service/api/device-command-jobs-api.ts` | Drift candidate; Swagger mainly shows older command set-log surface | Seeded API entry exists; diagnostics direct test still needs live backend runtime |
| Device onboarding/debug | `backend/router/apps/device.go` registers debug, logs, diagnostics, connection-guide, plus MQTT session open/snapshot/command/close routes | `device_debug.go`, `device_mqtt_debug.go`, `device_connection_diagnostics.go`, `device_connection_guide.go`; `backend/internal/mqttdebug/*` | `frontend/src/service/api/device-onboarding-api.ts` | Drift candidate until regenerated | MQTT routes are cataloged; source-only first-device/debug coverage exists, release API/E2E remains blocked |
| Device list RDI summary | `backend/router/apps/device.go` registers `GET /api/v1/device` | `device.go`, `device_read_queries.go`, `devices_list_read_model*.go` | `frontend/src/service/api/device.ts`; RDI Overview opts in | New query/response fields need Swagger regeneration | Source contract and unexecuted focused test source only; no runtime proof |
| Telemetry extras | `backend/router/apps/telemetry_data.go` registers statistic batch, pub, simulation, msg-count | telemetry API/service files | `frontend/src/service/api/device-telemetry-twin-api.ts` | Drift candidate beyond narrow statistic docs | Focused frontend/API coverage exists; full runtime is not closed |
| Attribute/event dead-letter ops | `backend/router/apps/telemetry_data.go` registers `GET/PATCH/POST /api/v1/telemetry/datas/uplink-dead-letters...` | `attribute_event_dead_letters.go` in API/service/model plus storage `attribute_event_dead_letter_operator.go` | No frontend operator page yet | New source route; Swagger regeneration required | Static contract only; PostgreSQL/API/role evidence not run |

### Device list RDI installation summary contract 2026-07-19

`GET /api/v1/device` keeps the existing response shape by default. An overview-style caller may send
`include_rdi_system_info_summary=true`; each visible list row then includes an authoritative
`rdi_system_info_summary` object, including `{}` when no installation values have been saved. The
projection is derived from the already selected device `additional_info`, so it adds no per-device
database or RDI configuration request.

The projection is deliberately limited to installation location/address/date, installer company,
contact/name/phone/email, controller serial number, and maintenance technician. It does not expose
customer contact fields, warranty data, arbitrary `extra_fields`, RDI settings, or share metadata.
Existing tenant and `TENANT_USER` device-owner filtering is applied before the projection is built.

## Swagger Drift Checklist 2026-07-09

Current documentation audit found likely Swagger drift. Before treating `backend/docs/swagger.yaml` as current API truth, compare it against:

- Backend route declarations in `backend/router/apps/*` and `backend/router/*.go`.
- Handler comments and request/response models under `backend/internal/api` and `backend/internal/model`.
- Frontend API wrappers under `frontend/src/service/api/*`.
- Current automation catalogs under `automation_tests/lib/test_metadata.js` and endpoint coverage tests.

Known routes that need drift attention include scene dry-run, OTA task preview/support bundle, Command Job support/saved-filter APIs, device debug/onboarding connection-guide APIs, service APIs, and telemetry statistic/simulation/pub/msg-count APIs. Regenerate `backend/docs/docs.go`, `swagger.json`, and `swagger.yaml` only after route comments are aligned; do not use stale Swagger to claim API coverage.

Minimum execution order:

1. Compare `backend/router/apps/*` route registration against `backend/internal/api/*` handlers.
2. Compare handler request/response DTOs against `backend/internal/model/*` and service return shapes.
3. Compare frontend wrappers under `frontend/src/service/api/*` and route-specific services.
4. Regenerate Swagger only after source comments are aligned, then rerun endpoint coverage contract.
5. Without release API/E2E runtime evidence, keep the result as API documentation alignment, not production runtime proof.


# AetherLink IoT API 指南

## 2026-07-09 Non-API Compatibility Boundary

API compatibility is not only request/response fields. Current cleanup must also
preserve:

- SQL seed/menu keys such as `device_service_details` while clean installs still
  use them.
- Built-in error routes `/403`, `/404`, and `/500`; the removed `/exception/*`
  custom menu chain should not be restored as current API/E2E page evidence.
- Legal old-link routes `/terms` and `/privacy`; they are reachable placeholders,
  not final legal content.
- RDI routes/fields, `x-token`, `additional_info` bridges, ThingsVis embed/SSO
  keys, GMQTT upstream module names, broker plugin hooks, and MQTT topic/packet
  contracts.

Treat changes to these names as breaking migrations unless a dual-compatibility
path and fresh API/E2E evidence are produced.

## 2026-07-09 Latest Verification Note

- Current API/E2E trust evidence is unchanged at the runtime boundary: after fixing one RDI weak assertion, the harness trust pack reports `51 passing`, but that only proves classification, oracle, endpoint, env-gate, and preflight guard behavior.
- Release API/E2E is still not closed in runtime because the machine still lacks real release accounts, `PREVIEW_URL`, `API_TARGET`, the `9725` preview proxy path, and the required Playwright proxy/reuse settings.
- This round improved frontend/API-adjacent contract confidence in the `/first-device` slice by moving onboarding focus naming from internal `sample` wording to `test` wording while keeping compatibility inputs.
- Do not overclaim from current greens: `boundary`, `catalog`, `contract`, and `preflight` coverage are stronger trust evidence now, but the stronger business-flow evidence still mainly comes from seeded automation/scene tests.
## API/E2E 证据口径

- `00_oracle_contract` 是 API/E2E 覆盖可信度的硬门之一；它会拦截未归类端点、弱业务断言、source inventory 冒充业务闭环等问题。
- `GET /deployment/health` 和 `GET /api/v1/deployment/health` 属于 `system-deployment` 能力证据，但仍是部署健康证据，不等于全业务 API 已通过。
- Release API/E2E 必须在真实后端、数据库、MQTT broker、release 账号和 9725 预览代理都可用时运行；preflight 通过前不能宣称全覆盖。

本指南用于快速判断 API 改动入口和兼容风险。Swagger 生成文件、数据库字段和前端请求类型属于共同合同，不能只改其中一边。

## 主要入口

- 后端路由：`backend/router` 和 `backend/router/apps`。
- HTTP handler：`backend/internal/api`。
- 业务服务：`backend/internal/service`。
- 数据访问：`backend/internal/dal`。
- 请求/响应模型：`backend/internal/model`。
- 前端请求封装：`frontend/src/service`、`frontend/src/typings/api.d.ts` 和具体页面组合式函数。

## 改 API 的最小动作

1. 找到路由、handler、service、DAL 和前端调用链。
2. 确认字段是否已经进入数据库、Swagger、自动化或外部设备协议。
3. 新增字段优先保持向后兼容；删除或改名必须有迁移路径。
4. 更新自动化元数据时，不要把 route smoke 写成业务证明。
5. 运行验证前先确认依赖、数据库、后端服务和预览代理是否可用。

## 兼容别名

- 批量命令预览新代码优先使用 `subset_limit` / `preview_devices`，旧 `sample_limit` / `sample_devices` 只保留为兼容字段。
- 首台设备接入新代码优先使用 `test_payload`，旧 `sample_payload` 只保留为兼容字段。
- Topic Mapping dry-run 新代码优先使用 `test_topic`，请求和响应仍保留 `sample_topic` 兼容旧调用方。
- Topic Mapping dry-run 的评估器已从 CRUD 服务文件移到
  `backend/internal/service/device_topic_mapping_dry_run.go`；`POST
  /api/v1/device/topic-mappings/dry-run`、所有权校验、`test_topic` 优先级和
  响应结构均未改变。这是内部结构调整，不是新增 API 证据。

## 首台设备 API 关注点

- 设备创建或预注册必须能返回前端生成接入参数所需的信息。
- 设备认证、MQTT/HTTP 上报和在线状态更新必须能串到 `/first-device`。
- 最新遥测查询必须能支撑页面展示“最新数据”和第一张图表。
- 成功证明只证明当前闭环步骤已经拿到证据，不等同于全平台回归通过。

## 覆盖结论口径

- API wrapper 单元测试证明前端请求形状，不证明真实后端业务。
- Go 单包覆盖证明对应包能编译并执行测试，不证明 API/E2E。
- API 自动化证明接口场景，仍需要区分真实业务断言和页面/路由冒烟。
- Playwright E2E 证明浏览器流程，不能替代后端源码覆盖。

## 当前已知阻塞

当前 `frontend/node_modules` 和 `automation_tests/node_modules` 已安装。API/E2E 运行阻塞已从依赖缺失变为环境缺口：缺真实 release 账号、`PREVIEW_URL`、`API_TARGET`、`PLAYWRIGHT_USE_PREVIEW_PROXY=1`、`PLAYWRIGHT_REUSE_EXISTING_SERVER=0` 和 9725 预览代理。

当前后端已有 `backend/coverage/router` profile，但 `go tool cover -func=coverage/router` 只显示路由层总覆盖 `3.4%`。它适合作为窄覆盖证据，不适合作为 API 或后端业务闭环证明。
## 2026-07-08 Latest Verification Note

- No public API contract rewrite landed this round.
- The main runtime API/E2E blocker is still environment setup, not missing test
  dependencies: real release accounts, `PREVIEW_URL`, `API_TARGET`,
  `PLAYWRIGHT_USE_PREVIEW_PROXY=1`, `PLAYWRIGHT_REUSE_EXISTING_SERVER=0`, and
  the 9725 preview proxy are still required before claiming API closure.
- Compatibility aliases such as `sample_limit`, `sample_devices`,
  `sample_topic`, and `sample_payload` remain compatibility fields until a
  documented migration removes them.
