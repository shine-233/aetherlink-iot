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

## Load generator (local baseline)

`scripts/api-load-baseline.js` is the missing ruler: it drives N concurrent
workers against a target endpoint and reports p50/p90/p95/p99, throughput and
error rate. Node standard library only — no k6/autocannon dependency.

```bash
node performance/scripts/api-load-baseline.js \
  --base-url http://127.0.0.1:9999 --path /health \
  --concurrency 4 --duration 15 --warmup 3 \
  --out performance/reports/local-api-baseline-<date>.json
```

It reports `evidenceKind: "local-baseline"` and `tierClaim: null` on purpose:
it applies **no resource quota**, so its output must never be presented as
1c2g / 2c4g / 4c8g tier compliance. Warmup samples are discarded, percentiles
use nearest-rank (not interpolation), and failures count toward the error rate
instead of being dropped.

## MQTT ingest load generator

`backend/cmd/mqttbench` drives N MQTT connections against the telemetry topic
(Go, reusing the already-vendored `paho.mqtt.golang`). This is the path that
actually matters for an IoT platform, and it is the one `tiers.json`'s
`mqttClients` was written for.

```bash
cd backend && go run ./cmd/mqttbench \
  -broker tcp://127.0.0.1:1883 -topic devices/telemetry \
  -device-id <device_id> -envelope \
  -clients 4 -rate 200 -duration 15 -warmup 3 \
  -out ../performance/reports/local-mqtt-ingest-<date>.json
```

Two traps this tool exists to expose:

1. **A broker PUBACK does not mean the platform ingested anything.** The MQTT
   adapter requires the envelope `{"device_id":...,"values":"<base64(flat JSON)>"}`
   (`publicPayload.Values` is `[]byte`, so Go marshals it as base64). With a flat
   payload the broker still ACKs every message while the adapter drops 100% of
   them — "0 failures, high throughput" and total data loss look identical.
   **Always confirm delivery** with
   `automation_tests/scripts/verify-telemetry-landed.js <device_id>`; if the read
   back fails, discard the run.
2. **The latency samples are not trustworthy.** p50 came out as exactly 0 ns,
   which is physically impossible for a localhost round trip — paho's QoS 1 token
   completes before `Wait()` for a large share of publishes. Only
   `messagesPerSecond` and `failures` should be cited.

`evidenceKind: "local-baseline"` / `tierClaim: null` for the same reason as the
API generator: no resource quota is applied, so it is not tier evidence.

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
