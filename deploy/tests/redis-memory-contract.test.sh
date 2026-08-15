#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

assert_contains() {
  file="$1"
  text="$2"
  grep -F -- "$text" "$file" >/dev/null || fail "$file must contain: $text"
}

assert_rendered_contains() {
  rendered="$1"
  text="$2"
  printf '%s\n' "$rendered" | grep -F -- "$text" >/dev/null || fail "rendered Compose must contain: $text"
}

# Redis stores shared core state, so it must reject writes at the configured
# memory watermark rather than silently evict keys. The default watermark must
# also remain below the default container limit and AOF must stay enabled.
assert_contains "$COMPOSE_FILE" 'mem_limit: "${AETHERLINK_REDIS_MEM_LIMIT:-128m}"'
assert_contains "$COMPOSE_FILE" '"--appendonly", "yes"'
assert_contains "$COMPOSE_FILE" '"--maxmemory", "${AETHERLINK_REDIS_MAXMEMORY:-96mb}"'
assert_contains "$COMPOSE_FILE" '"--maxmemory-policy", "noeviction"'

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  render_compose() {
    POSTGRES_PASSWORD=contract-test \
    REDIS_PASSWORD=contract-test \
    GOTP_DB_PSQL_PASSWORD=contract-test \
    GOTP_DB_REDIS_PASSWORD=contract-test \
    MQTT_ROOT_PASSWORD=contract-test \
    MQTT_PLUGIN_PASSWORD=contract-test \
    MQTT_BROKER_ID=contract-test \
    GOTP_JWT_KEY=contract-test \
      docker compose -f "$COMPOSE_FILE" config
  }

  unset AETHERLINK_REDIS_MAXMEMORY
  default_rendered="$(render_compose)"
  AETHERLINK_REDIS_MAXMEMORY=192mb
  export AETHERLINK_REDIS_MAXMEMORY
  override_rendered="$(render_compose)"
  assert_rendered_contains "$default_rendered" '96mb'
  assert_rendered_contains "$override_rendered" '192mb'
  assert_rendered_contains "$default_rendered" 'noeviction'
  assert_rendered_contains "$default_rendered" 'appendonly'
  echo "Redis memory contract: 4 static assertions and Compose default/override validation passed"
else
  echo "Redis memory contract: 4 static assertions passed; Compose validation skipped (Docker Compose unavailable)"
fi
