#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

DOCTOR_ONLY=0
SERVER_MODE="${AETHERLINK_SERVER_MODE:-0}"
AETHERLINK_PERFORMANCE_TIER="${AETHERLINK_PERFORMANCE_TIER:-}"
AETHERLINK_BIND_ADDRESS="${AETHERLINK_BIND_ADDRESS:-}"
while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      echo "Usage: ./deploy/init.sh [--doctor] [--server] [--no-build] [--skip-verify] [--public-url <url>] [--mqtt-address <host:port>] [--bind-address <ip>] [--performance-tier light|standard|production]"
      echo "  --doctor       Run deployment doctor only; do not start containers."
      echo "  --server       Treat this as a server/private deployment; localhost public addresses become blocking errors."
      echo "  --no-build     Start existing images without rebuilding."
      echo "  --skip-verify  Start containers without running startup health archive."
      echo "  --public-url   Browser address shown to users, for example http://1.2.3.4:8080."
      echo "  --mqtt-address Device MQTT address, for example 1.2.3.4:1883."
      echo "  --bind-address Host interface for published ports; server mode defaults a loopback value to 0.0.0.0."
      echo "  --performance-tier  Apply Compose resource presets: light, standard, or production."
      exit 0
      ;;
    --doctor)
      DOCTOR_ONLY=1
      ;;
    --server)
      SERVER_MODE=1
      AETHERLINK_SERVER_MODE=1
      export AETHERLINK_SERVER_MODE
      ;;
    --no-build)
      AETHERLINK_NO_BUILD=1
      export AETHERLINK_NO_BUILD
      ;;
    --skip-verify)
      AETHERLINK_SKIP_VERIFY=1
      export AETHERLINK_SKIP_VERIFY
      ;;
    --public-url)
      if [ "$#" -lt 2 ] || [ -z "${2:-}" ]; then
        echo "--public-url requires a value, for example http://1.2.3.4:8080" >&2
        exit 2
      fi
      AETHERLINK_PUBLIC_URL="$2"
      export AETHERLINK_PUBLIC_URL
      shift
      ;;
    --mqtt-address)
      if [ "$#" -lt 2 ] || [ -z "${2:-}" ]; then
        echo "--mqtt-address requires a value, for example 1.2.3.4:1883" >&2
        exit 2
      fi
      AETHERLINK_MQTT_ACCESS_ADDRESS="$2"
      export AETHERLINK_MQTT_ACCESS_ADDRESS
      shift
      ;;
    --bind-address)
      if [ "$#" -lt 2 ] || [ -z "${2:-}" ]; then
        echo "--bind-address requires an IP address, for example 0.0.0.0 or 192.168.1.10" >&2
        exit 2
      fi
      AETHERLINK_BIND_ADDRESS="$2"
      export AETHERLINK_BIND_ADDRESS
      shift
      ;;
    --performance-tier)
      if [ "$#" -lt 2 ] || [ -z "${2:-}" ]; then
        echo "--performance-tier requires light, standard, or production." >&2
        exit 2
      fi
      AETHERLINK_PERFORMANCE_TIER="$2"
      export AETHERLINK_PERFORMANCE_TIER
      shift
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: ./deploy/init.sh [--doctor] [--server] [--no-build] [--skip-verify] [--public-url <url>] [--mqtt-address <host:port>] [--bind-address <ip>] [--performance-tier light|standard|production]" >&2
      exit 2
      ;;
  esac
  shift
done

if [ "$SERVER_MODE" = "1" ]; then
  AETHERLINK_SERVER_MODE=1
  export AETHERLINK_SERVER_MODE
fi

generate_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 "${1:-32}" | tr '+/' '-_' | tr -d '='
    return
  fi

  LC_ALL=C tr -dc 'A-Za-z0-9_-' </dev/urandom | head -c "${1:-32}"
}

replace_env_value() {
  name="$1"
  value="$2"
  file="$3"

  if grep -q "^${name}=" "$file"; then
    tmp_file="${file}.tmp"
    sed "s|^${name}=.*|${name}=${value}|" "$file" >"$tmp_file"
    mv "$tmp_file" "$file"
  else
    printf '%s=%s\n' "$name" "$value" >>"$file"
  fi
}

read_env_value() {
  name="$1"
  if [ -f .env ]; then
    sed -n "s/^${name}=//p" .env | tail -n 1 | sed "s/^['\"]//;s/['\"]$//"
  fi
}

normalize_performance_tier() {
  tier="${1:-light}"
  tier="$(printf '%s' "$tier" | tr '[:upper:]' '[:lower:]')"
  case "$tier" in
    light|standard|production) printf '%s' "$tier" ;;
    *)
      echo "Invalid performance tier: $tier. Use light, standard, or production." >&2
      exit 2
      ;;
  esac
}

apply_performance_tier_env_file() {
  file="$1"
  tier="$(normalize_performance_tier "${2:-light}")"

  replace_env_value AETHERLINK_PERFORMANCE_TIER "$tier" "$file"
  case "$tier" in
    light)
      replace_env_value AETHERLINK_POSTGRES_CPUS "0.40" "$file"
      replace_env_value AETHERLINK_POSTGRES_MEM_LIMIT "512m" "$file"
      replace_env_value AETHERLINK_REDIS_CPUS "0.20" "$file"
      replace_env_value AETHERLINK_REDIS_MEM_LIMIT "128m" "$file"
      replace_env_value AETHERLINK_MQTT_CPUS "0.30" "$file"
      replace_env_value AETHERLINK_MQTT_MEM_LIMIT "128m" "$file"
      replace_env_value AETHERLINK_BACKEND_CPUS "0.70" "$file"
      replace_env_value AETHERLINK_BACKEND_MEM_LIMIT "768m" "$file"
      replace_env_value AETHERLINK_FRONTEND_CPUS "0.20" "$file"
      replace_env_value AETHERLINK_FRONTEND_MEM_LIMIT "128m" "$file"
      ;;
    standard)
      replace_env_value AETHERLINK_POSTGRES_CPUS "0.80" "$file"
      replace_env_value AETHERLINK_POSTGRES_MEM_LIMIT "1g" "$file"
      replace_env_value AETHERLINK_REDIS_CPUS "0.30" "$file"
      replace_env_value AETHERLINK_REDIS_MEM_LIMIT "256m" "$file"
      replace_env_value AETHERLINK_MQTT_CPUS "0.60" "$file"
      replace_env_value AETHERLINK_MQTT_MEM_LIMIT "256m" "$file"
      replace_env_value AETHERLINK_BACKEND_CPUS "1.50" "$file"
      replace_env_value AETHERLINK_BACKEND_MEM_LIMIT "1536m" "$file"
      replace_env_value AETHERLINK_FRONTEND_CPUS "0.30" "$file"
      replace_env_value AETHERLINK_FRONTEND_MEM_LIMIT "192m" "$file"
      ;;
    production)
      replace_env_value AETHERLINK_POSTGRES_CPUS "1.50" "$file"
      replace_env_value AETHERLINK_POSTGRES_MEM_LIMIT "2g" "$file"
      replace_env_value AETHERLINK_REDIS_CPUS "0.50" "$file"
      replace_env_value AETHERLINK_REDIS_MEM_LIMIT "512m" "$file"
      replace_env_value AETHERLINK_MQTT_CPUS "1.00" "$file"
      replace_env_value AETHERLINK_MQTT_MEM_LIMIT "512m" "$file"
      replace_env_value AETHERLINK_BACKEND_CPUS "2.50" "$file"
      replace_env_value AETHERLINK_BACKEND_MEM_LIMIT "3072m" "$file"
      replace_env_value AETHERLINK_FRONTEND_CPUS "0.50" "$file"
      replace_env_value AETHERLINK_FRONTEND_MEM_LIMIT "256m" "$file"
      ;;
    *)
      echo "Invalid performance tier: $tier. Use light, standard, or production." >&2
      exit 2
      ;;
  esac
}

is_interactive_terminal() {
  [ -t 0 ] && [ -t 1 ]
}

is_local_address() {
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  host="$(printf '%s' "$value" | sed -E 's#^[a-z][a-z0-9+.-]*://##; s#/.*$##; s/:[0-9]+$//')"
  case "$host" in
    localhost|127.0.0.1|::1|\[::1\]) return 0 ;;
    *) return 1 ;;
  esac
}

first_device_url() {
  base_url="${1:-http://localhost:8080}"
  printf '%s/first-device' "$(printf '%s' "$base_url" | sed 's#/*$##')"
}

prompt_server_addresses() {
  prompt="${1:-Server/private deployment needs public addresses before startup.}"
  echo "$prompt"
  printf 'Browser URL, for example http://192.168.1.10:8080: '
  IFS= read -r AETHERLINK_PUBLIC_URL
  printf 'Device MQTT address, for example 192.168.1.10:1883: '
  IFS= read -r AETHERLINK_MQTT_ACCESS_ADDRESS
  if [ -z "${AETHERLINK_PUBLIC_URL:-}" ] || [ -z "${AETHERLINK_MQTT_ACCESS_ADDRESS:-}" ]; then
    echo "Server/private deployment needs both --public-url and --mqtt-address." >&2
    exit 2
  fi
  SERVER_MODE=1
  AETHERLINK_SERVER_MODE=1
  export AETHERLINK_PUBLIC_URL AETHERLINK_MQTT_ACCESS_ADDRESS AETHERLINK_SERVER_MODE
}

resolve_first_run_addresses() {
  if [ "${AETHERLINK_PUBLIC_URL:-}" ] || [ "${AETHERLINK_MQTT_ACCESS_ADDRESS:-}" ]; then
    if [ -z "${AETHERLINK_PUBLIC_URL:-}" ] || [ -z "${AETHERLINK_MQTT_ACCESS_ADDRESS:-}" ]; then
      if is_interactive_terminal; then
        echo "Only one public address was provided. Fill the missing value before .env is updated."
        if [ -z "${AETHERLINK_PUBLIC_URL:-}" ]; then
          printf 'Browser URL, for example http://192.168.1.10:8080: '
          IFS= read -r AETHERLINK_PUBLIC_URL
          export AETHERLINK_PUBLIC_URL
        fi
        if [ -z "${AETHERLINK_MQTT_ACCESS_ADDRESS:-}" ]; then
          printf 'Device MQTT address, for example 192.168.1.10:1883: '
          IFS= read -r AETHERLINK_MQTT_ACCESS_ADDRESS
          export AETHERLINK_MQTT_ACCESS_ADDRESS
        fi
      fi
      if [ -z "${AETHERLINK_PUBLIC_URL:-}" ] || [ -z "${AETHERLINK_MQTT_ACCESS_ADDRESS:-}" ]; then
        echo "Public address updates must include both --public-url and --mqtt-address." >&2
        exit 2
      fi
    fi
    SERVER_MODE=1
    AETHERLINK_SERVER_MODE=1
    export AETHERLINK_SERVER_MODE
    return
  fi

  if [ ! -f .env ]; then
    if [ "$SERVER_MODE" = "1" ]; then
      if is_interactive_terminal; then
        prompt_server_addresses "No .env file was found. Enter the public addresses devices and browsers will use."
        return
      fi
      echo "--server needs --public-url and --mqtt-address when .env does not exist in a non-interactive shell." >&2
      exit 2
    fi

    if is_interactive_terminal; then
      echo "No .env file was found. Choose how devices will reach this install before startup."
      echo "  L = Local only: browser and first device run on this machine."
      echo "  S = Server/private deployment: browser or devices connect from another machine."
      printf 'Choose L or S [L]: '
      IFS= read -r mode
      case "$(printf '%s' "$mode" | tr '[:lower:]' '[:upper:]')" in
        S*) prompt_server_addresses "Server/private deployment selected." ;;
        *) echo "Using local-only defaults: http://localhost:8080 and localhost:1883." ;;
      esac
    fi
    return
  fi

  current_public_url="$(read_env_value AETHERLINK_PUBLIC_URL || true)"
  current_mqtt_address="$(read_env_value AETHERLINK_MQTT_ACCESS_ADDRESS || true)"
  public_is_local=0
  mqtt_is_local=0
  is_local_address "$current_public_url" || public_is_local=1
  is_local_address "$current_mqtt_address" || mqtt_is_local=1
  if [ "$public_is_local" -ne 0 ] && [ "$mqtt_is_local" -ne 0 ]; then
    return
  fi

  if [ "$SERVER_MODE" = "1" ]; then
    if is_interactive_terminal; then
      prompt_server_addresses ".env exists but its public addresses still look local-only. Enter server/private addresses."
      return
    fi
    return
  fi

  if is_interactive_terminal; then
    echo ".env already exists and its public addresses still look local-only."
    echo "  Browser URL: $current_public_url"
    echo "  Device MQTT: $current_mqtt_address"
    echo "  K = Keep local-only: browser and first device run on this machine."
    echo "  S = Switch to server/private addresses before startup."
    printf 'Choose K or S [K]: '
    IFS= read -r mode
    case "$(printf '%s' "$mode" | tr '[:lower:]' '[:upper:]')" in
      S*) prompt_server_addresses "Existing .env public addresses will be updated before startup. Secrets and volumes are kept." ;;
      *) echo "Keeping existing local-only addresses from .env." ;;
    esac
  fi
}

initialize_env_file() {
  cp .env.example .env

  postgres_password="$(generate_secret 32)"
  redis_password="$(generate_secret 32)"
  mqtt_root_password="$(generate_secret 32)"
  mqtt_plugin_password="$(generate_secret 32)"
  jwt_key="$(generate_secret 48)"

  replace_env_value POSTGRES_PASSWORD "$postgres_password" .env
  replace_env_value GOTP_DB_PSQL_PASSWORD "$postgres_password" .env
  replace_env_value REDIS_PASSWORD "$redis_password" .env
  replace_env_value GOTP_DB_REDIS_PASSWORD "$redis_password" .env
  replace_env_value MQTT_ROOT_PASSWORD "$mqtt_root_password" .env
  replace_env_value MQTT_PLUGIN_PASSWORD "$mqtt_plugin_password" .env
  replace_env_value GOTP_MQTT_USER "root" .env
  replace_env_value GOTP_MQTT_PASS "$mqtt_root_password" .env
  replace_env_value GOTP_JWT_KEY "$jwt_key" .env
  replace_env_value AETHERLINK_SERVER_MODE "$SERVER_MODE" .env
  apply_performance_tier_env_file .env "${AETHERLINK_PERFORMANCE_TIER:-light}"

  if [ "${AETHERLINK_PUBLIC_URL:-}" ]; then
    replace_env_value AETHERLINK_PUBLIC_URL "$AETHERLINK_PUBLIC_URL" .env
    replace_env_value GOTP_OTA_DOWNLOAD_ADDRESS "$AETHERLINK_PUBLIC_URL" .env
  fi

  if [ "${AETHERLINK_MQTT_ACCESS_ADDRESS:-}" ]; then
    replace_env_value AETHERLINK_MQTT_ACCESS_ADDRESS "$AETHERLINK_MQTT_ACCESS_ADDRESS" .env
    replace_env_value GOTP_MQTT_ACCESS_ADDRESS "$AETHERLINK_MQTT_ACCESS_ADDRESS" .env
  fi

  if [ "${AETHERLINK_BIND_ADDRESS:-}" ]; then
    replace_env_value AETHERLINK_BIND_ADDRESS "$AETHERLINK_BIND_ADDRESS" .env
  elif [ "$SERVER_MODE" = "1" ]; then
    replace_env_value AETHERLINK_BIND_ADDRESS "0.0.0.0" .env
  fi

  sync_auth_cookie_secure_env_file

  echo "Created .env with generated local secrets."
}

# GOTP_AUTH_COOKIE_SECURE 必须跟随公网入口协议：HTTPS 部署不允许下发非 Secure 认证 cookie。
# 仅在检测到 https:// 公网地址时强制 true；HTTP 本地/联调保留运维显式配置。
sync_auth_cookie_secure_env_file() {
  [ -f .env ] || return
  effective_public_url="${AETHERLINK_PUBLIC_URL:-$(read_env_value AETHERLINK_PUBLIC_URL || true)}"
  case "$effective_public_url" in
    https://*) replace_env_value GOTP_AUTH_COOKIE_SECURE "true" .env ;;
  esac
}

# server 模式把端口发布到 loopback 之外；当整条链路仍是明文（公网入口非 HTTPS）时
# 打印醒目告警，要求运维显式选择 TLS 终结方案。AETHERLINK_SKIP_TLS_WARNING=1 表示已知悉并静默。
warn_plaintext_server_exposure() {
  [ "$SERVER_MODE" = "1" ] || return
  [ "${AETHERLINK_SKIP_TLS_WARNING:-}" = "1" ] && return
  effective_bind_address="${AETHERLINK_BIND_ADDRESS:-$(read_env_value AETHERLINK_BIND_ADDRESS || true)}"
  effective_public_url="${AETHERLINK_PUBLIC_URL:-$(read_env_value AETHERLINK_PUBLIC_URL || true)}"
  case "$effective_bind_address" in
    ""|localhost|127.0.0.1|::1|\[::1\]) return ;;
  esac
  case "$effective_public_url" in
    https://*) return ;;
  esac
  cat >&2 <<'WARN_EOF'
====================================================================
WARNING: server mode exposes plaintext services beyond loopback.
- Web/API (8080/9999) and device MQTT (1883) currently have no TLS.
- Device vouchers and JWT tokens travel unencrypted on these ports.
Recommended hardening:
- Terminate TLS for the web entry with a reverse proxy and keep
  AETHERLINK_PUBLIC_URL on https:// (this also flips the auth cookie
  to Secure automatically on the next start).
- For MQTTS, mount real CA-signed certificates, enable the :8883
  listener in mqtt-broker/cmd/gmqttd/default_config.yml, then publish
  8883 via a Compose override. deploy/gen-mqtt-certs.* issues
  self-signed certificates for intranet/testing only.
Set AETHERLINK_SKIP_TLS_WARNING=1 to acknowledge and silence this warning.
====================================================================
WARN_EOF
}

sync_address_env_file() {
  [ -f .env ] || return

  effective_performance_tier="${AETHERLINK_PERFORMANCE_TIER:-}"
  if [ -z "$effective_performance_tier" ]; then
    effective_performance_tier="$(read_env_value AETHERLINK_PERFORMANCE_TIER || true)"
  fi
  apply_performance_tier_env_file .env "${effective_performance_tier:-light}"

  if [ "${AETHERLINK_PUBLIC_URL:-}" ]; then
    replace_env_value AETHERLINK_PUBLIC_URL "$AETHERLINK_PUBLIC_URL" .env
    replace_env_value GOTP_OTA_DOWNLOAD_ADDRESS "$AETHERLINK_PUBLIC_URL" .env
  fi

  if [ "${AETHERLINK_MQTT_ACCESS_ADDRESS:-}" ]; then
    replace_env_value AETHERLINK_MQTT_ACCESS_ADDRESS "$AETHERLINK_MQTT_ACCESS_ADDRESS" .env
    replace_env_value GOTP_MQTT_ACCESS_ADDRESS "$AETHERLINK_MQTT_ACCESS_ADDRESS" .env
  fi

  if [ "${AETHERLINK_BIND_ADDRESS:-}" ]; then
    replace_env_value AETHERLINK_BIND_ADDRESS "$AETHERLINK_BIND_ADDRESS" .env
  elif [ "$SERVER_MODE" = "1" ]; then
    current_bind_address="$(read_env_value AETHERLINK_BIND_ADDRESS || true)"
    case "$current_bind_address" in
      ""|localhost|127.0.0.1|0.0.0.0|::1|\[::1\]) replace_env_value AETHERLINK_BIND_ADDRESS "0.0.0.0" .env ;;
    esac
  fi

  if [ "$SERVER_MODE" = "1" ]; then
    replace_env_value AETHERLINK_SERVER_MODE "1" .env
  fi

  sync_auth_cookie_secure_env_file

  echo "Updated .env from explicit startup arguments."
}

resolve_first_run_addresses

if [ ! -f .env ]; then
  initialize_env_file
else
  sync_address_env_file
fi

sync_auth_cookie_secure_env_file
warn_plaintext_server_exposure

doctor_args=""
if [ "$SERVER_MODE" = "1" ]; then
  doctor_args="--server"
fi
if [ "${AETHERLINK_PERFORMANCE_TIER:-}" ]; then
  doctor_args="$doctor_args --performance-tier $(normalize_performance_tier "$AETHERLINK_PERFORMANCE_TIER")"
fi
sh "$ROOT_DIR/deploy/doctor.sh" $doctor_args

if [ "$DOCTOR_ONLY" = "1" ]; then
  echo "Doctor-only mode finished. No containers were started."
  exit 0
fi

echo "AetherLink IoT one-click startup"
echo "Frontend: ${AETHERLINK_PUBLIC_URL:-http://localhost:8080}"
echo "MQTT: ${AETHERLINK_MQTT_ACCESS_ADDRESS:-localhost:1883}"
current_performance_tier="$(read_env_value AETHERLINK_PERFORMANCE_TIER || true)"
current_public_url="$(read_env_value AETHERLINK_PUBLIC_URL || true)"
current_first_device_url="$(first_device_url "${current_public_url:-http://localhost:8080}")"
echo "Performance tier: $(normalize_performance_tier "${current_performance_tier:-light}")"
echo "Options: --server blocks localhost public addresses; --no-build or AETHERLINK_NO_BUILD=1 skips image rebuild; --skip-verify or AETHERLINK_SKIP_VERIFY=1 skips startup health archive; --doctor only runs preflight."

docker compose config >/dev/null
if [ "${AETHERLINK_NO_BUILD:-}" = "1" ]; then
  docker compose up -d
else
  docker compose up -d --build
fi
docker compose ps

if [ "${AETHERLINK_SKIP_VERIFY:-}" != "1" ]; then
  "$ROOT_DIR/deploy/verify.sh"
fi

echo
echo "AetherLink IoT is starting."
echo "Frontend: ${AETHERLINK_PUBLIC_URL:-http://localhost:8080}"
echo "MQTT: ${AETHERLINK_MQTT_ACCESS_ADDRESS:-localhost:1883}"
echo "Open: $current_first_device_url"
echo "Next: follow 接入第一台设备: check deployment health -> generate the first device -> send the first telemetry -> download the success proof."
echo "If startup is stuck, check verification/startup-*/manifest.json and rerun sh ./deploy/init.sh --doctor."
