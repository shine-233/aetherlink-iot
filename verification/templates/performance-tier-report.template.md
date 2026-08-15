# Performance Tier Report

Tier:
Target URL:
Backend URL:
MQTT address:
Started:
Finished:
Git commit:
Verdict: unknown
Capacity claim status: unknown

## Resource Limit Evidence

- CPU limit:
- Memory limit:
- Docker/host evidence:
- Resource snapshot:
- Confirmed resource limits: no

## Tier Profile

| Field | Value |
| --- | --- |
| CPU | |
| Memory MB | |
| Duration seconds | |
| API concurrent users | |
| MQTT clients | |
| API p95 SLO ms | |
| Error-rate max | |
| Frontend first-load p95 SLO ms | |

## Required Measured Results

| Result | Value | Raw Evidence |
| --- | --- | --- |
| Device count sustained | TODO | TODO |
| MQTT messages per second sustained | TODO | TODO |
| API p95 latency | TODO | TODO |
| API error rate | TODO | TODO |
| Frontend first load p95 | TODO | TODO |
| Backend CPU and memory peak | TODO | TODO |
| Broker CPU and memory peak | TODO | TODO |
| PostgreSQL CPU, memory, DB size | TODO | TODO |
| Redis CPU and memory peak | TODO | TODO |

## Scenario Results

| Scenario | Status | Key Metrics | Raw Evidence |
| --- | --- | --- | --- |
| api-baseline | unknown | | |
| telemetry-ingest-mqtt | unknown | | |
| browser-e2e-smoke | unknown | | |

## Linked Archives

- Startup verification:
- First-device closeout:
- API/E2E:
- Playwright:

## Blocking Gaps

- Resource limits have not been confirmed.
- Device count and message-rate capacity are not measured.
- API/E2E/Playwright archives are not linked.
- A reviewer has not approved any capacity claim.

## Verdict

Unknown until raw resource snapshots, scenario outputs, exit codes, API/E2E/
Playwright archives, and reviewer approval are recorded. Do not claim supported
device count or message rate from a tier preset alone.
