#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUTPUT_DIR="${1:-verification/backups/postgres-${TIMESTAMP}}"
DUMP_FILE="$OUTPUT_DIR/database.dump"
HASH_FILE="$OUTPUT_DIR/database.dump.sha256"
MANIFEST_FILE="$OUTPUT_DIR/manifest.json"

if [ -d "$OUTPUT_DIR" ] && [ -n "$(find "$OUTPUT_DIR" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
  echo "Backup refused: output directory is not empty: $OUTPUT_DIR" >&2
  exit 1
fi
mkdir -p "$OUTPUT_DIR"

if ! command -v docker >/dev/null 2>&1; then
  echo "external blocker: Docker is required to back up the Compose deployment." >&2
  exit 1
fi
if ! docker compose ps --status running postgres >/dev/null 2>&1; then
  echo "PostgreSQL service is not running." >&2
  exit 1
fi

if ! docker compose exec -T postgres sh -c 'pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' >"$DUMP_FILE"; then
  rm -f "$DUMP_FILE"
  echo "PostgreSQL backup failed." >&2
  exit 1
fi

[ -s "$DUMP_FILE" ] || { rm -f "$DUMP_FILE"; echo "PostgreSQL backup is empty." >&2; exit 1; }

if command -v sha256sum >/dev/null 2>&1; then
  HASH="$(sha256sum "$DUMP_FILE" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  HASH="$(shasum -a 256 "$DUMP_FILE" | awk '{print $1}')"
else
  rm -f "$DUMP_FILE"
  echo "A SHA-256 tool is required." >&2
  exit 1
fi
printf '%s  %s\n' "$HASH" "database.dump" >"$HASH_FILE"
BYTES="$(wc -c <"$DUMP_FILE" | tr -d ' ')"
cat >"$MANIFEST_FILE" <<EOF
{
  "schema_version": 1,
  "created_at_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "format": "postgresql-custom",
  "database_service": "postgres",
  "dump_file": "database.dump",
  "sha256": "$HASH",
  "bytes": $BYTES,
  "includes_cluster_globals": false
}
EOF

echo "Created PostgreSQL backup: $DUMP_FILE"
echo "Restore testing is required before treating this backup as recoverable."
