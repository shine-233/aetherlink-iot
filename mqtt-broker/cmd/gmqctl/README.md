# GMQTT Control CLI

`mqtt-broker/cmd/gmqctl` contains broker control CLI code.

## Folder Role

- `main.go` and `command/` provide command-line control entrypoints.
- `version.go` exposes CLI version metadata.

## Review Notes

- Problem: control commands operate against broker admin APIs and runtime addresses.
- Improvement: document connection flags and keep examples free of credentials.
- Expected effect: safer broker operations and clearer public docs.
