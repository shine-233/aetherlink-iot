#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$ROOT_DIR"

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
AetherLink IoT one-click starter

Usage:
  sh ./start-aetherlink.sh
  sh ./start-aetherlink.sh --doctor
  sh ./start-aetherlink.sh --performance-tier light
  sh ./start-aetherlink.sh --server --public-url http://1.2.3.4:8080 --mqtt-address 1.2.3.4:1883 --bind-address 0.0.0.0

This wrapper delegates to ./deploy/init.sh so the root folder has one obvious
entrypoint for local and private deployments. Performance tiers are light,
standard, and production.
EOF
  exit 0
fi

if [ ! -f "./deploy/init.sh" ]; then
  echo "Missing ./deploy/init.sh. Run this script from a complete AetherLink IoT package." >&2
  exit 1
fi

echo "AetherLink IoT one-click starter"
echo "Project root: $ROOT_DIR"
echo "Running: sh ./deploy/init.sh $*"
echo

sh ./deploy/init.sh "$@"

read_env_value() {
  name="$1"
  if [ -f .env ]; then
    sed -n "s/^${name}=//p" .env | tail -n 1 | sed "s/^['\"]//;s/['\"]$//"
  fi
}

first_device_url() {
  base_url="${1:-http://localhost:8080}"
  printf '%s/first-device' "$(printf '%s' "$base_url" | sed 's#/*$##')"
}

PUBLIC_URL="$(read_env_value AETHERLINK_PUBLIC_URL)"
MQTT_ADDRESS="$(read_env_value AETHERLINK_MQTT_ACCESS_ADDRESS)"

if [ -z "$PUBLIC_URL" ]; then
  PUBLIC_URL="http://localhost:8080"
fi
if [ -z "$MQTT_ADDRESS" ]; then
  MQTT_ADDRESS="localhost:1883"
fi
FIRST_DEVICE_URL="$(first_device_url "$PUBLIC_URL")"

echo
echo "Done. Next:"
echo "  Open: $FIRST_DEVICE_URL"
echo "  Next: follow 接入第一台设备: check deployment health -> generate the first device -> send the first telemetry -> download the success proof."
echo "  Device MQTT address: $MQTT_ADDRESS"
echo
echo "If the first admin page is unavailable, run: sh ./deploy/first-admin.sh"
