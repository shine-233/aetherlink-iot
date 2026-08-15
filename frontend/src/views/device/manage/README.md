# Device Manage

## Scope

`frontend/src/views/device/manage` is the active device-fleet entry page. It owns
device search/filtering, fleet bulk actions, group assignment, first-device
entry points, and the live path into service-access and device details.

## Current structure

- `index.vue`
  - list shell, fleet actions, onboarding entry, and drawer orchestration
- `modules/add-devices-step1.vue`
  - create-device form and config selection
- `modules/add-devices-step2.vue`
  - dynamic connection form and credential update
- `modules/add-devices-step3.vue`
  - result and closeout state

## Current static notes

- The old unreachable server-step drawer flow has been removed.
- The visible "Add by Third-party Integration" entry now routes through the live
  `service-access` path instead of a dead local wizard branch.
- Future cleanup should keep shrinking list-shell orchestration out of
  `index.vue` where possible.
