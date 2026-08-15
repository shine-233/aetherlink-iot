#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
DEFAULT_COMPOSE="$ROOT_DIR/docker-compose.yml"
OPTIONAL_COMPOSE="$ROOT_DIR/deploy/docker-compose.optional-integrations.yml"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

assert_contains() {
  file="$1"
  text="$2"
  grep -F -- "$text" "$file" >/dev/null || fail "$file must contain: $text"
}

assert_service_networks() {
  file="$1"
  service="$2"
  expected="$3"
  actual="$(awk -v service="$service" '
    $0 == "  " service ":" { in_service = 1; next }
    in_service && $0 ~ /^  [A-Za-z0-9_-]+:$/ { exit }
    in_service && $0 ~ /^      - [A-Za-z0-9_-]+$/ { print $2 }
  ' "$file" | paste -sd, -)"
  [ "$actual" = "$expected" ] || fail "$file service $service networks = $actual, want $expected"
}

# Separate data planes from the application plane. Network order is kept
# stable because this contract is intended to make overlay drift visible.
assert_contains "$DEFAULT_COMPOSE" '  postgres_net:'
assert_contains "$DEFAULT_COMPOSE" '    internal: true'
assert_contains "$DEFAULT_COMPOSE" '  redis_net:'
assert_contains "$DEFAULT_COMPOSE" '  core_net:'
assert_contains "$OPTIONAL_COMPOSE" '  thingsvis_net:'

assert_service_networks "$DEFAULT_COMPOSE" postgres postgres_net
assert_service_networks "$DEFAULT_COMPOSE" redis redis_net
assert_service_networks "$DEFAULT_COMPOSE" mqtt-broker postgres_net,redis_net,core_net
assert_service_networks "$DEFAULT_COMPOSE" backend postgres_net,redis_net,core_net
assert_service_networks "$DEFAULT_COMPOSE" frontend core_net

assert_service_networks "$OPTIONAL_COMPOSE" http_adapter core_net
assert_service_networks "$OPTIONAL_COMPOSE" thingsvis-server postgres_net,thingsvis_net
assert_service_networks "$OPTIONAL_COMPOSE" thingsvis-studio thingsvis_net
assert_service_networks "$OPTIONAL_COMPOSE" backend postgres_net,redis_net,core_net
assert_service_networks "$OPTIONAL_COMPOSE" frontend core_net,thingsvis_net

# Every configured cross-service endpoint must remain reachable through a
# shared network; service names and container ports are intentionally stable.
for endpoint in \
  'P_PLATFORM_URL: http://backend:9999' \
  'P_PLATFORM_MQTT_BROKER: tcp://mqtt-broker:1883' \
  'DATABASE_URL: "postgresql://' \
  'SERVER_HOST: thingsvis-server'
do
  assert_contains "$OPTIONAL_COMPOSE" "$endpoint"
done
assert_contains "$DEFAULT_COMPOSE" 'GOTP_MQTT_BROKER: ${GOTP_MQTT_BROKER:-mqtt-broker:1883}'
assert_contains "$DEFAULT_COMPOSE" 'GOTP_DB_REDIS_ADDR: ${GOTP_DB_REDIS_ADDR:-redis:6379}'

# Exact membership above is also the negative isolation contract: frontend and
# adapter cannot reach either data plane, studio cannot reach any core service,
# and ThingsVis server cannot reach Redis unless a reviewed network is added.

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  POSTGRES_PASSWORD=contract-test \
  REDIS_PASSWORD=contract-test \
  GOTP_DB_PSQL_PASSWORD=contract-test \
  GOTP_DB_REDIS_PASSWORD=contract-test \
  MQTT_ROOT_PASSWORD=contract-test \
  MQTT_PLUGIN_PASSWORD=contract-test \
  MQTT_BROKER_ID=contract-test \
  GOTP_JWT_KEY=contract-test \
  THINGSVIS_AUTH_SECRET=contract-test \
    docker compose -f "$DEFAULT_COMPOSE" -f "$OPTIONAL_COMPOSE" \
      --profile optional-integrations config >/dev/null
  echo "Network segmentation contract: static assertions passed; merged Compose model valid"
else
  echo "Network segmentation contract: static assertions passed; Compose validation skipped"
fi
