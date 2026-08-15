# Telemetry Module

This directory owns the device-detail telemetry workbench: current values,
realtime subscription, history lookup, trend charts, simulated reporting, and
operation logs.

## File Map

- `telemetry.vue`: main entry that assembles cards, operation logs, and dialogs.
- `TelemetryRealtimeView.vue`: realtime telemetry display.
- `useTelemetryOperationsSection.ts`: operation-log visibility, simulated report
  entry, and refresh coordination.
- `useTelemetryLogState.ts`: operation-log filters, pagination, loading state,
  and result normalization.
- `useTelemetryViewShell.ts`: display-only shell state such as card sizing,
  ordering, platform width, and animation refs.
- `telemetryControlState.ts` and `telemetryDeviceOperations.ts`: command payload,
  parameter deletion, and lightweight API adapters.
- `modules/history-data.vue`: telemetry history dialog.
- `modules/time-series-data.vue`: trend chart dialog.

## Maintenance Notes

1. Keep request loading state paired with `try/finally` so failed requests do
   not leave tables or charts stuck in loading.
2. When enriching API rows for display, clone the row before adding local fields
   such as `device_id`; do not mutate response objects in place.
3. Keep `telemetry.vue` as the assembly layer. Prefer moving display-only shell
   state into small helpers instead of pulling log or control logic back into
   the page.
