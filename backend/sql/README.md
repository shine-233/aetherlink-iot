# Backend SQL

`backend/sql` contains database migration, seed, and schema-related SQL assets.

## Migration source of truth

- `backend/pkg/global/global.go` currently declares `VERSION_NUMBER = 53`.
- The numbered migration chain is `1.sql` through `55.sql`, with no missing number. `backend/initialize/pg_init.go` (`CheckVersion`) reads `sys_version` and executes `sql/<n>.sql` in ascending order for every version greater than the database version and up to `55`.
- The upgrade is committed as one database transaction; the `sys_version` row is updated only after all selected scripts succeed. The migration files themselves must not create the `sys_version` table.
- `1.sql` is the baseline schema/seed. `19.sql` is an intentionally retained migration bridge, so it must remain in the sequence even though it adds no feature schema. Files `40.sql` through `53.sql` are active migrations and are not historical junk; `45.sql` registers the hidden Command Center handoff route, `46.sql` binds an optional payload schema to each device configuration, `48.sql` adds Native dashboard publication/share state, `49.sql` adds `open_api_keys.key_prefix` for API-key digest storage (hard cutover: legacy plaintext keys stop authenticating until regenerated), `50.sql` adds `devices.voucher_hash` (column + index only; existing rows are backfilled by Go startup logic in `backend/internal/dal/device_voucher_hash.go`, and the plaintext `voucher` column stays as the dual-mode fallback until Phase 2), `51.sql` migrates credential uniqueness from the plaintext `voucher` column (UNIQUE constraint dropped) to a unique index on `voucher_hash`, matching the phase-2b stop-plaintext-persistence behavior, `52.sql` adds the `device_shadow_messages` offline command queue (pending/delivered/expired/canceled + pending-only partial index), and `53.sql` adds the rule chain editor tables (`rule_chains`, `rule_chain_nodes`), and `54.sql` rebuilds `calculated_fields` on the tenant + device_template dimension (dropping the earlier never-wired stub) so safe govaluate expressions can derive additional telemetry keys, and `55.sql` adds `device_modbus_profiles` (per-device Modbus register point tables editable from the console and pulled by the modbus plugin via OpenAPI key; profile stores mapping only, never credentials).

## Folder Role

- Provides database setup and migration material for backend services.
- May contain seed data or defaults that affect local verification and publication readiness.

## Review Notes

- Problem: SQL changes can silently alter default users, permissions, tenant data, or generated query assumptions.
- Improvement: review default accounts and secrets before publication, rerun focused backend/DAL tests after schema changes, and update generated query files when required.
- Expected effect: safer database setup and fewer runtime migration surprises.

## Review boundary

- Do not delete, merge, or renumber a migration as a cleanup operation. A migration may be needed by an older database even when a newer database already contains its objects.
- Inspect seed data and placeholders such as `CHANGE_ME_SMTP_PASSWORD` before publication. This README does not certify that a database has been upgraded or that any SQL has been executed.
- A schema snapshot elsewhere in the repository is reference material only; verify the target database version and the numbered migration chain before generating model/query code.
