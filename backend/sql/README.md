# Backend SQL

`backend/sql` contains database migration, seed, and schema-related SQL assets.

## Migration source of truth

- `backend/pkg/global/global.go` declares `VERSION_NUMBER = 64`.
- The numbered migration chain is `1.sql` through `63.sql`, with no missing number. `backend/initialize/pg_init.go` (`CheckVersion`) reads `sys_version` and executes `sql/<n>.sql` in ascending order for every version greater than the database version and up to `63`.
- The upgrade is committed as one database transaction; the `sys_version` row is updated only after all selected scripts succeed. The migration files themselves must not create the `sys_version` table.
- `1.sql` is the baseline schema/seed. `19.sql` is an intentionally retained migration bridge, so it must remain in the sequence even though it adds no feature schema. Files `40.sql` through `53.sql` are active migrations and are not historical junk; `45.sql` registers the hidden Command Center handoff route, `46.sql` binds an optional payload schema to each device configuration, `48.sql` adds Native dashboard publication/share state, `49.sql` adds `open_api_keys.key_prefix` for API-key digest storage (hard cutover: legacy plaintext keys stop authenticating until regenerated), `50.sql` adds `devices.voucher_hash` (column + index only; existing rows are backfilled by Go startup logic in `backend/internal/dal/device_voucher_hash.go`), `51.sql` migrates credential uniqueness from the plaintext `voucher` column to a unique index on `voucher_hash`, `52.sql` adds the `device_shadow_messages` offline command queue (pending/delivered/expired/canceled + pending-only partial index), `53.sql` adds the rule chain editor tables (`rule_chains`, `rule_chain_nodes`), `54.sql` rebuilds `calculated_fields` on the tenant + device template dimension so safe govaluate expressions can derive additional telemetry keys, `55.sql` adds `device_modbus_profiles`, `56.sql` adds the `rule_chains.graph` JSONB column for the B2 visual editor plus a tenant+enabled index, `57.sql` conditionally converts `telemetry_datas`/`alarm_info` to TimescaleDB hypertables with 7-day compression when the `timescaledb` extension is present (ROADMAP C1), `58.sql` adds `entity_versions` (ROADMAP C7 Git-style version control), `59.sql` adds `theme_color` + `favicon` columns to the `logo` brand table (ROADMAP C5 white-label), `60.sql` adds `tenants.parent_tenant_id` plus the `assets` tree table (ROADMAP C2 first step), `61.sql` adds `user_totp` / `user_totp_recovery_codes` for 2FA (ROADMAP C7), and `62.sql` adds `tenant_oidc_providers` for tenant-level OIDC/SSO (ROADMAP C7), and `63.sql` seeds the casbin RBAC bootstrap — 286 protected route patterns registered in g2, full `allow` grants for the three builtin authorities (`SYS_ADMIN`/`TENANT_ADMIN`/`TENANT_USER`), and `users.authority`-driven g bindings for existing users (status-quo-preserving activation for ROADMAP C7+; per-role tightening is a row-level delete on this seed), and 64.sql adds logo.tenant_id for tenant-level white-label brand isolation.
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
