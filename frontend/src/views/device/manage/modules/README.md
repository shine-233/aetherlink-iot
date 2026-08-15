# Device Manage Modules

## Scope

This folder now contains only the active manual device-add flow used by
`device/manage`.

## Active files

- `add-devices-step1.vue`
  - device basics, config selection, and create-device submission
- `add-devices-step2.vue`
  - connection fields, credential edits, and submit/update handling
- `add-devices-step3.vue`
  - success/failure handoff and next-step guidance

## Current static notes

- The old local third-party integration selector/modal branch was removed
  because it was no longer reachable from the live page flow.
- Keep this folder focused on active wizard steps; route-level redirects or
  service-access entry shells should live outside it.
