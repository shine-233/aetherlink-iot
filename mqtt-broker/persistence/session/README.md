# Session Persistence

`mqtt-broker/persistence/session` contains MQTT session state persistence.

## Folder Role

- `mem/` implements in-memory sessions.
- `redis/` implements Redis-backed sessions.
- `test/` contains shared behavior tests.

## Review Notes

Session behavior affects clean start, expiry, subscriptions, and queued messages. Keep implementation changes covered by shared tests.
