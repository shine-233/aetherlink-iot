## 2026-07-09 IoT Operation Tooling Boundary

- Future MCP tools should preserve the same product distinction as mature IoT platforms: direct methods are immediate request-response controls for online devices, while jobs handle fleet/offline/long-running operations.
- Read-only MCP tools can safely summarize access packets, Ready Check, device twin state, command job support bundles, and release preflight results, but they must report whether evidence is fresh runtime evidence or static/catalog evidence.
- Write-capable MCP tools for commands, OTA, credential rotation, or job retries should require explicit confirmation, tenant/role scope, and an audit trail; do not expose them as simple fire-and-forget tools.
- Future provisioning tools must be treated as high-risk write tools: claim-certificate exchange, unique certificate issuance, and credential rotation need explicit confirmation, audit, redaction, and runtime evidence before they can be exposed through MCP.

## Minimal MCP Response Schemas 2026-07-09

These schemas are drafts for future tools. They are not implemented runtime tools.

`get_first_device_status` should return:

```json
{
  "evidence_kind": "business|boundary|catalog|preflight|runtime-blocked",
  "freshness": "fresh|archived|static-only|runtime-blocked",
  "generated_at": "2026-07-09T00:00:00Z",
  "source": {
    "api_paths": ["/api/v1/device/:device_id/onboarding/connection-guide"],
    "report_paths": [],
    "command": ""
  },
  "device": {
    "device_id": "redacted-or-id",
    "online": false,
    "latest_telemetry_at": null,
    "ready": false
  },
  "next_actions": [],
  "runtime_blockers": []
}
```

`get_plugin_runtime_surface` should return:

```json
{
  "evidence_kind": "catalog|runtime-blocked|business",
  "freshness": "static-only|fresh|archived",
  "plugin_id": "aetherlink",
  "runtime_surface": {
    "auth_hook": "mqtt-broker/plugin/aetherlink/hooks_auth.go",
    "lifecycle_hook": "mqtt-broker/plugin/aetherlink/hooks_lifecycle.go",
    "subscribe_hook": "mqtt-broker/plugin/aetherlink/hooks_subscribe.go",
    "message_hook": "mqtt-broker/plugin/aetherlink/hooks_messages.go",
    "debug_hook": "mqtt-broker/plugin/aetherlink/devdebug_hooks.go"
  },
  "runtime_evidence": {
    "plugin_loaded": false,
    "device_auth": false,
    "subscribe_acl": false,
    "message_forwarding": false,
    "lifecycle_transition": false,
    "debug_log_capture": false
  },
  "redactions": ["device_secret", "jwt", "mqtt_password", "database_url"],
  "runtime_blockers": []
}
```

All MCP outputs must prefer `test_payload`, `test_topic`, and `subset_limit`
wording while preserving compatibility fields in redacted internal evidence.

## MCP Future Tool Contract 2026-07-09

No live MCP runtime exists yet. Any future MCP tool must declare:

- Tool name, read/write risk level, and required role/tenant scope.
- Backend API path or report file it reads; tools should not read the database directly.
- Evidence kind: `business`, `boundary`, `catalog`, `preflight`, or `runtime-blocked`.
- Source timestamp, command/report path, and whether the evidence is fresh or archived.
- Redaction policy for secrets such as device credentials, JWTs, MQTT credentials, DB URLs, and compatibility `sample_*` fields.
- Test requirement: at minimum a schema/unit test plus API/E2E mapping before the tool can be counted as coverage evidence.

See also `references/mcp-tool-contract.md`.

Latest planning closeout: `references/mcp-tool-contract.md` now carries the
canonical first-slice MCP tool matrix so future implementation work has one
source of truth for source routes, auth scope, freshness, redaction, and
minimum verification.

External anchors: MCP security guidance requires explicit authorization and
server/operator threat modeling before tool exposure; OpenAPI/Swagger can
describe API surfaces, but stale generated docs are not runtime evidence.


# AetherLink IoT MCP 集成指南

## 2026-07-09 Latest Verification Note

- No live MCP transport, auth, or deployment wiring was added this round.
- The `/first-device` onboarding slice is now slightly cleaner for future MCP exposure because new internal focus naming prefers `test` wording while legacy compatibility inputs still map correctly.
- Current green checks remain focused frontend and automation-harness trust evidence only; they still do not prove a live MCP server, MCP auth flow, or runtime deployment path.
## MCP 覆盖证据口径

- 当前仓库没有内置 MCP Server runtime，因此 MCP 相关内容只能算设计边界或未来集成建议，不能计入当前 API/E2E 业务闭环。
- 如果未来 MCP 暴露覆盖报告读取能力，工具必须返回报告来源、生成时间、运行命令和 runtime 阻塞状态，不能只返回“通过/失败”的二次摘要。
- MCP 工具读取插件、设备或首台设备证据时，应沿用 API/E2E 的 evidence kind：`business`、`boundary`、`catalog`、`preflight` 要分开。

当前仓库没有内置 MCP Server 运行时。本文件记录建议集成边界，避免把还没有实现的 MCP 能力写成已交付功能。

## 适合暴露给 MCP 的能力

- 读取部署健康状态和首台设备 closeout manifest。
- 查询设备基础信息、在线状态、最新遥测和告警摘要。
- 触发受限的测试遥测发送或诊断读取。
- 读取 API/E2E/覆盖报告归档，而不是重新解释过期报告。

## 不应直接暴露的能力

- 不经确认的批量删除、批量命令、OTA 或大规模 Job。
- 数据库原始写入。
- 绕过租户权限的设备读取。
- 直接泄露 `template_secret`、设备密钥、JWT、数据库密码或 MQTT 凭据。
- 直接把旧 `sample_*` 兼容字段展示给终端用户；MCP 工具应优先使用 `test_payload`、`test_topic`、`subset_limit` 等新语义字段。

## 推荐架构

1. MCP Server 作为独立进程部署，不嵌进前端页面。
2. MCP Server 只调用 AetherLink 后端 API，不直接连数据库。
3. 后端 API 继续负责租户、角色、审计和限流。
4. 高风险工具先做只读版本，再逐步增加带确认的写操作。
5. 所有 MCP 工具响应都带证据来源，例如 API 路径、时间戳和设备 ID。

## 首台设备 MCP 场景

最早可做的 MCP 工具应围绕傻瓜式首台设备闭环：

- `get_first_device_status`：返回部署健康、设备在线状态、最新遥测和图表是否有数据。
- `get_first_device_connection_params`：返回脱敏后的 MQTT/HTTP 接入参数。
- `create_first_device_closeout`：读取启动 manifest 和成功证明，生成 closeout 摘要。

这些工具在实现前只能算设计建议，不能算当前功能覆盖。
## 2026-07-08 Latest Verification Note

- No MCP integration contract changed this round.
- Current green checks cover focused frontend suites and automation harness
  trust only; they do not prove live MCP transport, auth, or deployment wiring.
