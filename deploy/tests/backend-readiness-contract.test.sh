#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
ROUTER_FILE="$ROOT_DIR/backend/router/router_init.go"
API_FILE="$ROOT_DIR/backend/internal/api/deployment_health.go"

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

# Liveness and readiness remain separate public contracts.
assert_contains "$ROUTER_FILE" 'router.GET("/health", controllers.SystemApi.HealthCheck)'
assert_contains "$ROUTER_FILE" 'router.GET("/ready", controllers.SystemApi.Readiness)'
assert_contains "$API_FILE" 'service.RunDeploymentHealthCheck()'
assert_contains "$API_FILE" 'http.StatusServiceUnavailable'

# Compose must wait for required core dependencies instead of process liveness.
assert_contains "$COMPOSE_FILE" 'http://127.0.0.1:9999/ready'
assert_not_contains "$COMPOSE_FILE" 'http://127.0.0.1:9999/health >/dev/null'
assert_contains "$COMPOSE_FILE" 'mqtt-broker:'
assert_contains "$COMPOSE_FILE" 'condition: service_healthy'
assert_contains "$COMPOSE_FILE" 'AETHERLINK_SERVER_MODE: ${AETHERLINK_SERVER_MODE:-0}'

# The broker healthcheck must be the metrics probe used by the core service.
assert_contains "$COMPOSE_FILE" 'http://127.0.0.1:8082/metrics'

echo "Backend readiness contract: 10 assertions passed"
