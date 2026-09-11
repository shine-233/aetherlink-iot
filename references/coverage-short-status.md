# Coverage trust status

Date: 2026-09-11
Revision inspected: `30fd899b3dde1920115abd7e44eb2565ae7c46b3` (dirty worktree; no commit)

## Current verdict

Phase 0 is **not trustworthy yet** and release readiness is **unknown**. The
repository-owned inspector recognizes the live Git root, and all 16 local
verification manifests are legacy; there is no current canonical archive containing a hash-bound summary,
endpoint reachability report, route-render report, authored browser-flow
results, and case-level outcomes.

Route renders are now separated from browser business flows. A page visit no
longer grants flow credit, and declared flow dimensions persist across
replacement Playwright workers. Four initial API operation suites now use exact
`file + Mocha fullTitle` managed metadata, static inventory collection does not
execute test bodies, and runtime reconciliation rejects stale, missing, duplicate,
failed, skipped, or blocked identities before producing oracle cases. The 16
managed cases also have validated stable case IDs, explicit operation dimensions,
and actor/role/state/cleanup semantics; reporter outcomes retain those canonical
identities. Metadata remains authored expectation and cannot substitute for runtime
proof.

Inspector negative controls fail closed for a wrong root, legacy-only evidence,
revision mismatch, path traversal, stale or ambiguous evidence, failed/skipped
runs, malformed/unrelated/duplicate/missing case identities, manifest/report
contradictions, credential leakage, insufficient evidence kind, and 100%
reachability without business-flow closure (all nine timeout aborts seen on
2026-09-11 were harness limits and pass unchanged once timeboxed realistically).
The canonical producer now writes
directly to unique run-scoped staging, classifies exact full versus diagnostic
scope, hashes only reports actually produced for diagnostic runs, requires all
three reports for publication, validates full candidates through the canonical
inspector, writes the manifest last, and atomically renames the complete tree so
nested Playwright artifacts survive. Failed/interrupted/service-unavailable runs
remain explicit noncanonical staging; successful publication remaps returned
paths away from staging. Cleanup is no longer hardcoded: only exact passing
operation cases carrying the cleanup dimension can produce `cleanup=passed`.
Template Market now has an authored cleanup case that deletes both created
templates and reads back exact not-found and list-absence state; it remains
runtime-unknown until that exact case passes in a current canonical archive.

## Latest focused checkpoints

- Phase 1 static automation contracts: 301 passing. These are contract results, not live API/browser evidence.
- Canonical producer/policy/inspector contracts were rerun on 2026-09-10: 61 passing. They cover scope enforcement, API-only/E2E-only diagnostics, failed-versus-dirty precedence, cleanup derivation, interrupted staging, manifest credential redaction, inspector-backed publication, atomic path remapping with nested artifacts, hash/revision/path controls, and `.staging` exclusion. These remain static contract results, not live API/browser evidence.
- Phase 2 source repairs now include fail-closed Direct APP writes, Board zero-row not-found semantics, safe plugin scaffolds, visualization capability guards, SafeLua setup recovery, and structured DataItemFetcher HTTP/script failures. Focused Go/Vitest and frontend typecheck checkpoints passed where recorded; no live browser/API proof was inferred from them.
- The active revision still lacks a fresh same-revision canonical runtime archive, so none of these checkpoints promote Phase 0 or release readiness.
- Durable scheduled-report migration 83 is now an active implementation batch. Its schema, DAL, worker lifecycle, public API, frontend workspace, and exact managed API/Playwright case identities are now authored. The managed API suite no longer asserts an immediate `queued`/`pending` state; it polls to a terminal generation/delivery state, adds nested run tenant isolation and a retry-eligibility matrix, and waits for inactive work before schedule deletion. The report Playwright flow polls the same terminal state and waits for inactive work before cleanup. Source-side claims remain unproven at runtime: `AETHERLINK_TEST_PSQL_DSN` is unset, so PostgreSQL execution of migration 83 is unverified, and no live report API/browser run has been produced. Legacy report endpoint hits and the existing in-process scheduler still receive no business-closure credit.
- The report polling hook (`frontend/src/views/visualization/report/useSelectedReportRunPoll.ts`) had two real defects now fixed: a self-inflicted `selectedRun` reassignment invalidated the in-flight request and dropped the queued follow-up refresh, and resuming from a hidden document waited a full interval instead of refreshing immediately. Re-verified on 2026-09-11: `report-model`, `useSelectedReportRunPoll` and `index` Vitest suites pass, 20/20. Source-side result only.
- Durable report persistence review delivered on 2026-09-11 as `references/report-persistence-review.md`. Seven findings (P1–P7), statically derived. The highest is **P1**: a terminally failed generation never advances `report_schedules.last_status`/`last_run_at`, because the only writer of `last_run_id` runs on generation success (`dal/report_run.go:648`) while both failure paths use a `last_run_id = run.ID` guard (`:566`, `:693`, `:731`) that can never match. That is a silent false-success surface. Also flagged: a hardcoded `status: "queued"` on every 202 run action including replays (`service/report_schedule.go:203`), a dead exported `dal.UpdateReportScheduleSummary` (`:704`), and all-or-nothing batch claims (`report_run.go:470`, `report_delivery.go:60`). None was runtime-proven; no PostgreSQL DSN was available. P1–P5 have since been
implemented in the working tree (`go build ./...`, `go vet`, `gofmt`,
`git diff --check` and in-process `go test` on the touched packages pass, which
proves compilation and existing unit coverage only) and remain unverified until a
DSN and a live API run exist. See the disposition table in the review document.
- Phase 1 static automation contracts re-run on 2026-09-11: `tests/00_coverage_contract.test.js` is **31/31 passing** and `tests/00_e2e_metadata_contract.test.js` + `tests/00_oracle_contract.test.js` are **23/23 passing** once the harness allows realistic time (individual cases take 49s–102s). Under the previous limits nine cases aborted on timeouts — these were harness limits, **not** behavioural regressions: no assertion changed, only the timebox. Two timeboxes were raised: the `test` script in `automation_tests/package.json` (30s → 180s) and the suite-level `this.timeout` in `00_e2e_metadata_contract.test.js:115` (40s → 180s, which had been overriding the CLI flag). The repeated full-repository inventory scans behind that cost are recorded as performance debt below, not fixed.

## Known debt carried into the next gate

- `tests/00_coverage_contract.test.js` repeatedly re-runs full repository inventory
  scans (49s–102s per case, ~8 min for the file; `00_e2e_metadata_contract` +
  `00_oracle_contract` add ~5 min). The raised harness timeouts make the suite
  green but do not make it fast. Caching the inventory per run is the real fix and
  is still outstanding.
- The P2 change was checked for frontend impact and has **none**: `index.vue` reads
  only `idempotent_replay` and `run_id` off the run-action response and takes real
  status from `overall_status` on the run list/detail. No frontend change needed.
  (An earlier note claiming the frontend consumed that field was wrong and is
  withdrawn.)

## Next gate

0. **Prove the P1–P5 fixes.** All five are implemented in the working tree but
   unverified: `TestReportMigration83Postgres` skips with no DSN, and the new P2
   regression guard in `tests/37_report_schedule.test.js` needs a live API run.
   Until both execute on this revision, none may be reported as fixed. Compile-time
   evidence only: `go build ./...`, `go vet`, `gofmt`, `git diff --check` and the
   in-process `go test` suites for `dal`/`model`/`service`/`app`/`api` all pass.
   P6 needs a product decision on deleted-schedule readability; P7 is deferred as
   an oracle for a state that cannot currently occur.
1. Replace backend/GMQTT file-level traceability with exact Go test-function identities, stable evidence IDs, semantic anchors, and current-run `go test -json` reconciliation.
2. Add the production-reachable placeholder/no-op gate with generated-file and scaffold boundaries; use it to block fabricated success states without treating generated protobuf defaults as implementations.
3. Finish durable report migration 83, then run its PostgreSQL suite against an authorized DSN and align its source catalogs and exact managed schema-v2 API/Playwright identities; do not publish report evidence before same-revision PostgreSQL/API/browser results exist.
4. Expand managed schema-v2 metadata beyond the report and initial operation suites, then repair the remaining reachable P0 false-success paths.
5. Only after the identity, placeholder, and P0 gates pass, run prepared-account preflight, API automation, and Playwright against the real local backend proxy. Keep real-device and target-deploy evidence separate until authorized environments exist.
