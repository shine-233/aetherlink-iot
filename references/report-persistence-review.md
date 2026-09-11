# Durable report persistence review (migration 83)

Date: 2026-09-11
Scope: `backend/sql/83.sql` + `backend/internal/{model,dal,service,app,api}/report*` + `router/apps/report_schedule.go`
Method: **static review only.** `AETHERLINK_TEST_PSQL_DSN` is unset and no PostgreSQL
runtime was reachable, so every finding below is derived from source and SQL text.
Nothing here is runtime-proven. No files were modified by this review.

Revision: dirty worktree on `main` (no commit).

## Disposition (updated same day)

P1, P2 and P3 have been **implemented** in the working tree. They are **not
verified**: `TestReportMigration83Postgres` skips because no DSN is set, so no
PostgreSQL behaviour was exercised. `go build ./...`, `go vet` on the touched
packages and `go test` on `dal` / `model` / `service` / `app` all pass — that
proves compilation and the existing in-process tests, nothing about the fixed
SQL paths.

| Finding | Change | Verified |
| --- | --- | --- |
| P1 | `report_run.go` failure paths now call `updateReportScheduleSummaryTx` instead of the `IfCurrent` helper, so a terminal failure advances `last_run_id`/`last_status`/`last_run_at`. The helper's existing `created_at, id` monotonicity guard still prevents an older run from overwriting a newer one. | no (needs PostgreSQL) |
| P2 | `dal.ReportRunSubmission` gained `OverallStatus`; `findIdempotentReportRun` and `createIdempotentReportRun` project it from the run's real generation + delivery state; `service.reportRunAction` returns it instead of a literal. `model.ProjectReportStatus` was exported so the action and detail responses share one projector. | no (needs live API) |
| P3 | `UpdateReportScheduleSummary` deleted. `updateReportScheduleSummaryIfCurrentTx` became unreachable once P1 landed and was deleted with it. | compile-only |
| P4 | Both claim loops now `continue` instead of `return ErrReportClaimLost` when the claim update matches 0 rows, so one contended row no longer discards the batch. The delivery→run lookup after a *successful* claim still aborts on purpose: that row is already `processing`, and skipping it would orphan a leased delivery that the reaper would later mark `ambiguous`. | no (needs PostgreSQL) |
| P5 | `tenant_id` added to the `next_run_at` update in `MaterializeDueReportScheduleRuns`. | no |

A deterministic regression guard for P2 was added at the end of
`tests/37_report_schedule.test.js`: after the run is polled to a terminal state,
replaying the same `Idempotency-Key` must report that terminal projection rather
than `queued`. It still needs a live API run.

P6 and P7 were **not** changed. P6 needs a product decision (should a deleted
schedule's run history stay readable?), and P7 is a test-side alignment for a
state that is currently unreachable — changing it now would only tighten an oracle
that cannot yet fail.

All five implemented findings are compile-verified only. The decisive evidence is
still a same-revision PostgreSQL run of `TestReportMigration83Postgres` plus a live
API run of `tests/37_report_schedule.test.js`.

---

## P1 — A terminally failed generation never updates the schedule summary

**Severity: high.** This is a silent false-success surface: the schedule row keeps
reporting the *previous* outcome after the newest run has failed.

- `backend/internal/dal/report_run.go:648` — `updateReportScheduleSummaryTx` is the
  **only** writer of `report_schedules.last_run_id`. It is called from exactly one
  place: `CompleteReportGeneration`, i.e. generation **success**.
- `backend/internal/dal/report_run.go:566` (`settleClaimedReportGenerationError`,
  terminal-failure branch) and `:693` (`ReapExpiredReportGenerations`, terminal
  branch) both call `updateReportScheduleSummaryIfCurrentTx`.
- `backend/internal/dal/report_run.go:731` — that helper's predicate is
  `id = ? AND tenant_id = ? AND last_run_id = ?` with `run.ID`.

Failure scenario: create a schedule, let the first generation fail terminally
(`attempt_count` reaches `max_attempts`). Because `last_run_id` was never set to
that run, `RowsAffected = 0` and the update is a **silent no-op**. `last_status`
stays `NULL` (or keeps a stale `succeeded` from an earlier run) and `last_run_at`
never advances. The schedule list/detail and any "last run" widget then advertise
success for a run that failed.

Note the same gap exists for delivery: `settleReportDelivery:172-174` guards on
`last_run_id = delivery.RunID`, which *is* satisfied, because generation succeeded
first. So delivery failures do propagate; generation failures do not.

Fix direction: advance the summary on the failure paths too, or make
`updateReportScheduleSummaryTx` run at run-creation time (it already orders by
`created_at, id`, so the monotonicity guard in `:717-725` stays correct).

## P2 — Every 202 run action reports `status: "queued"`, including replays

**Severity: medium.**

- `backend/internal/service/report_schedule.go:201-207` — `reportRunAction`
  hardcodes `Status: "queued"` and never reads `submission.Run.GenerationStatus`.
- Replaying an `Idempotency-Key` for a run that already reached
  `succeeded` / `failed` / `ambiguous` still answers `202` with `queued`, so the
  caller believes work is still in flight.
- The API test does **not** enshrine this. `tests/37_report_schedule.test.js:248`
  asserts `status: 'queued'` only for the *first* submission, where the run really
  is `pending` and the value is correct. The replay assertion at `:259-264`
  simply never checks `status`. The gap is missing coverage, not a wrong oracle.

Fix direction: return the projected status, and assert it on a replay of a run
known to be terminal.

## P3 — `dal.UpdateReportScheduleSummary` has no production caller

**Severity: medium (audit category: API without a production consumer).**

- Defined at `backend/internal/dal/report_run.go:704-713` (original line numbers).
- A repository-wide grep including tests finds **no** call site. Dead exported
  surface. Either delete it or wire it into the P1 fix.

## P4 — Batch claim is all-or-nothing; one lost claim rolls back the whole batch

**Severity: medium (availability, not integrity).**

- `backend/internal/dal/report_run.go:470-472` — inside `ClaimReportGenerations`,
  any row whose re-check yields `RowsAffected != 1` returns `ErrReportClaimLost`,
  which aborts and rolls back **every claim already made in that batch**.
- `backend/internal/dal/report_delivery.go:60-62` — same pattern for deliveries.
- `backend/internal/dal/report_delivery.go:63-66` — after claiming, the run is
  re-read with `generation_status = 'succeeded'`; a miss returns an error and
  likewise discards the whole batch.

Failure scenario: one row in the batch was concurrently settled or reaped (or the
reaper moved it between the `SKIP LOCKED` select and the update). The entire
batch's claims are discarded and that generation/delivery work stalls for a full
worker interval. Under contention this degrades throughput to roughly one
effective claim per contested cycle.

Fix direction: skip and continue on a per-row claim loss; collect per-row errors
and return them alongside the successful claims.

## P5 — `next_run_at` update omits `tenant_id` (constraint drift)

**Severity: low.**

- `backend/internal/dal/report_run.go:181-183` — the `Updates` guard is
  `id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at = ?`, with no
  `tenant_id`.
- The sibling statements in the same function at `:112` and `:197` both include
  `tenant_id`.

Schedule IDs are UUIDv4, so this is not an exploitable cross-tenant write today.
It is inconsistent with the rest of the function and would become a real leak if
IDs were ever tenant-derived or reused.

## P6 — Soft-deleted schedules still pass the run-list tenant scope check

**Severity: low.**

- `backend/internal/dal/report_run.go:789-798` — `ensureReportScheduleTenantScope`
  counts with `Unscoped()`, so rows with `deleted_at IS NOT NULL` count as
  existing.
- Cross-tenant access still fails closed (`id AND tenant_id` → 0 → not found), so
  this is **not** a tenant leak. It does mean run history for a deleted schedule
  stays listable. Remove `Unscoped()` if the product semantics are
  "delete means unreadable".

## P7 — The API test's retryable predicate is looser than the DAL's

**Severity: low (currently unreachable, but a latent false-green).**

- Test: `automation_tests/tests/37_report_schedule.test.js:310-313` treats a run as
  retryable when `generation_status === 'failed' || delivery_status === 'failed' ||
  delivery_status === 'ambiguous'`.
- DAL: `backend/internal/dal/report_run.go:340-346` requires
  `(generation == 'failed' AND no delivery row) OR (generation == 'succeeded' AND
  delivery ∈ {'failed','ambiguous'})`.

Today a failed generation never has a delivery row, so the two agree. If that
state ever occurs, the test will expect `202` while the API returns `201002`.
Align the test to the DAL rule.

---

## Explicitly checked and found correct

- **Tenant scoping.** Every read/write in `dal/report_run.go`,
  `dal/report_delivery.go` and `dal/report_schedule.go` carries `tenant_id`, and
  the composite FKs in `83.sql:88-93, 178-179, 227-228` make cross-tenant rows
  unrepresentable. `GetReportRunContext:775` and `ListReportRunsContext:744` both
  filter by `(tenant_id, schedule_id)`.
- **State machine vs. SQL constraints.** `processing_lease_check` is satisfied:
  claim sets `claim_token`/`lease_until` (`report_run.go:462-465`), and every
  terminal settlement clears both (`report_run.go:550`, `:626-627`,
  `report_delivery.go:142`, `:204`).
- **`payload_check`.** Terminal delivery settlement sets `payload = NULL`
  (`report_delivery.go:149` and `:204`) while `payload_size` stays for history —
  matching the constraint's terminal branch (`83.sql:185-187`).
- **`terminal_time_check`.** `completed_at` is written on every terminal
  transition and reset to `NULL` on retry (`report_run.go:554`).
- **Fencing.** All settlements re-check `claim_token` **and** `lease_until > now`
  before writing, so an expired lease cannot overwrite a newer owner
  (`report_run.go:557`, `report_delivery.go:161`).
- **Retry semantics.** Expired SMTP claims become `ambiguous`, never a silent
  retry (`report_delivery.go:179-223`), which is the correct choice for a
  non-idempotent side effect.

## Limits of this review

No PostgreSQL execution, no live API or browser run, no `go test` against a real
database. Findings P1–P4 are structural and do not depend on runtime, but the
fixes for them will need same-revision PostgreSQL evidence before any of this
batch can be published as verified.

### Addendum: the migration-83 harness could never have produced that evidence

The absence of PostgreSQL evidence for migration 83 was **not only** a missing
DSN. `openReport83Postgres` in
`backend/internal/dal/report_migration83_postgres_test.go` built its connection
with:

```go
connectionString := stdlib.RegisterConnConfig(configuration)
db, err := gorm.Open(postgres.Open(connectionString), ...)
```

`stdlib.RegisterConnConfig` returns a `database/sql` *registration name* (such as
`registeredConnConfig0`), not a DSN. `postgres.Open` parses its argument as a DSN,
so the open fails. The observable effect was misleading: with no DSN configured
the test skipped cleanly, but **the moment a DSN was supplied it would have failed
at connection time** — meaning this file had never produced a single real
PostgreSQL result, and could not have, regardless of who ran it.

The corrected form opens a `*sql.DB` and hands it to gorm, which also makes the
per-schema `search_path` take effect:

```go
schemaDB := stdlib.OpenDB(*configuration)
db, err := gorm.Open(postgres.New(postgres.Config{Conn: schemaDB}), ...)
```

Consequences for this review:

- P1 was **doubly** blocked: no DSN *and* a harness that could not connect. Only
  the first remains. Supplying a DSN should now exercise
  `updateReportScheduleSummaryTx` for real.
- Any earlier claim of PostgreSQL-backed evidence for migration 83 should be
  treated as unsupported, since the harness could not have produced it.
- A repository-wide grep for `RegisterConnConfig` in the backend finds no other
  call site, so this defect was confined to the migration-83 harness. The other
  PostgreSQL-only suites do not share it.
