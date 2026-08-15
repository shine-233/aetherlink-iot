# MQTT Publish

## Scope

This directory owns the backend MQTT publish client used for OTA downlink.
Runtime MQTT subscriptions now live in the adapter-managed flow started from
`backend/internal/app/mqtt_service.go`, so this package is publish-only.

## Files

- `mqtt_client.go`: builds the shared Paho MQTT publish client, reconnect
  policy, and OTA topic helper.
- `publish_test.go`: checks client option construction and OTA topic rules
  without depending on a real broker.

## Notes

- The package still uses a shared process-level MQTT client. Future refactors
  should move toward an explicit publisher lifecycle instead of hidden global
  state.
- After changing topic rules or reconnect policy, verify with
  `go test ./mqtt/publish -count=1` during the later runtime validation phase.
