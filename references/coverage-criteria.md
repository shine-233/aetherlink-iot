# AetherLink IoT Coverage Criteria

Updated: 2026-07-04 09:38 +08:00

## Evidence Rules

- Structured metadata is classification help, not runtime proof.
- Case-level `capabilityIds` are a traceability guard, not business proof.
- `@page-coverage-only` and route-smoke checks stay non-business evidence.
- Fallback-heavy, shape-only, broad-status, or handler-only cases do not count
  as business closure by themselves.
- Helper-level tests are still useful, but they only prove local mapping,
  defaulting, parsing, or guard logic. They do not replace API/E2E proof.
- Business closure needs exact status/body assertions plus real seed, mutation,
  negative, or visible-result evidence for the claimed capability.

## Current Structured Metadata Status

- P0 API case metadata is modeled for:
  `02_device`, `16_device_alarm_share`, `17_api_coverage_closure`,
  `18_seeded_device_data`, `19_seeded_alarm_notification`,
  `20_seeded_system_permission`, `21_seeded_ota_script_openapi`, and
  `22_mqtt_device_pipeline`.
- Those mixed P0 API files now use case-level `capabilityIds`.
- P1 E2E metadata remains modeled case by case, but still needs more true
  business-flow evidence in OTA, MQTT pipeline, and system/deployment areas.

## Current Trust Boundary

- Current frontend hotspot tests are partially trustworthy where they prove real
  child-event wiring.
- Current backend market tests are partially trustworthy where they prove:
  - market HTTP client request/response mapping
  - install helper/default mapping
  - publish helper/default mapping
- They are still not enough to claim full workflow closure for market
  publish/install end to end.
