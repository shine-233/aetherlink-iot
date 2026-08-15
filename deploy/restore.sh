#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

DUMP_FILE="${1:-}"
CONFIRM="${2:-}"
[ -n "$DUMP_FILE" ] || { echo "Usage: $0 <database.dump> --confirm-restore" >&2; exit 2; }
[ "$CONFIRM" = "--confirm-restore" ] || { echo "Restore refused: pass --confirm-restore explicitly." >&2; exit 2; }
[ -f "$DUMP_FILE" ] || { echo "Dump file not found: $DUMP_FILE" >&2; exit 1; }

HASH_FILE="$DUMP_FILE.sha256"
if [ ! -f "$HASH_FILE" ]; then
  HASH_FILE="$(dirname -- "$DUMP_FILE")/database.dump.sha256"
fi
[ -f "$HASH_FILE" ] || { echo "Restore refused: SHA-256 sidecar is required." >&2; exit 1; }

EXPECTED="$(awk 'NR==1 {print $1}' "$HASH_FILE")"
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "$DUMP_FILE" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "$DUMP_FILE" | awk '{print $1}')"
else
  echo "A SHA-256 tool is required." >&2
  exit 1
fi
[ "$ACTUAL" = "$EXPECTED" ] || { echo "Restore refused: dump SHA-256 mismatch." >&2; exit 1; }

docker compose ps --status running postgres >/dev/null 2>&1 || { echo "PostgreSQL service is not running." >&2; exit 1; }

echo "Restoring PostgreSQL database from verified custom-format dump..."
# Keep destructive cleanup and object recreation atomic: a restore error must not
# leave the live database in a partially restored state.
docker compose exec -T postgres sh -c 'pg_restore --clean --if-exists --exit-on-error --single-transaction --no-owner -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <"$DUMP_FILE"
docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "ANALYZE;"'
echo "PostgreSQL restore completed. Validate application health before routing traffic."
