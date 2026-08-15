# Unacknowledged Message Persistence

`mqtt-broker/persistence/unack` contains persistence for QoS messages awaiting acknowledgement.

## Folder Role

- `mem/` implements in-memory unacknowledged message state.
- `redis/` implements Redis-backed unacknowledged message state.
- `test/` contains shared behavior tests.

## Review Notes

Unacknowledged message behavior affects QoS reliability. Keep expiry, retry, and cleanup behavior tested across implementations.
