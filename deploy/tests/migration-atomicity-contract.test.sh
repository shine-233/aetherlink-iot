#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
MIGRATION_SCRIPT="$ROOT_DIR/deploy/postgres/00-run-migrations.sh"
SQL_DIR="$ROOT_DIR/backend/sql"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

contains() {
  grep -F -- "$2" "$1" >/dev/null || fail "$1 must contain: $2"
}

[ -f "$MIGRATION_SCRIPT" ] || fail "missing migration bootstrap script"

# The schema chain and sys_version marker must share one fail-fast transaction.
contains "$MIGRATION_SCRIPT" '} | psql --single-transaction -v ON_ERROR_STOP=1 \'
contains "$MIGRATION_SCRIPT" '--dbname "$POSTGRES_DB" -f -'
contains "$MIGRATION_SCRIPT" 'CREATE TABLE IF NOT EXISTS public.sys_version'
contains "$MIGRATION_SCRIPT" 'DELETE FROM public.sys_version;'
contains "$MIGRATION_SCRIPT" "cat \"\$sql_file\""
contains "$SQL_DIR/3.sql" 'CREATE SEQUENCE IF NOT EXISTS public.casbin_rule_id_seq'
contains "$SQL_DIR/3.sql" 'CREATE TABLE IF NOT EXISTS public.casbin_rule'
contains "$SQL_DIR/3.sql" 'CREATE UNIQUE INDEX IF NOT EXISTS idx_casbin_rule'

psql_calls="$(grep -c '^} | psql ' "$MIGRATION_SCRIPT")"
[ "$psql_calls" -eq 1 ] || fail "migration bootstrap must use exactly one psql process (found $psql_calls)"

# A future non-transactional migration must not silently weaken atomic bootstrap.
if grep -E -i -- '(^|[[:space:]])(CREATE[[:space:]]+(UNIQUE[[:space:]]+)?INDEX[[:space:]]+CONCURRENTLY|REINDEX([^;]*)[[:space:]]+CONCURRENTLY|VACUUM([[:space:]]|;)|CREATE[[:space:]]+DATABASE|DROP[[:space:]]+DATABASE)' "$SQL_DIR"/*.sql >/dev/null; then
  fail "numbered migrations contain a statement incompatible with atomic bootstrap"
fi

echo "Migration atomicity contract: 10 assertions passed"
