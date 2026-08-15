# Broker Docs

`mqtt-broker/docs` contains broker-specific operational and topic-mapping documentation.

## Folder Role

- Documents MQTT topic conversion, deployment behavior, and broker operation notes.
- Should stay aligned with `plugin/aetherlink` topic mapping code and backend device configuration semantics.

## Review Notes

- Problem: topic conversion docs can drift from code and database mapping behavior.
- Improvement: update docs when topic-map service, cache, or backend config semantics change.
- Expected effect: users can reproduce MQTT mapping behavior from public docs.
