#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
ENV_EXAMPLE="$ROOT_DIR/.env.example"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

assert_contains() {
  file="$1"
  text="$2"
  grep -F -- "$text" "$file" >/dev/null || fail "$file must contain: $text"
}

for mapping in \
  '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${MQTT_PORT:-1883}:1883' \
  '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${BROKER_METRICS_PORT:-8082}:8082' \
  '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${BACKEND_PORT:-9999}:9999' \
  '${AETHERLINK_BIND_ADDRESS:-127.0.0.1}:${FRONTEND_PORT:-8080}:8080'
do
  assert_contains "$COMPOSE_FILE" "$mapping"
done

assert_contains "$ENV_EXAMPLE" 'AETHERLINK_BIND_ADDRESS=127.0.0.1'
assert_contains "$ROOT_DIR/deploy/init.sh" 'replace_env_value AETHERLINK_BIND_ADDRESS "0.0.0.0" .env'
assert_contains "$ROOT_DIR/deploy/init.ps1" 'Set-AetherLinkEnvValue $content "AETHERLINK_BIND_ADDRESS" "0.0.0.0"'

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  rendered="$({
    POSTGRES_PASSWORD=contract-test \
    REDIS_PASSWORD=contract-test \
    GOTP_DB_PSQL_PASSWORD=contract-test \
    GOTP_DB_REDIS_PASSWORD=contract-test \
    MQTT_ROOT_PASSWORD=contract-test \
    MQTT_PLUGIN_PASSWORD=contract-test \
    MQTT_BROKER_ID=contract-test \
    GOTP_JWT_KEY=contract-test \
      docker compose -f "$COMPOSE_FILE" config
  })"
  for published in \
    'host_ip: 127.0.0.1' \
    'published: "1883"' \
    'published: "8082"' \
    'published: "9999"' \
    'published: "8080"'
  do
    printf '%s\n' "$rendered" | grep -F -- "$published" >/dev/null || fail "rendered Compose must contain: $published"
  done
  echo "Host port binding contract: static and Compose validation passed"
else
  echo "Host port binding contract: 7 static assertions passed; Compose validation skipped"
fi
