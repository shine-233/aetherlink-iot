#!/usr/bin/env sh
set -eu

ROOT_DIR="${AETHERLINK_VERIFY_ROOT:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)}"
cd "$ROOT_DIR"

SERVER_MODE="${AETHERLINK_SERVER_MODE:-0}"

TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-180}"
INTERVAL_SECONDS="${INTERVAL_SECONDS:-5}"
HTTP_CONNECT_TIMEOUT_SECONDS="${HTTP_CONNECT_TIMEOUT_SECONDS:-5}"
HTTP_REQUEST_TIMEOUT_SECONDS="${HTTP_REQUEST_TIMEOUT_SECONDS:-10}"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
ARCHIVE_ROOT="${ARCHIVE_ROOT:-verification/startup-${TIMESTAMP}}"
RAW_ROOT="${ARCHIVE_ROOT}/raw"
STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

read_env_value() {
  name="$1"
  if [ -f .env ]; then
    sed -n "s/^${name}=//p" .env | tail -n 1 | sed "s/^['\"]//;s/['\"]$//"
  fi
}

join_url() {
  base="$1"
  path="$2"
  printf '%s/%s' "$(printf '%s' "$base" | sed 's|/*$||')" "$(printf '%s' "$path" | sed 's|^/*||')"
}

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

address_host() {
  value="$1"
  case "$value" in
    http://*|https://*)
      value="${value#*://}"
      value="${value%%/*}"
      value="${value%%\?*}"
      ;;
    \[*\]:*) value="${value#\[}"; value="${value%%\]*}" ;;
    *:*) value="${value%:*}" ;;
  esac
  printf '%s' "$value" | tr '[:upper:]' '[:lower:]'
}

is_local_host_value() {
  case "$(printf '%s' "$1" | sed 's/\.$//' | tr '[:upper:]' '[:lower:]')" in
    localhost|127.0.0.1|0.0.0.0|::|::1) return 0 ;;
    *) return 1 ;;
  esac
}

is_placeholder_host_value() {
  case "$(printf '%s' "$1" | sed 's/\.$//' | tr '[:upper:]' '[:lower:]')" in
    ""|example.com|example.net|example.org|your-ip|your_ip|your-domain|your_domain|change-me|change_me|placeholder|todo) return 0 ;;
    *) return 1 ;;
  esac
}

is_server_address() {
  host="$(address_host "$1")"
  [ -n "$host" ] || return 1
  is_local_host_value "$host" && return 1
  is_placeholder_host_value "$host" && return 1
  return 0
}

deployment_failed_checks_json() {
  body_file="$1"
  if [ ! -s "$body_file" ]; then
    printf '["health-payload-missing"]'
    return
  fi

  python_bin=""
  if command -v python3 >/dev/null 2>&1; then
    python_bin="python3"
  elif command -v python >/dev/null 2>&1; then
    python_bin="python"
  fi
  if [ -z "$python_bin" ]; then
    printf '["health-payload-parser-unavailable"]'
    return
  fi

  "$python_bin" - "$body_file" <<'PY'
import json
import sys

path = sys.argv[1]
try:
    with open(path, "r", encoding="utf-8") as fh:
        report = json.load(fh)
except Exception:
    print('["health-payload-invalid-json"]')
    raise SystemExit(0)

if not isinstance(report, dict):
    print('["health-payload-contract"]')
    raise SystemExit(0)

failed = []
legacy_fields = ("frontend_proxy", "api")
has_legacy_contract = any(field in report for field in legacy_fields)
has_supported_contract = has_legacy_contract
if has_legacy_contract:
    for field in legacy_fields:
        value = report.get(field)
        if not isinstance(value, dict) or value.get("ok") is not True:
            failed.append(field)

if "checks" in report:
    has_supported_contract = True
    checks = report.get("checks")
    if not isinstance(checks, dict) or not checks:
        failed.append("checks")
    else:
        for name, value in checks.items():
            if not isinstance(value, dict) or value.get("ok") is not True:
                failed.append(name)

if not has_supported_contract:
    failed.append("health-payload-contract")

print(json.dumps(list(dict.fromkeys(failed)), ensure_ascii=False, separators=(",", ":")))
PY
}

deployment_health_body_ok() {
  [ "$(deployment_failed_checks_json "$1")" = "[]" ]
}

check_manifest_json() {
  name="$1"
  url="$2"
  out_file="${RAW_ROOT}/${name}.out.txt"
  err_file="${RAW_ROOT}/${name}.err.txt"
  status_file="${RAW_ROOT}/${name}.status.txt"
  attempts_file="${RAW_ROOT}/${name}.attempts.txt"
  status_code="$(cat "$status_file" 2>/dev/null || printf '000')"
  status_number="$(printf '%s' "$status_code" | sed 's/^0*//')"
  attempts="$(cat "$attempts_file" 2>/dev/null || printf '0')"
  ok=false
  failed_checks="[]"

  if [ -z "$status_number" ]; then
    status_number=0
  fi
  if [ "$name" = "deployment-health" ]; then
    failed_checks="$(deployment_failed_checks_json "$out_file")"
  fi
  if [ "$status_code" = "200" ] && { [ "$name" != "deployment-health" ] || [ "$failed_checks" = "[]" ]; }; then
    ok=true
  fi

  cat <<EOF
    {
      "name": "$(json_escape "$name")",
      "url": "$(json_escape "$url")",
      "ok": $ok,
      "attempts": $attempts,
      "final": {
        "status_code": $status_number,
        "failed_checks": $failed_checks,
        "stdout": "$(json_escape "$out_file")",
        "stderr": "$(json_escape "$err_file")"
      }
    }
EOF
}

TARGET_URL="${TARGET_URL:-${AETHERLINK_PUBLIC_URL:-$(read_env_value AETHERLINK_PUBLIC_URL)}}"
FRONTEND_PORT="${FRONTEND_PORT:-$(read_env_value FRONTEND_PORT)}"
BACKEND_PORT="${BACKEND_PORT:-$(read_env_value BACKEND_PORT)}"
BROKER_METRICS_PORT="${BROKER_METRICS_PORT:-$(read_env_value BROKER_METRICS_PORT)}"
MQTT_ACCESS_ADDRESS="${MQTT_ACCESS_ADDRESS:-${AETHERLINK_MQTT_ACCESS_ADDRESS:-$(read_env_value AETHERLINK_MQTT_ACCESS_ADDRESS)}}"

TARGET_URL="${TARGET_URL:-http://localhost:${FRONTEND_PORT:-8080}}"
BACKEND_URL="${BACKEND_URL:-http://localhost:${BACKEND_PORT:-9999}}"
BROKER_METRICS_URL="${BROKER_METRICS_URL:-http://localhost:${BROKER_METRICS_PORT:-8082}/metrics}"
MQTT_ACCESS_ADDRESS="${MQTT_ACCESS_ADDRESS:-localhost:1883}"
FIRST_DEVICE_URL="$(join_url "$TARGET_URL" /first-device)"
FRONTEND_ROOT_URL="$TARGET_URL"
BACKEND_HEALTH_URL="$(join_url "$BACKEND_URL" /health)"
DEPLOYMENT_HEALTH_URL="$(join_url "$BACKEND_URL" /api/v1/deployment/health)"

if [ "$SERVER_MODE" = "1" ]; then
  if ! is_server_address "$TARGET_URL" || ! is_server_address "$MQTT_ACCESS_ADDRESS"; then
    echo "Server verification requires a non-local, non-placeholder AETHERLINK_PUBLIC_URL and AETHERLINK_MQTT_ACCESS_ADDRESS." >&2
    echo "PublicUrl=$TARGET_URL" >&2
    echo "MqttAddress=$MQTT_ACCESS_ADDRESS" >&2
    exit 2
  fi
fi

run_http_check() {
  name="$1"
  url="$2"
  out_file="${RAW_ROOT}/${name}.out.txt"
  err_file="${RAW_ROOT}/${name}.err.txt"
  status_file="${RAW_ROOT}/${name}.status.txt"

  if command -v curl >/dev/null 2>&1; then
    status_code="$(curl -L -sS \
      --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
      --max-time "$HTTP_REQUEST_TIMEOUT_SECONDS" \
      -o "$out_file" -w '%{http_code}' "$url" 2>"$err_file" || true)"
  else
    echo "curl was not found" >"$err_file"
    : >"$out_file"
    status_code="000"
  fi

  printf '%s' "$status_code" >"$status_file"
  [ "$status_code" = "200" ]
}

wait_http_check() {
  name="$1"
  url="$2"
  deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
  attempts=0

  while [ "$(date +%s)" -le "$deadline" ]; do
    attempts=$((attempts + 1))
    if run_http_check "$name" "$url"; then
      if [ "$name" != "deployment-health" ] || deployment_health_body_ok "${RAW_ROOT}/${name}.out.txt"; then
        printf '%s' "$attempts" >"${RAW_ROOT}/${name}.attempts.txt"
        return 0
      fi
    fi
    sleep "$INTERVAL_SECONDS"
  done

  printf '%s' "$attempts" >"${RAW_ROOT}/${name}.attempts.txt"
  return 1
}

if [ "${AETHERLINK_VERIFY_LIBRARY_ONLY:-}" = "1" ]; then
  return 0 2>/dev/null || exit 0
fi

mkdir -p "$RAW_ROOT"

docker_compose_exit=0
if command -v docker >/dev/null 2>&1; then
  docker compose ps >"${RAW_ROOT}/docker-compose-ps.out.txt" 2>"${RAW_ROOT}/docker-compose-ps.err.txt" || docker_compose_exit=$?
else
  echo "docker was not found" >"${RAW_ROOT}/docker-compose-ps.err.txt"
  : >"${RAW_ROOT}/docker-compose-ps.out.txt"
  docker_compose_exit=1
fi

overall_ok=1
wait_http_check frontend-root "$FRONTEND_ROOT_URL" || overall_ok=0
wait_http_check backend-health "$BACKEND_HEALTH_URL" || overall_ok=0
wait_http_check deployment-health "$DEPLOYMENT_HEALTH_URL" || overall_ok=0
wait_http_check broker-metrics "$BROKER_METRICS_URL" || overall_ok=0

if [ "$docker_compose_exit" -ne 0 ]; then
  overall_ok=0
fi

cat >"${ARCHIVE_ROOT}/manifest.json" <<EOF
{
  "kind": "startup-verification",
  "started_at": "$STARTED_AT",
  "target_url": "$(json_escape "$TARGET_URL")",
  "first_device_url": "$(json_escape "$FIRST_DEVICE_URL")",
  "backend_url": "$(json_escape "$BACKEND_URL")",
  "broker_metrics_url": "$(json_escape "$BROKER_METRICS_URL")",
  "mqtt_access_address": "$(json_escape "$MQTT_ACCESS_ADDRESS")",
  "first_use_next_steps": [
    "Open: $(json_escape "$FIRST_DEVICE_URL")",
    "Create the super admin and tenant admin if first-run setup is still pending.",
    "Follow 接入第一台设备: check deployment health, generate the first device, send the first telemetry, then download the success proof.",
    "Use device MQTT address: $(json_escape "$MQTT_ACCESS_ADDRESS")"
  ],
  "timeout_seconds": $TIMEOUT_SECONDS,
  "interval_seconds": $INTERVAL_SECONDS,
  "docker_compose_ps": {
    "exit_code": $docker_compose_exit,
    "stdout": "$(json_escape "${RAW_ROOT}/docker-compose-ps.out.txt")",
    "stderr": "$(json_escape "${RAW_ROOT}/docker-compose-ps.err.txt")"
  },
  "checks": [
$(check_manifest_json frontend-root "$FRONTEND_ROOT_URL"),
$(check_manifest_json backend-health "$BACKEND_HEALTH_URL"),
$(check_manifest_json deployment-health "$DEPLOYMENT_HEALTH_URL"),
$(check_manifest_json broker-metrics "$BROKER_METRICS_URL")
  ],
  "ok": $([ "$overall_ok" -eq 1 ] && echo true || echo false),
  "finished_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

print_failed_check() {
  name="$1"
  url="$2"
  status_file="${RAW_ROOT}/${name}.status.txt"
  status_code="$(cat "$status_file" 2>/dev/null || printf '000')"

  if [ "$status_code" != "200" ]; then
    printf -- '- %s: status=%s url=%s\n' "$name" "$status_code" "$url"
  elif [ "$name" = "deployment-health" ]; then
    failed_checks="$(deployment_failed_checks_json "${RAW_ROOT}/${name}.out.txt")"
    if [ "$failed_checks" != "[]" ]; then
      printf -- '- %s: status=%s url=%s failed_checks=%s\n' "$name" "$status_code" "$url" "$failed_checks"
    fi
  fi
}

print_startup_troubleshooting() {
  echo "Startup verification failed. Archive: ${ARCHIVE_ROOT}"
  echo
  echo "Failed checks:"
  if [ "$docker_compose_exit" -ne 0 ]; then
    echo "- docker compose ps failed with exit code ${docker_compose_exit}"
  fi
  print_failed_check frontend-root "$FRONTEND_ROOT_URL"
  print_failed_check backend-health "$BACKEND_HEALTH_URL"
  print_failed_check deployment-health "$DEPLOYMENT_HEALTH_URL"
  print_failed_check broker-metrics "$BROKER_METRICS_URL"
  echo
  echo "Next commands:"
  echo "  docker compose ps"
  echo "  docker compose logs frontend --tail=80"
  echo "  docker compose logs backend --tail=80"
  echo "  docker compose logs mqtt-broker --tail=80"
  echo "  sh ./deploy/init.sh --doctor"
}

if [ "$overall_ok" -ne 1 ]; then
  print_startup_troubleshooting
  exit 1
fi

echo "Startup verification passed. Archive: ${ARCHIVE_ROOT}"
