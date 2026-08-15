# Device Manage Module Tests

## Scope

These tests protect the active manual device-add flow.

## Covered modules

- `add-devices-step1.test.ts`
  - creation form defaults, validation, and create-device request edges
- `add-devices-step2.test.ts`
  - dynamic credential fields, payload mapping, and submit/update states
- `add-devices-step3.test.ts`
  - success/failure result rendering and callback contracts

## Current static notes

- Tests for the removed unreachable server-step wizard were deleted together
  with the dead source modules.
