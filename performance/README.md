# AetherLink IoT Performance Evidence

This folder defines the repeatable performance evidence path for lightweight
private deployments.

## Tiers

- `1c2g`: minimum single-node smoke tier.
- `2c4g`: small private deployment tier.
- `4c8g`: larger single-node validation tier.

The tier names describe resource limits, not measured results. A tier report is
valid only when the raw archive captures the host/container resource limits,
commands, exit codes, target URLs, and scenario outputs.

## Scenarios

- `api-baseline`: backend health and API readiness.
- `telemetry-ingest-mqtt`: MQTT/telemetry ingest path.
- `browser-e2e-smoke`: user-visible browser workflow smoke path.

API/E2E archives can support release evidence, but they do not replace
resource-limited performance evidence.

## Capture Evidence Scaffold

```powershell
.\performance\scripts\run-tier-benchmark.ps1 -Tier 1c2g -TargetUrl http://localhost:8080 -BackendUrl http://localhost:9999
```

The script captures resource and health evidence into:

```text
verification/performance/<timestamp>/<tier>/
```

It does **not** enforce the tier resource limits or execute the duration, API
concurrency, or MQTT client load declared in `tiers.json`. Scenario JSON files
are an intended-scenario catalog, not proof that those scenarios ran.

## Current Boundary

These files define an evidence scaffold and archive shape. They are not a
measured performance report. A real benchmark still needs a separate load
generator, enforced resource limits, raw scenario metrics, and review.

## Report Contract

Each benchmark archive should keep these files together:

- `manifest.json`: generated run metadata, tier profile, command exit codes,
  and unresolved blocking gaps.
- `summary.json`: generated summary for quick triage.
- `report.md`: generated human-readable report. It starts with
  `verdict=unknown` unless a command failed, because command success alone does
  not prove capacity.
- `raw/resource-snapshot.json`: host, Docker, Compose, and container resource
  evidence.
- Optional API/E2E and Playwright archives when release behavior is in scope.

For handoff or publication, copy
`verification/templates/performance-benchmark-manifest.template.json` into the
archive and fill it from raw evidence. Do not publish supported device counts or
message rates until resource limits, scenario metrics, and reviewer approval are
recorded.
