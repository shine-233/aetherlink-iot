# Queue Persistence

`mqtt-broker/persistence/queue` contains offline/outgoing MQTT message queue persistence.

## Folder Role

- `mem/` implements in-memory queue state.
- `redis/` implements Redis-backed queue state.
- `test/` contains shared behavior tests for queue implementations.

## Review Notes

- Problem: memory and Redis queue behavior can drift.
- Improvement: keep shared tests as the contract and update both implementations together.
- Expected effect: consistent MQTT delivery behavior across persistence backends.
