#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
BACKEND_DOCKERFILE="$ROOT_DIR/backend/Dockerfile"
BROKER_DOCKERFILE="$ROOT_DIR/mqtt-broker/Dockerfile"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

assert_contains() {
  file="$1"
  text="$2"
  grep -F -- "$text" "$file" >/dev/null || fail "$file must contain: $text"
}

assert_not_contains() {
  file="$1"
  text="$2"
  if grep -F -- "$text" "$file" >/dev/null; then
    fail "$file must not contain: $text"
  fi
}

# Hand-written Go runtime images must not run as root. Fixed numeric IDs keep
# volume ownership predictable without depending on host account names.
for dockerfile in "$BACKEND_DOCKERFILE" "$BROKER_DOCKERFILE"
do
  assert_contains "$dockerfile" 'addgroup -S -g 10001 aetherlink'
  assert_contains "$dockerfile" 'adduser -S -D -H -u 10001 -G aetherlink aetherlink'
  assert_contains "$dockerfile" 'USER 10001:10001'
done

# Application-facing services need bounded process counts, no ambient Linux
# capabilities, and no privilege escalation. Database image hardening remains a
# separate upgrade because its entrypoint must retain initialization rights.
for service in mqtt-broker backend frontend
do
  awk -v service="$service" '
    $0 == "  " service ":" { in_service = 1; found = 1; next }
    in_service && $0 ~ /^  [A-Za-z0-9_-]+:$/ { exit }
    in_service && $0 ~ /^    pids_limit: / { pids = 1 }
    in_service && $0 == "    cap_drop: [ALL]" { caps = 1 }
    in_service && $0 == "    security_opt: [no-new-privileges:true]" { no_new = 1 }
    END { exit !(found && pids && caps && no_new) }
  ' "$COMPOSE_FILE" || fail "$service must keep pids_limit, cap_drop ALL, and no-new-privileges"
done

assert_not_contains "$COMPOSE_FILE" 'privileged: true'
assert_not_contains "$COMPOSE_FILE" '/var/run/docker.sock'

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  POSTGRES_PASSWORD=contract-test \
  REDIS_PASSWORD=contract-test \
  GOTP_DB_PSQL_PASSWORD=contract-test \
  GOTP_DB_REDIS_PASSWORD=contract-test \
  MQTT_ROOT_PASSWORD=contract-test \
  MQTT_PLUGIN_PASSWORD=contract-test \
  MQTT_BROKER_ID=contract-test \
  GOTP_JWT_KEY=contract-test \
    docker compose -f "$COMPOSE_FILE" config >/dev/null
  echo "Container runtime security contract: 11 static assertions; Compose model valid"
else
  echo "Container runtime security contract: 11 static assertions passed; Compose validation skipped"
fi
