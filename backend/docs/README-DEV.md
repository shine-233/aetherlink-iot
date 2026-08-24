# Backend Development Notes

This document is a short contributor guide for AetherLink IoT backend contributors.

## Local API

The backend listens on the configured HTTP port, commonly `9999` in local validation.

Swagger files are checked in under `backend/docs/` and the router exposes Swagger UI when the backend is running:

```text
http://localhost:9999/swagger/index.html
```

## Generated Code

GORM model/query files are generated artifacts. See `../cmd/gen/README.md` and the root `../../GENERATED_FILES.md` before changing or deleting generated files.

## Configuration

- `../configs/conf.yml` and related committed configs must stay placeholder-only.
- Local secrets, RSA keys, and private overrides belong in ignored files such as `../configs/conf-localdev.yml` or environment variables.
- Do not document default super-user credentials, hardcoded permission bypasses, or real deployment secrets in committed docs/configs.
- `mqtt.enabled=false` can be used in `conf-localdev.yml` for API-only validation when local MQTT is unavailable. In that mode, MQTT and downlink startup are skipped so `/health` can still come up once database and Redis credentials are correct.
- A common local blocker sequence is: MQTT unavailable first, then Postgres password mismatch. If `9999/health` still does not listen after disabling MQTT, verify `GOTP_DB_PSQL_PASSWORD` or the `db.psql.password` value in `conf-localdev.yml`.
- `local-api-preflight.ps1` reports whether the effective Postgres password source is `env:GOTP_DB_PSQL_PASSWORD`, `envfile:...`, `config:conf-localdev.yml`, or still missing, which helps distinguish placeholder config from a bad environment override or a reusable root `.env`.
- `start-local-api.ps1` can launch the backend without persisting a real password into committed config: pass `-DbPassword ...`, export `GOTP_DB_PSQL_PASSWORD`, place it in the repo root `.env`, or keep a private password in `conf-localdev.yml`. Use `-PrintCommandOnly -NoPrompt` when you only want a safe readiness check for automation.
- `set-local-env-db-password.ps1` can create or update the repo root `.env` from `.env.example` and keep `POSTGRES_PASSWORD` / `GOTP_DB_PSQL_PASSWORD` aligned, which is useful before local backend startup or future Docker-based validation.
- `storage.telemetry_spool` is the independent filesystem fallback used only after both the primary telemetry write and PostgreSQL dead-letter write fail. Production must mount its directory on persistent storage independent from PostgreSQL, restrict access, and monitor capacity; quarantined `.corrupt*` records are retained for inspection and continue to count against capacity.
- `storage.attribute_event_spool` is a separate complete-envelope durability tier used after the attribute/event primary transaction and `uplink_storage_dead_letters` both fail. Keep it on its own private persistent directory/volume, separate from PostgreSQL, the telemetry spool, and public `./files`. Load both rule files under `deploy/observability/`; the lightweight Compose stack does not run Prometheus or Alertmanager.
- `email_templates` is migration `32.sql`. The template service keeps system (`SYS_ADMIN`) and tenant (`TENANT_ADMIN`) scopes separate, renders only the six documented plain-text variables, and falls back to the original alarm subject/body if a template cannot be loaded or rendered. The shared template manager is mounted on the system notification page and the tenant-accessible notification-group page; template versioning and rollback still need product confirmation.
- Device MQTT debug routes live under `/api/v1/device/:device_id/mqtt-debug/session`. Canonical device topics use an isolated Paho client; shared uplinks use one lazy observer over messages already accepted and identity-resolved by the production adapter. Keep `broker_subscription` and `accepted_application_uplink_observer` semantics distinct. Snapshot `connected` is only that isolated client's broker connection; `platform_device_online` is the freshly queried platform device record. External snapshot GET calls are limited independently from open/command response snapshots. Never expose broker credentials, widen the canonical topic allowlists, or restore request/response body capture for these metadata-only operation-log paths. The current server-side broker principal is not a dedicated least-privilege debug account, so production deployments still need an explicit debug principal/ACL/quotas decision and real broker validation.
- The six alarm email-template routes are registered in the automation endpoint catalog/coverage contract. Tenant-default and system-default lookup must use fresh query instances; reusing a tenant-scoped GORM statement can silently suppress the system fallback.
- `mqtt_session_revocations.worker` controls the SW3/MQTT revocation outbox retry and ACK-consumer loop. `ack_timeout` controls redelivery while an event is `awaiting_ack`; `required_broker_ids` is snapshotted per event and, when non-empty, requires every listed stable broker ID. An empty list is only the current single-broker “accept the first named ACK” mode. Redis subscriber count is retained as observability and never completes an event; only persisted ACKs move it to `acknowledged`.
- Root Compose injects `MQTT_BROKER_ID` into the broker and also maps it to `GOTP_MQTT_SESSION_REVOCATIONS_REQUIRED_BROKER_IDS`, so its single broker is an explicit required ACK target rather than relying on the empty-list fallback. Multi-broker deployments must supply a whitespace-separated backend list matching the distinct stable IDs assigned to every broker replica.
- `command_jobs.scheduled_at` is migration `34.sql`. A future timestamp persists the job as `scheduled`; the recovery scan conditionally activates a due, unexpired job before reusing the durable row-dispatch path. Command Center requires a future time within one year, shows it in job detail and history, and wakes polling when the planned time arrives. Migration `36.sql` adds database-locked global and tenant concurrency/rate quotas plus durable `next_dispatch_at` wakeups across backend instances. This still is not an external Jobs queue and does not provide cross-tenant fairness, dynamic per-tenant policy, large-rollout orchestration, or runtime proof.
- The Postgres image runs `deploy/postgres/00-run-migrations.sh` only when creating a fresh data volume. Existing volumes are upgraded by backend `PgInit -> CheckVersion`, which executes the numbered SQL files copied into the backend image and blocks startup on any failure. The current source declares `VERSION_NUMBER=50`; before traffic, verify the highest `sys_version` and the required migration chain (`29.sql` through `50.sql`) in the target database. Migrations `35.sql`–`39.sql` cover current-alarm-stream views, Command Job quotas, attribute/event receipt and dead-letter fencing, and OTA rollout governance; `40.sql` adds alarm trigger duration, `41.sql` adds payload schemas, `42.sql` adds saved-filter sharing, `43.sql` widens alarm-history remarks and rebuilds views, `44.sql` hardens legacy alarm JSON view handling, `45.sql` registers the hidden Command Center handoff route, `46.sql` binds an optional payload schema to each device configuration, `47.sql` aligns tenant-user navigation visibility, `48.sql` persists Native board publication and sharing fields, `49.sql` adds the `open_api_keys.key_prefix` display column for API-key digest storage (legacy plaintext keys stop authenticating until regenerated), and `50.sql` adds the `devices.voucher_hash` column + index (existing rows are backfilled by idempotent Go startup logic after migrations; the plaintext `voucher` column remains as the dual-mode fallback until Phase 2). This is a static source contract; it is not proof that a particular database has been migrated. Never rerun the fresh-volume all-SQL bootstrap against an existing database.

## Development Rules

- Keep tenant and permission checks explicit.
- Validate request payloads before service work.
- Prefer service-layer tests for business rules and API automation for public HTTP behavior.
- Keep Swagger and route/API contracts in sync with source changes.
