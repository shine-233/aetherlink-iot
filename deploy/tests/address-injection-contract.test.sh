#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

assert_contains() {
  file="$1"
  text="$2"
  grep -F -- "$text" "$file" >/dev/null || fail "$file must contain: $text"
}

# A deployment-facing AETHERLINK_* value must win over stale public GOTP_*
# values from an existing local .env; both still retain localhost defaults.
assert_contains "$ROOT_DIR/docker-compose.yml" 'GOTP_MQTT_ACCESS_ADDRESS: ${AETHERLINK_MQTT_ACCESS_ADDRESS:-${GOTP_MQTT_ACCESS_ADDRESS:-localhost:1883}}'
assert_contains "$ROOT_DIR/docker-compose.yml" 'GOTP_OTA_DOWNLOAD_ADDRESS: ${AETHERLINK_PUBLIC_URL:-${GOTP_OTA_DOWNLOAD_ADDRESS:-http://localhost:8080}}'
assert_contains "$ROOT_DIR/docker-compose.yml" 'AETHERLINK_SERVER_MODE: ${AETHERLINK_SERVER_MODE:-0}'

# Both verification entry points must use the same deployment-facing env
# values before falling back to .env and then local defaults.
assert_contains "$ROOT_DIR/deploy/verify.sh" 'TARGET_URL="${TARGET_URL:-${AETHERLINK_PUBLIC_URL:-$(read_env_value AETHERLINK_PUBLIC_URL)}}"'
assert_contains "$ROOT_DIR/deploy/verify.sh" 'MQTT_ACCESS_ADDRESS="${MQTT_ACCESS_ADDRESS:-${AETHERLINK_MQTT_ACCESS_ADDRESS:-$(read_env_value AETHERLINK_MQTT_ACCESS_ADDRESS)}}"'
assert_contains "$ROOT_DIR/deploy/verify.ps1" '$TargetUrl = $env:AETHERLINK_PUBLIC_URL'
assert_contains "$ROOT_DIR/deploy/verify.ps1" '$mqttAccessAddress = $env:AETHERLINK_MQTT_ACCESS_ADDRESS'

# Existing init/doctor contracts remain the source of server-mode rejection
# and generated GOTP pair synchronization.
assert_contains "$ROOT_DIR/deploy/init.sh" 'replace_env_value GOTP_OTA_DOWNLOAD_ADDRESS "$AETHERLINK_PUBLIC_URL" .env'
assert_contains "$ROOT_DIR/deploy/init.sh" 'replace_env_value GOTP_MQTT_ACCESS_ADDRESS "$AETHERLINK_MQTT_ACCESS_ADDRESS" .env'
assert_contains "$ROOT_DIR/deploy/init.ps1" 'Set-AetherLinkEnvValue $content "GOTP_OTA_DOWNLOAD_ADDRESS" $PublicUrl'
assert_contains "$ROOT_DIR/deploy/init.ps1" 'Set-AetherLinkEnvValue $content "GOTP_MQTT_ACCESS_ADDRESS" $MqttAddress'
assert_contains "$ROOT_DIR/deploy/init.ps1" '$doctorParams = @{}'
assert_contains "$ROOT_DIR/deploy/init.ps1" '$doctorParams["PublicUrl"] = $PublicUrl'
assert_contains "$ROOT_DIR/deploy/init.ps1" '$doctorParams["MqttAddress"] = $MqttAddress'
assert_contains "$ROOT_DIR/deploy/init.ps1" '& (Join-Path $PSScriptRoot "doctor.ps1") @doctorParams'
assert_contains "$ROOT_DIR/deploy/start-windows.ps1" '$initParams = @{}'
assert_contains "$ROOT_DIR/deploy/start-windows.ps1" '$initParams["PublicUrl"] = $PublicUrl'
assert_contains "$ROOT_DIR/deploy/start-windows.ps1" '& (Join-Path $PSScriptRoot "init.ps1") @initParams'
assert_contains "$ROOT_DIR/start-aetherlink.ps1" '$starterParams = @{}'
assert_contains "$ROOT_DIR/start-aetherlink.ps1" '$starterParams["PublicUrl"] = $PublicUrl'
assert_contains "$ROOT_DIR/start-aetherlink.ps1" '& $starter @starterParams'
assert_contains "$ROOT_DIR/deploy/doctor.sh" 'server-public-url-not-local'
assert_contains "$ROOT_DIR/deploy/doctor.ps1" 'server-public-url-not-local'
assert_contains "$ROOT_DIR/.env.example" 'AETHERLINK_SERVER_MODE=0'
assert_contains "$ROOT_DIR/deploy/init.sh" 'replace_env_value AETHERLINK_SERVER_MODE "$SERVER_MODE" .env'
assert_contains "$ROOT_DIR/deploy/init.ps1" 'Set-AetherLinkEnvValue $content "AETHERLINK_SERVER_MODE"'

echo "Address injection deployment contract: 26 assertions passed"
