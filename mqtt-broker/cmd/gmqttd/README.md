# GMQTT Daemon

`mqtt-broker/cmd/gmqttd` contains the broker daemon entrypoint.

## Folder Role

- `main.go` registers the daemon command.
- `plugins.go` imports enabled plugins, including the AetherLink integration plugin.
- `command/start.go` parses config, creates listeners, initializes the server, handles signals, and runs shutdown.
- `default_config.yml` is a reference runtime config and should not carry environment secrets.

## Review Notes

- Problem: startup changes can alter plugin loading, TLS/listener behavior, or shutdown semantics.
- Improvement: pair startup changes with broker lifecycle tests and update config documentation.
- Expected effect: safer broker runtime maintenance.
