# Roadmap framework map

This directory is deliberately disconnected from production startup and route
registration. It gives an implementation model a stable place to begin each
roadmap slice:

- `backend/internal/roadmap/`: Go domain types and service contracts in the backend source tree.
- `frontend/src/framework/roadmap/`: TypeScript UI/API contracts in the frontend source tree.
- `scripts/`: fail-closed PowerShell release/deployment scaffolds.
- `backend/internal/roadmap/schema/relations.sql`: reviewed-before-migration generic relation schema draft, deliberately outside numbered migration discovery.

Every scaffold either returns an explicit not-implemented error or exits in a
check-only mode. Replace it with repository-native adapters only after adding
tenant authorization, persistence, idempotency, observability, and integration
tests.
