# Roadmap framework map

This directory is deliberately disconnected from production startup and route
registration. It gives an implementation model a stable place to begin each
roadmap slice:

- `backend/framework/roadmap/`: Go domain types and service contracts.
- `framework/frontend-contracts/`: TypeScript UI/API contracts.
- `scripts/`: fail-closed PowerShell release/deployment scaffolds.
- `sql/relations.sql`: reviewed-before-migration generic relation schema draft.

Every scaffold either returns an explicit not-implemented error or exits in a
check-only mode. Replace it with repository-native adapters only after adding
tenant authorization, persistence, idempotency, observability, and integration
tests.
