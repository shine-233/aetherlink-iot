# Broker Shared Packages

`mqtt-broker/pkg` contains shared broker infrastructure packages.

## Folder Role

- Packet parsing, MQTT codes, bitmap helpers, PID file support, and shared utilities live here.
- Packages are used by server, command, persistence, and plugin code.

## Review Notes

- Problem: shared package changes can affect the MQTT protocol path broadly.
- Improvement: keep protocol helpers small, tested, and documented before changing server/client flows.
- Expected effect: more reliable broker maintenance.
