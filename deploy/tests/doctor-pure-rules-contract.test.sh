#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
FIXTURE_DIR="$SCRIPT_DIR/fixtures"
SOURCE_DIR="$(pwd)"

export AETHERLINK_DOCTOR_LIBRARY_ONLY=1
. "$ROOT_DIR/deploy/doctor.sh"
unset AETHERLINK_DOCTOR_LIBRARY_ONLY
cd "$SOURCE_DIR"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

mqtt_count=0
while IFS="$(printf '\t')" read -r name endpoint expected_valid expected_host expected_port expected_local; do
  case "$name" in ''|'#'*) continue ;; esac
  mqtt_count=$((mqtt_count + 1))
  MQTT_ENDPOINT_HOST="polluted-host"
  MQTT_ENDPOINT_PORT="polluted-port"
  actual_valid=0
  if parse_mqtt_endpoint "$endpoint"; then actual_valid=1; fi
  [ "$actual_valid" = "$expected_valid" ] || fail "$name validity expected $expected_valid, got $actual_valid"
  if [ "$expected_valid" = "1" ]; then
    [ "$MQTT_ENDPOINT_HOST" = "$expected_host" ] || fail "$name host expected $expected_host, got $MQTT_ENDPOINT_HOST"
    [ "$MQTT_ENDPOINT_PORT" = "$expected_port" ] || fail "$name port expected $expected_port, got $MQTT_ENDPOINT_PORT"
    actual_local=0
    if is_local_host_value "$MQTT_ENDPOINT_HOST"; then actual_local=1; fi
    [ "$actual_local" = "$expected_local" ] || fail "$name local expected $expected_local, got $actual_local"
  fi
done < "$FIXTURE_DIR/doctor-mqtt-endpoints.tsv"

performance_count=0
while IFS="$(printf '\t')" read -r name input expected_normalized expected_valid; do
  case "$name" in ''|'#'*) continue ;; esac
  performance_count=$((performance_count + 1))
  [ "$input" = "<empty>" ] && input=""
  actual_normalized="$(normalize_performance_tier "$input")"
  [ "$expected_normalized" = "<empty>" ] && expected_normalized=""
  [ "$actual_normalized" = "$expected_normalized" ] || fail "$name tier expected $expected_normalized, got $actual_normalized"
  actual_valid=0
  case "$actual_normalized" in light|standard|production) actual_valid=1 ;; esac
  [ "$actual_valid" = "$expected_valid" ] || fail "$name tier validity expected $expected_valid, got $actual_valid"
done < "$FIXTURE_DIR/doctor-performance-tiers.tsv"

port_count=0
while IFS="$(printf '\t')" read -r name input expected_valid expected_port; do
  case "$name" in ''|'#'*) continue ;; esac
  port_count=$((port_count + 1))
  [ "$input" = "<empty>" ] && input=""
  actual_valid=0
  MQTT_ENDPOINT_HOST="polluted-host"
  MQTT_ENDPOINT_PORT="polluted-port"
  if parse_mqtt_endpoint "broker.example:$input"; then actual_valid=1; fi
  [ "$actual_valid" = "$expected_valid" ] || fail "$name port validity expected $expected_valid, got $actual_valid"
  if [ "$expected_valid" = "1" ]; then
    [ "$MQTT_ENDPOINT_PORT" = "$expected_port" ] || fail "$name port expected $expected_port, got $MQTT_ENDPOINT_PORT"
  fi
done < "$FIXTURE_DIR/doctor-tcp-ports.tsv"

server_address_count=0
while IFS="$(printf '\t')" read -r name value expected; do
  case "$name" in ''|'#'*) continue ;; esac
  server_address_count=$((server_address_count + 1))
  actual=0
  if is_server_address "$value"; then actual=1; fi
  [ "$actual" = "$expected" ] || fail "$name server address expected $expected, got $actual"
done <<'EOF'
server-local-url	http://127.0.0.1:8080	0
server-unspecified-ip	http://0.0.0.0:8080	0
server-placeholder-url	http://YOUR-IP:8080	0
server-local-mqtt	localhost:1883	0
server-placeholder-mqtt	example.com:1883	0
server-real-url	https://console.example-customer.invalid:8443	1
server-real-mqtt	broker.example-customer.invalid:1883	1
EOF

total=$((mqtt_count + performance_count + port_count + server_address_count))
echo "POSIX doctor pure rules contract: $total passed (MQTT $mqtt_count, performance $performance_count, port $port_count, server-address $server_address_count)"
