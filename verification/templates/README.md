# Verification Templates

These templates are starting points for runtime evidence archives. Copy a
template into a timestamped archive only after the corresponding runtime flow
has actually produced raw evidence.

## First Device Closeout

Use `first-device-closeout-manifest.template.json` after a real private
deployment startup and homepage first-device flow.

Required evidence before changing `verdict` away from `unknown`:

- Startup verification archive and `manifest.json`.
- Startup `first_device_url` proving the exact workbench URL operators used.
- Delivery URLs from the success proof: `/first-device` and
  `/home?onboarding=first-device&focus=proof`.
- First-run admin and tenant-admin evidence.
- First device identity and connection parameters.
- Copied sample or publish command evidence.
- Web MQTT/HTTP online tester or real device publish evidence.
- Online state, latest telemetry object, and first chart evidence.
- Deployment health rows shown on the first-device workbench.
- Downloaded `aetherlink.first-device.success-proof.v1` JSON.
- Success-proof handoff summary, proof items, and current blocker state.
- API/E2E/Playwright archive paths when those runs are enabled.

Keep secrets out of the copied manifest. Record only presence/redaction states
for passwords, tokens, and credentials.

The deploy helpers can pre-fill the copied manifest from a startup manifest and
the downloaded first-device success proof:

```powershell
.\deploy\first-device-closeout.ps1 -StartupManifest verification\startup-...\manifest.json -SuccessProof path\to\aetherlink-first-device-proof.json
```

```sh
sh ./deploy/first-device-closeout.sh --startup-manifest verification/startup-.../manifest.json --success-proof path/to/aetherlink-first-device-proof.json
```

The generated closeout manifest still keeps `verdict=unknown` until a person
reviews the real runtime evidence and links the API/E2E/Playwright archive.
It mirrors the browser-downloaded success proof but does not replace the raw
proof file or runtime archive.

## Performance Benchmark

Use `performance-benchmark-manifest.template.json` and
`performance-tier-report.template.md` after a real resource-limited benchmark
run, not when merely selecting `light`, `standard`, or `production`.

Required evidence before changing `verdict` or `capacity_claim_status` away from
`unknown`:

- Confirmed resource limits for 1C/2GB, 2C/4GB, or 4C/8GB.
- Raw `resource-snapshot.json` with Docker/host evidence.
- API baseline latency and error-rate outputs.
- MQTT telemetry ingest device count, message rate, publish success rate,
  latest-telemetry confirmation, and broker metrics.
- Browser smoke or Playwright archive proving the first-device path still works
  under the selected tier.
- CPU, memory, PostgreSQL size, Redis, backend, frontend, and broker peak
  resource measurements.
- Reviewer approval for any device-count or message-rate capacity statement.

The performance helper writes generated archives under
`verification/performance/<timestamp>/<tier>/`. The generated report still keeps
capacity claims `unknown` until the measured fields are filled from raw evidence.
