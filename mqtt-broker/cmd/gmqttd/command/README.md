# GMQTT Daemon Commands

`mqtt-broker/cmd/gmqttd/command` contains daemon subcommands and startup/reload logic.

## Review Notes

- Problem: command changes can alter config loading, listener startup, reload behavior, or shutdown.
- Improvement: document command flags and pair lifecycle changes with broker tests.
- Expected effect: safer broker operations.
