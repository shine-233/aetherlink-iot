# Persistence Encoding

`mqtt-broker/persistence/encoding` contains serialization helpers for persisted MQTT runtime state.

## Review Notes

- Problem: encoding changes can break compatibility with stored sessions, queues, subscriptions, and unacknowledged messages.
- Improvement: add migration notes or compatibility tests before changing serialized formats.
- Expected effect: safer persistence upgrades.
