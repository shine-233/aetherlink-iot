# Retained Messages

`mqtt-broker/retained` contains retained-message storage and lookup implementations.

## Folder Role

- Stores retained MQTT messages and retrieves retained matches for subscribers.
- Works with broker server publish/subscribe flow and persistence choices.

## Review Notes

- Problem: retained-message behavior affects MQTT delivery semantics.
- Improvement: test wildcard matching, deletion, replacement, and persistence interactions before changing implementations.
- Expected effect: safer MQTT retained-message behavior.
