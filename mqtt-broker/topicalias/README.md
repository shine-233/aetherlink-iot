# Topic Alias

`mqtt-broker/topicalias` contains MQTT topic-alias management.

## Folder Role

- Implements alias manager behavior for MQTT v5 topic alias support.
- Integrates with packet handling and client connection state.

## Review Notes

- Problem: topic alias state is per-client protocol behavior and can break interoperability if changed casually.
- Improvement: keep alias policies documented and covered by protocol tests.
- Expected effect: clearer MQTT v5 compatibility.
