# Home View

`frontend/src/views/home` is the signed-in customer entry for AetherLink IoT.
It focuses on first-device onboarding, deployment readiness, ThingsVis home
dashboard embedding, and a compatible fallback for remote dashboard cards.

## Responsibilities

- `index.vue` orchestrates the first-device guide, deployment health checks,
  ThingsVis home-dashboard mounting, and classic-card fallback rendering.
- `HomeFirstDevice*` components render the customer-facing onboarding,
  connection, telemetry, proof, and support-summary sections.
- `homeFirstDevice*` and `homeCustomerGuide.ts` files keep the source-level
  business state for the first-device flow testable outside the large SFC.

## Maintenance Notes

- If ThingsVis is unavailable, `/home` must still show the first-device and
  fallback guidance instead of blocking the customer.
- Keep customer-visible metrics tied to IoT operations evidence: device
  access, telemetry freshness, alarms, OTA, command jobs, or Ready Check state.
- Add Home-only modules and tests only when they are imported by the live
  customer path and provide a real business function.

## Refactor Direction

- Keep moving data preparation into focused helpers so the large Home SFC stays
  mostly orchestration and layout.
- Gate lower-page support/diagnostic sections by viewport or explicit customer
  action when it does not hide the primary first-device workflow.
