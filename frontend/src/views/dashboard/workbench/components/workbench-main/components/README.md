# Dashboard Workbench Main Components

This folder contains the small cards used by the customer-facing Dashboard
Workbench route.

## Contents

- `capability-card.vue`: opens an IoT operations capability such as first-device,
  Command Center, OTA, or alarm closure.
- `shortcuts-card.vue`: opens a compact route shortcut.
- `index.ts`: exports the local cards for the workbench page.
- `__tests__/`: focused source tests for the card contracts.

## Maintenance

- Keep visible copy aligned with IoT operations tasks, not generic admin
  dashboard language.
- When a card route or visible label changes, update the focused tests in this
  folder.
- This README is static source documentation only; runtime closure still needs
  the normal API/E2E verification gate.
