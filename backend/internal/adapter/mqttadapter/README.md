# MQTT Adapter

`backend/internal/adapter/mqttadapter` bridges the backend, MQTT broker topics,
and internal uplink/downlink message buses.

## Files

- `adapter.go`: validates incoming MQTT payloads, classifies topics, loads
  device context, records diagnostics, and forwards normalized messages.
- `client.go`: creates the MQTT client and owns reconnect lifecycle hooks.
- `subscriber.go`: subscribes to device, gateway, and response topics, then
  forwards received broker messages to the adapter.
- `publisher.go`: sends ACKs or downlink responses back to device topics.
- `topics.go`: owns topic templates and construction helpers.
- `*_test.go`: focused tests for parsing and topic construction boundaries.

## Maintenance Notes

- Treat topic strings as external contracts. Changes may affect devices, broker
  plugins, API automation, and E2E evidence.
- Keep protocol parsing separate from product business rules where possible.
- If subscription behavior changes, update backend MQTT docs, GMQTT plugin
  compatibility notes, and coverage-contract metadata together.
- Initial connection is deliberately bounded to three attempts. Connection,
  subscription, and response-publish tokens must use `waitMQTTToken` rather
  than an unbounded `Token.Wait`, so application startup and shutdown rollback
  always regain control. Paho automatic reconnect remains enabled only after a
  successful initial connection.
- The retired backend MQTT subscribe package is not a current source path;
  subscription runtime logic is here.
