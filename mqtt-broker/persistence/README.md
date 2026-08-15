# Broker Persistence

`mqtt-broker/persistence` contains queue, session, subscription, and unacknowledged-message persistence implementations and tests.

## Folder Role

- Provides pluggable persistence backends for broker runtime state.
- Supports MQTT session durability, subscription lookup, offline queues, and inflight message recovery.

## Review Notes

- Problem: persistence changes can alter MQTT delivery guarantees.
- Improvement: pair changes with package tests and document behavior around queue limits, expiry, and Redis/memory backends.
- Expected effect: safer MQTT reliability changes.
