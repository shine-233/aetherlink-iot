#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
DEFAULT_COMPOSE="$ROOT_DIR/docker-compose.yml"
OPTIONAL_COMPOSE="$ROOT_DIR/deploy/docker-compose.optional-integrations.yml"
DEPLOY_README="$ROOT_DIR/deploy/README.md"

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

# The default single-node stack must remain lightweight and must not acquire
# the external ThingsVis or HTTP-adapter services by accident.
assert_not_contains "$DEFAULT_COMPOSE" "thingsvis-server:"
assert_not_contains "$DEFAULT_COMPOSE" "thingsvis-studio:"
assert_not_contains "$DEFAULT_COMPOSE" "http_adapter:"
assert_not_contains "$DEFAULT_COMPOSE" "optional-integrations"

# Every service introduced or overridden by the optional file is profile-gated.
for service in http_adapter thingsvis-server thingsvis-studio backend frontend
do
  awk -v service="$service" '
    $0 == "  " service ":" { in_service = 1; found = 1; next }
    in_service && $0 ~ /^  [A-Za-z0-9_-]+:$/ { exit }
    in_service && $0 ~ /^    profiles: \[optional-integrations\]$/ { gated = 1 }
    END { exit !(found && gated) }
  ' "$OPTIONAL_COMPOSE" || fail "$service must require the optional-integrations profile"
done

# Optional runtimes keep their public port variables, but default to the same
# loopback-only host boundary as the core single-node stack.
assert_contains "$OPTIONAL_COMPOSE" '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${HTTP_ADAPTER_PORT:-19090}:19090'
assert_contains "$OPTIONAL_COMPOSE" '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${HTTP_ADAPTER_HTTP_PORT:-19091}:19091'
assert_contains "$OPTIONAL_COMPOSE" '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${THINGSVIS_SERVER_PORT:-8000}:8000'
assert_contains "$OPTIONAL_COMPOSE" '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${THINGSVIS_STUDIO_PORT:-3000}:3000'

# Preserve the deployed service names, ports, seeded adapter endpoint, and the
# mandatory ThingsVis secret rather than replacing the integration with mocks.
assert_contains "$OPTIONAL_COMPOSE" 'P_PLATFORM_URL: http://backend:9999'
assert_contains "$OPTIONAL_COMPOSE" 'P_PLATFORM_MQTT_BROKER: tcp://mqtt-broker:1883'
assert_contains "$OPTIONAL_COMPOSE" 'AUTH_SECRET: ${THINGSVIS_AUTH_SECRET:?Set THINGSVIS_AUTH_SECRET in .env to enable ThingsVis}'
assert_contains "$OPTIONAL_COMPOSE" 'SERVER_HOST: thingsvis-server'
assert_contains "$OPTIONAL_COMPOSE" 'VITE_ENABLE_THINGSVIS_COMPAT: "Y"'
assert_contains "$DEPLOY_README" 'backend address `http_adapter:19091`'
assert_contains "$DEPLOY_README" 'These host ports bind to `127.0.0.1` by'
assert_contains "$DEPLOY_README" 'Set `AETHERLINK_BIND_ADDRESS`'

# Non-localizable runtime/data prerequisites remain explicit and fail closed.
assert_contains "$DEPLOY_README" '`external-blocked`, but it does not make the required lightweight stack report'
assert_contains "$DEPLOY_README" 'external-blocked E2E paths'
assert_contains "$DEPLOY_README" '`THINGSVIS_MIRRORED_DASHBOARD_ID`'
assert_contains "$DEPLOY_README" 'Do not weaken that check or insert a fake row'

# If Docker Compose is available, validate both the default and opt-in merged
# models. Static assertions above remain useful on packaging/CI hosts without it.
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  POSTGRES_PASSWORD=contract-test \
  REDIS_PASSWORD=contract-test \
  GOTP_DB_PSQL_PASSWORD=contract-test \
  GOTP_DB_REDIS_PASSWORD=contract-test \
  MQTT_ROOT_PASSWORD=contract-test \
  MQTT_PLUGIN_PASSWORD=contract-test \
  MQTT_BROKER_ID=contract-test \
  GOTP_JWT_KEY=contract-test \
    docker compose -f "$DEFAULT_COMPOSE" config --services >"${TMPDIR:-/tmp}/aetherlink-default-services.$$"

  default_services="$(cat "${TMPDIR:-/tmp}/aetherlink-default-services.$$")"
  rm -f "${TMPDIR:-/tmp}/aetherlink-default-services.$$"
  for optional_service in http_adapter thingsvis-server thingsvis-studio
  do
    printf '%s\n' "$default_services" | grep -Fx -- "$optional_service" >/dev/null && \
      fail "default Compose unexpectedly enables $optional_service"
  done

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
      --profile optional-integrations config --services >"${TMPDIR:-/tmp}/aetherlink-optional-services.$$"

  optional_services="$(cat "${TMPDIR:-/tmp}/aetherlink-optional-services.$$")"
  rm -f "${TMPDIR:-/tmp}/aetherlink-optional-services.$$"
  for required_service in http_adapter thingsvis-server thingsvis-studio frontend
  do
    printf '%s\n' "$optional_services" | grep -Fx -- "$required_service" >/dev/null || \
      fail "opt-in Compose is missing $required_service"
  done

  echo "Optional integrations deployment contract: 28 static assertions; Compose models valid"
else
  echo "Optional integrations deployment contract: 28 static assertions passed; Compose validation skipped (Docker Compose unavailable)"
fi
