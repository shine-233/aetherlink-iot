# Backend MQTT Integration

`backend/mqtt` contains backend MQTT integration code used by device and broker workflows.

## Folder Role

- Bridges backend device state and command behavior with MQTT transport concerns.
- Should stay aligned with the broker plugin and backend service-layer device rules.

## Review Notes

- Problem: backend MQTT behavior crosses service, broker, and device contracts.
- Improvement: update broker docs, backend service docs, and API/E2E evidence together when MQTT semantics change.
- Expected effect: consistent device pipeline behavior across backend and broker code.
