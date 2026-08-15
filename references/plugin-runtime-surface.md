# AetherLink IoT Plugin Runtime Surface

Updated: 2026-07-09 +08:00

This document records the current plugin runtime surface for docs, MCP planning,
and coverage classification. It is a catalog boundary, not proof that the broker
runtime has been exercised in this round.

## Broker Plugin Surface

- Plugin id: `aetherlink`.
- Runtime config: `aetherlink.yml` beside the broker config.
- Registration and lifecycle entry: `mqtt-broker/plugin/aetherlink/plugin.go`.
- Hook adapter surface: `mqtt-broker/plugin/aetherlink/hooks.go`.
- Auth hook behavior: `mqtt-broker/plugin/aetherlink/hooks_auth.go`.
- Connection lifecycle behavior: `mqtt-broker/plugin/aetherlink/hooks_lifecycle.go`.
- Subscribe ACL behavior: `mqtt-broker/plugin/aetherlink/hooks_subscribe.go`.
- Message arrival, routing, and forwarding behavior: `mqtt-broker/plugin/aetherlink/hooks_messages.go`.
- Device debug hook evidence: `mqtt-broker/plugin/aetherlink/devdebug_hooks.go` and `devdebug.go`.
- Topic mapping cache, matching, repository, and service behavior: `topicmap_*.go`.
- Internal MQTT client and diagnostics: `mqtt.go` and `mqtt_diagnostic_recorder.go`.
- DB/Redis state and device identity lookup: `db.go`.

## External Contract Boundaries

- Do not rename the `aetherlink` plugin id without a migration plan.
- Do not change config keys, MQTT root/plugin credential semantics, topic mapping
  behavior, or device debug log shape as an internal refactor.
- Keep Redis/Postgres access behind plugin helpers; future MCP tools should read
  broker evidence through backend APIs or archived reports, not direct DB access.
- Treat generated protobuf files in neighboring plugin directories as generated
  artifacts, not hand-edit targets.

## Evidence Boundary

- Catalog evidence: current file structure and docs prove the hook split exists.
- Unit evidence: focused Go tests can prove helper and topic-map behavior.
- Runtime evidence still required: a broker process with the plugin loaded,
  authenticated device connection, subscribe ACL check, message forwarding,
  lifecycle online/offline transition, and debug-log capture.
- MCP evidence still required: no live MCP tool currently calls this plugin
  surface. `get_plugin_runtime_surface` can only be counted as catalog evidence
  until a real MCP server, auth, audit, and tool tests exist.

## Runtime Evidence Checklist

Do not mark `mqtt-broker-pipeline` as business-closed until each row has fresh evidence.

| Evidence item | Minimum proof | Current status |
| --- | --- | --- |
| Plugin loaded | Broker startup log or admin/runtime report showing `aetherlink` registered | Missing fresh runtime proof |
| Device auth | Successful MQTT connect using a seeded device credential plus a rejected invalid credential | Missing fresh runtime proof |
| Subscribe ACL | Allowed telemetry/command topic subscription plus rejected cross-device or cross-tenant topic | Missing fresh runtime proof |
| Message forwarding | Published telemetry reaches backend/API-visible latest telemetry or archived broker forwarding report | Missing fresh runtime proof |
| Lifecycle state | Online and offline transition observed for the same seeded device | Missing fresh runtime proof |
| Debug log capture | Device debug mode captures sanitized connect/subscribe/message evidence | Missing fresh runtime proof |
| Negative evidence | Invalid topic, invalid credential, and unauthorized subscription produce explicit errors | Missing fresh runtime proof |

Suggested artifact shape: record command, broker config hash or path, device id,
timestamp, API target, and redaction policy. A screenshot, catalog row, or source
file path alone is not runtime evidence.

## External Documentation Anchors

- OpenAPI descriptions are useful for human and machine discovery only when kept
  synchronized with the implemented API surface: https://spec.openapis.org/oas/v3.2.0.html
- Swagger tooling can visualize OpenAPI definitions and generate docs or clients,
  but stale definitions should not be treated as runtime proof: https://swagger.io/docs/
- MCP security guidance emphasizes explicit authorization, threat modeling, and
  server/operator responsibilities before tools are exposed: https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices
