# Broker Commands

`mqtt-broker/cmd` contains command-line entrypoints for the GMQTT broker workspace.

## Folder Role

- `gmqttd/` is the broker daemon entrypoint.
- `gmqctl/` contains command-line control tooling.
- `keep_alive_test/` contains keepalive-focused runtime/test tooling.

## Review Notes

- Problem: command behavior depends on config files, plugins, listeners, and runtime credentials.
- Improvement: document startup paths and keep config examples free of secrets.
- Expected effect: easier broker deployment and safer public publishing.
