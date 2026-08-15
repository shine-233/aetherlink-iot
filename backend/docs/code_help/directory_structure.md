# Backend Directory Structure

This note is a quick orientation guide for the AetherLink IoT backend tree.
Generated files, Swagger output, and runtime evidence are documented elsewhere;
this file should stay short enough to remain useful in GitHub.

## Top-Level Folders

- `cmd/`: command-line tools, code generation helpers, and device autotest
  utilities.
- `configs/`: runtime configuration examples and environment-specific config.
- `docs/`: Swagger output, API standards, design notes, and development guides.
- `files/`: static project assets such as branding files.
- `initialize/`: application startup initialization for caches, cron jobs,
  database, Redis, logging, Casbin, RSA, Viper, and related infrastructure.
- `internal/`: private backend implementation. See `backend/internal/README.md`.
- `mqtt/`: backend MQTT client configuration, publishing, and simulation publish
  helpers. Runtime subscription handling now lives under
  `internal/adapter/mqttadapter`.
- `pkg/`: shared helpers, constants, error codes, metrics, and global runtime
  utilities.
- `router/`: HTTP and SSE route initialization.
- `sql/`: database initialization and migration scripts.
- `static/`: bundled static backend pages such as metrics viewers.
- `third_party/`: external service clients such as generated gRPC code.

## Architecture Hints

- Main request flow: `router` -> `api` -> `service` -> `dal/query` -> `model`.
- MQTT flow spans `mqtt/`, `internal/adapter/mqttadapter`, device services, and
  GMQTT plugin contracts. Keep these in sync when changing topics or payloads.
- GORM Gen outputs and Swagger outputs should be regenerated rather than
  hand-edited for long-lived changes.
- Do not list removed demo folders or retired compatibility shells as active
  capabilities.

## Related Docs

- [`internal/README.md`](../../internal/README.md)
- [`mqtt/README.md`](../../mqtt/README.md)
- [`GENERATED_FILES.md`](../../../GENERATED_FILES.md)
