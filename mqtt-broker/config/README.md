# Broker Config Package

`mqtt-broker/config` contains GMQTT configuration parsing and validation.

## Folder Role

- Loads YAML config, plugin config defaults, logging settings, listener settings, persistence settings, and MQTT runtime options.
- Provides config structures used by broker command startup and server initialization.

## Review Notes

- Problem: config defaults are part of the deployment contract.
- Improvement: document new keys, validate unsafe combinations, and update `cmd/gmqttd/default_config.yml` when behavior changes.
- Expected effect: clearer deployment behavior and fewer misconfigured broker runs.
