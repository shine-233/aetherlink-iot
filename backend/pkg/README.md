# Backend Shared Packages

`backend/pkg` contains shared backend infrastructure used by internal packages.

## Folder Role

- Common helpers, constants, error codes, global runtime state, utilities, and cross-cutting support code live here.
- Packages under this folder should stay generic enough to be reused across service, API, DAL, middleware, and initialization code.

## Review Notes

- Problem: shared helpers can become hidden business-rule dependencies.
- Improvement: keep domain-specific behavior in `internal/service` and use `pkg` for stable infrastructure only.
- Expected effect: clearer ownership and safer reuse across backend modules.
