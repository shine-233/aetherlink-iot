# Backend SQL

`backend/sql` contains database migration, seed, and schema-related SQL assets.

## Migration source of truth

- `backend/pkg/global/global.go` currently declares `VERSION_NUMBER = 49`.
- The numbered migration chain is `1.sql` through `49.sql`, with no missing number. `backend/initialize/pg_init.go` (`CheckVersion`) reads `sys_version` and executes `sql/<n>.sql` in ascending order for every version greater than the database version and up to `49`.
- The upgrade is committed as one database transaction; the `sys_version` row is updated only after all selected scripts succeed. The migration files themselves must not create the `sys_version` table.
- `1.sql` is the baseline schema/seed. `19.sql` is an intentionally retained migration bridge, so it must remain in the sequence even though it adds no feature schema. Files `40.sql` through `49.sql` are active migrations and are not historical junk; `45.sql` registers the hidden Command Center handoff route, `46.sql` binds an optional payload schema to each device configuration, `48.sql` adds Native dashboard publication/share state, and `49.sql` adds `open_api_keys.key_prefix` for API-key digest storage (hard cutover: legacy plaintext keys stop authenticating until regenerated).

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
