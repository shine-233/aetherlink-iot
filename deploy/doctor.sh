#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

ERROR_COUNT=0
WARNING_COUNT=0
ENV_ISSUE_COUNT=0
SERVER_MODE="${AETHERLINK_SERVER_MODE:-0}"
PERFORMANCE_TIER="${AETHERLINK_PERFORMANCE_TIER:-}"
LIVE_DB="${AETHERLINK_DOCTOR_LIVE_DB:-0}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      echo "Usage: ./deploy/doctor.sh [--server] [--live-db] [--public-url <url>] [--mqtt-address <host:port>] [--performance-tier light|standard|production]"
      echo "Checks Docker, Compose, .env, secrets, ports, required files, disk, memory, and compose config without starting containers."
      echo "  --server  Treat localhost public browser/MQTT addresses as blocking errors."
      echo "  --live-db  Also probe the configured PostgreSQL endpoint with pg_isready/psql or a TCP fallback."
      echo "  --performance-tier  Validate Compose resource preset: light, standard, or production."
      exit 0
      ;;
    --server)
      SERVER_MODE=1
      AETHERLINK_SERVER_MODE=1
      export AETHERLINK_SERVER_MODE
      ;;
    --live-db)
      LIVE_DB=1
      AETHERLINK_DOCTOR_LIVE_DB=1
      export AETHERLINK_DOCTOR_LIVE_DB
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
    --performance-tier)
      if [ "$#" -lt 2 ] || [ -z "${2:-}" ]; then
        echo "--performance-tier requires light, standard, or production." >&2
        exit 2
      fi
      PERFORMANCE_TIER="$2"
      export PERFORMANCE_TIER
      shift
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: ./deploy/doctor.sh [--server] [--live-db] [--public-url <url>] [--mqtt-address <host:port>] [--performance-tier light|standard|production]" >&2
      exit 2
      ;;
  esac
  shift
done

read_env_value() {
  name="$1"
  if [ -f .env ]; then
    sed -n "s/^${name}=//p" .env | tail -n 1 | sed "s/^['\"]//;s/['\"]$//"
  fi
}

check_env_syntax() {
  if [ ! -f .env ]; then
    return
  fi

  awk '
    BEGIN { issues = 0 }
    /^[[:space:]]*($|#)/ { next }
    index($0, "=") == 0 {
      printf("Line %d is ignored because it does not contain =.\n", NR)
      issues++
      next
    }
    {
      split($0, parts, "=")
      raw_key = parts[1]
      key = raw_key
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
      value = substr($0, index($0, "=") + 1)
      trimmed_value = value
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", trimmed_value)
      if (key == "") {
        printf("Line %d has an empty key.\n", NR)
        issues++
        next
      }
      if (key ~ /^export[[:space:]]+/) {
        sub(/^export[[:space:]]+/, "", key)
        printf("Line %d uses export; .env entries should be plain KEY=value.\n", NR)
        issues++
      }
      if (key !~ /^[A-Za-z_][A-Za-z0-9_]*$/) {
        printf("Line %d key %s is not a valid variable name.\n", NR, key)
        issues++
      }
      if (raw_key != key) {
        printf("Line %d key %s has surrounding whitespace.\n", NR, key)
        issues++
      }
      first = substr(trimmed_value, 1, 1)
      last = substr(trimmed_value, length(trimmed_value), 1)
      if ((first == "\"" && last != "\"") || (first == "'"'"'" && last != "'"'"'")) {
        printf("Line %d value for %s has an unmatched quote.\n", NR, key)
        issues++
      }
      seen[key]++
      if (seen[key] > 1) {
        printf("Line %d duplicates key %s; the last value wins.\n", NR, key)
        issues++
      }
    }
    END { exit issues > 0 ? 1 : 0 }
  ' .env > "${TMPDIR:-/tmp}/aetherlink-env-issues.$$" || ENV_ISSUE_COUNT=1

  if [ "$ENV_ISSUE_COUNT" -eq 0 ]; then
    add_check error 1 env-syntax "Parsed .env with 0 syntax issues."
  else
    add_check error 0 env-syntax "Parsed .env with syntax issues." "Fix malformed, duplicate, or whitespace-padded keys in .env."
    while IFS= read -r issue; do
      [ "$issue" ] && add_check warning 0 env-syntax-detail "$issue" "Keep one KEY=value entry per line."
    done < "${TMPDIR:-/tmp}/aetherlink-env-issues.$$"
  fi
  rm -f "${TMPDIR:-/tmp}/aetherlink-env-issues.$$"
}

add_check() {
  level="$1"
  ok="$2"
  name="$3"
  message="$4"
  fix="${5:-}"

  if [ "$ok" = "1" ]; then
    printf '[OK] %s: %s\n' "$name" "$message"
    return
  fi

  if [ "$level" = "warning" ]; then
    WARNING_COUNT=$((WARNING_COUNT + 1))
    printf '[WARN] %s: %s\n' "$name" "$message"
  else
    ERROR_COUNT=$((ERROR_COUNT + 1))
    printf '[ERROR] %s: %s\n' "$name" "$message"
  fi

  if [ "$fix" ]; then
    printf '      Fix: %s\n' "$fix"
  fi
}

env_key_list() {
  file="$1"
  [ -f "$file" ] || return 0
  awk '
    /^[[:space:]]*($|#)/ { next }
    index($0, "=") == 0 { next }
    {
      key = substr($0, 1, index($0, "=") - 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
      sub(/^export[[:space:]]+/, "", key)
      if (key != "") print key
    }
  ' "$file" | sort -u
}

normalize_performance_tier() {
  tier="${1:-light}"
  printf '%s' "$tier" | awk '{ gsub(/^[[:space:]]+|[[:space:]]+$/, ""); print tolower($0) }'
}

check_env_example_keys() {
  [ -f .env ] || return
  [ -f .env.example ] || return

  env_keys="${TMPDIR:-/tmp}/aetherlink-env-keys.$$"
  example_keys="${TMPDIR:-/tmp}/aetherlink-example-keys.$$"
  env_key_list .env > "$env_keys"
  env_key_list .env.example > "$example_keys"

  missing="$(comm -23 "$example_keys" "$env_keys" | paste -sd ', ' -)"
  extra="$(comm -13 "$example_keys" "$env_keys" | paste -sd ', ' -)"
  if [ "$missing" ]; then
    add_check error 0 env-example-required-keys ".env is missing keys from .env.example: $missing." "Add the missing KEY=value entries."
  else
    add_check error 1 env-example-required-keys ".env contains all keys from .env.example."
  fi
  if [ "$extra" ]; then
    add_check warning 0 env-extra-keys ".env has keys not present in .env.example: $extra." "Check for typos or document intentional custom keys."
  else
    add_check warning 1 env-extra-keys ".env has no extra keys outside .env.example."
  fi

  rm -f "$env_keys" "$example_keys"
}

env_or_default() {
  name="$1"
  default="$2"
  value="$(read_env_value "$name" || true)"
  if [ "$value" ]; then
    printf '%s' "$value"
  else
    printf '%s' "$default"
  fi
}

is_placeholder() {
  value="$1"
  lower="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$lower" in
    ""|change_me*) return 0 ;;
    *) return 1 ;;
  esac
}

check_port_available() {
  port="$1"
  python_bin=""
  if command -v python3 >/dev/null 2>&1; then
    python_bin="python3"
  elif command -v python >/dev/null 2>&1; then
    python_bin="python"
  fi

  if [ -z "$python_bin" ]; then
    return 2
  fi

  "$python_bin" - "$port" <<'PY'
import socket
import sys

port = int(sys.argv[1])
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    sock.bind(("127.0.0.1", port))
except OSError:
    raise SystemExit(1)
finally:
    sock.close()
PY
}

check_port() {
  name="$1"
  port="$2"

  case "$port" in
    ''|*[!0-9]*)
      add_check error 0 "$name" "$name must be a numeric TCP port; current value: $port." "Edit .env and set a valid $name."
      return
      ;;
  esac

  if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
    add_check error 0 "$name" "$name must be between 1 and 65535; current value: $port." "Edit .env and set a valid $name."
    return
  fi

  if check_port_available "$port"; then
    add_check error 1 "$name" "$name=$port is available on localhost."
  else
    rc=$?
    if [ "$rc" -eq 2 ]; then
      add_check warning 0 "$name" "$name=$port could not be checked because Python is unavailable." "Install python3 for local port preflight, or verify the port manually."
    else
      add_check warning 0 "$name" "$name=$port is already in use on localhost." "If this is not an existing AetherLink container, stop the conflicting service or change $name in .env."
    fi
  fi
}

check_port_duplicates() {
  duplicates="$(
    printf '%s\n' "$@" |
      awk -F= '{ ports[$2] = ports[$2] ? ports[$2] ", " $1 : $1; counts[$2]++ } END { for (port in counts) if (counts[port] > 1) print port ": " ports[port] }'
  )"
  if [ "$duplicates" ]; then
    add_check error 0 env-port-duplicates "Internal port duplicates: $duplicates." "Give each exposed service a unique port in .env."
  else
    add_check error 1 env-port-duplicates "Internal port duplicates: none."
  fi
}

check_secret_length() {
  name="$1"
  value="$2"
  minimum="$3"
  length=${#value}
  if [ "$length" -ge "$minimum" ]; then
    add_check warning 1 "env-$name-length" "$name length is $length character(s)."
  else
    add_check warning 0 "env-$name-length" "$name length is $length character(s)." "Use at least $minimum random characters."
  fi
}

check_weak_secret() {
  name="$1"
  value="$2"
  lower="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  weak=0
  case "$lower" in
    ""|password|postgres|redis|admin|root|123456|aetherlink)
      weak=1
      ;;
  esac

  if [ "$weak" -eq 0 ] && printf '%s\n' "$lower" | awk '
    length($0) < 6 { exit 1 }
    {
      first = substr($0, 1, 1)
      for (i = 2; i <= length($0); i++) {
        if (substr($0, i, 1) != first) exit 1
      }
    }
  '; then
    weak=1
  fi

  if [ "$weak" -eq 1 ]; then
    add_check warning 0 "env-$name-weak-value" "$name looks like a weak/default value." "Use a unique random value, not password/postgres/redis/admin/root/123456 or a repeated character."
  else
    add_check warning 1 "env-$name-weak-value" "$name weak/default value check passed."
  fi
}

parse_mqtt_endpoint() {
  value="${1:-}"
  parsed="$(
    printf '%s\n' "$value" | awk '
      function valid_ipv4(value, parts, count, i, octet) {
        count = split(value, parts, ".")
        if (count != 4) return 0
        for (i = 1; i <= count; i++) {
          if (parts[i] !~ /^[0-9]+$/ || length(parts[i]) > 3 || (length(parts[i]) > 1 && substr(parts[i], 1, 1) == "0")) return 0
          octet = parts[i] + 0
          if (octet < 0 || octet > 255) return 0
        }
        return 1
      }

      function normalize_ipv4(value, parts) {
        split(value, parts, ".")
        return (parts[1] + 0) "." (parts[2] + 0) "." (parts[3] + 0) "." (parts[4] + 0)
      }

      function count_hextets(value, parts, count, i) {
        if (value == "") return 0
        count = split(value, parts, ":")
        for (i = 1; i <= count; i++) {
          if (length(parts[i]) < 1 || length(parts[i]) > 4 || parts[i] !~ /^[0-9A-Fa-f]+$/) return -1
        }
        return count
      }

      function valid_ipv6(value, last_colon, ipv4_tail, compressed_at, left, right, left_count, right_count, i) {
        if (value == "" || value !~ /^[0-9A-Fa-f:.]+$/) return 0

        if (index(value, ".") > 0) {
          last_colon = 0
          for (i = 1; i <= length(value); i++) {
            if (substr(value, i, 1) == ":") last_colon = i
          }
          if (last_colon == 0) return 0
          ipv4_tail = substr(value, last_colon + 1)
          if (!valid_ipv4(ipv4_tail)) return 0
          value = substr(value, 1, last_colon) "0:0"
        }

        if (value ~ /:::/) return 0
        compressed_at = index(value, "::")
        if (compressed_at > 0) {
          if (index(substr(value, compressed_at + 2), "::") > 0) return 0
          left = substr(value, 1, compressed_at - 1)
          right = substr(value, compressed_at + 2)
          left_count = count_hextets(left)
          right_count = count_hextets(right)
          return left_count >= 0 && right_count >= 0 && left_count + right_count < 8
        }

        return count_hextets(value) == 8
      }

      function normalize_hostname(value, labels, count, i, label) {
        if (substr(value, length(value), 1) == ".") value = substr(value, 1, length(value) - 1)
        if (value == "" || length(value) > 253) return ""
        count = split(value, labels, ".")
        for (i = 1; i <= count; i++) {
          label = labels[i]
          if (length(label) < 1 || length(label) > 63) return ""
          if (label !~ /^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$/) return ""
        }
        return tolower(value)
      }

      NR > 1 { exit 1 }

      {
        endpoint = $0
        if (endpoint == "" || endpoint ~ /[[:space:]]/) exit 1

        bracketed_ipv6 = 0
        if (substr(endpoint, 1, 1) == "[") {
          closing = index(endpoint, "]")
          if (closing <= 2 || substr(endpoint, closing + 1) !~ /^:[0-9]+$/) exit 1
          host = substr(endpoint, 2, closing - 2)
          port_text = substr(endpoint, closing + 2)
          bracketed_ipv6 = 1
        } else {
          if (index(endpoint, "[") > 0 || index(endpoint, "]") > 0) exit 1
          colon_count = gsub(/:/, ":", endpoint)
          if (colon_count != 1) exit 1
          separator = index(endpoint, ":")
          host = substr(endpoint, 1, separator - 1)
          port_text = substr(endpoint, separator + 1)
        }

        if (host == "" || port_text !~ /^[0-9]+$/ || port_text + 0 < 1 || port_text + 0 > 65535) exit 1

        if (bracketed_ipv6) {
          if (!valid_ipv6(host)) exit 1
          normalized_host = tolower(host)
        } else if (host ~ /^[0-9]+(\.[0-9]+)+$/) {
          if (!valid_ipv4(host)) exit 1
          normalized_host = normalize_ipv4(host)
        } else {
          normalized_host = normalize_hostname(host)
          if (normalized_host == "") exit 1
        }

        printf "%s|%d\n", normalized_host, port_text + 0
      }
    '
  )" || return 1

  MQTT_ENDPOINT_HOST="${parsed%|*}"
  MQTT_ENDPOINT_PORT="${parsed##*|}"
  [ -n "$MQTT_ENDPOINT_HOST" ] && [ -n "$MQTT_ENDPOINT_PORT" ]
}

url_port() {
  value="$1"
  case "$value" in
    http://*:*) printf '%s' "$value" | sed -E 's#^https?://[^:/]+:([0-9]+).*#\1#' ;;
    https://*) printf '443' ;;
    http://*) printf '80' ;;
    *) printf '' ;;
  esac
}

address_host() {
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  printf '%s' "$value" | sed -E 's#^[a-z][a-z0-9+.-]*://##; s#/.*$##; s/:[0-9]+$//'
}

is_ipv6_loopback() {
  printf '%s\n' "${1:-}" | awk '
    {
      value = tolower($0)
      if (value == "" || index(value, ".") > 0) exit 1
      compressed_at = index(value, "::")
      item_count = 0
      if (compressed_at > 0) {
        left = substr(value, 1, compressed_at - 1)
        right = substr(value, compressed_at + 2)
        left_count = left == "" ? 0 : split(left, left_parts, ":")
        right_count = right == "" ? 0 : split(right, right_parts, ":")
        zero_count = 8 - left_count - right_count
        for (i = 1; i <= left_count; i++) items[++item_count] = left_parts[i]
        for (i = 1; i <= zero_count; i++) items[++item_count] = "0"
        for (i = 1; i <= right_count; i++) items[++item_count] = right_parts[i]
      } else {
        item_count = split(value, items, ":")
      }
      if (item_count != 8) exit 1
      for (i = 1; i < 8; i++) {
        if (items[i] !~ /^0+$/) exit 1
      }
      exit items[8] ~ /^0*1$/ ? 0 : 1
    }
  '
}

is_local_host_value() {
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  value="${value%.}"
  case "$value" in
    \[*\]) value="${value#\[}"; value="${value%\]}" ;;
  esac
  case "$value" in
    localhost|127.0.0.1|0.0.0.0|::|::1) return 0 ;;
  esac
  is_ipv6_loopback "$value"
}

is_local_host() {
  is_local_host_value "$(address_host "$1")"
}

is_placeholder_host_value() {
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  value="${value%.}"
  case "$value" in
    ""|example.com|example.net|example.org|your-ip|your_ip|your-domain|your_domain|change-me|change_me|placeholder|todo) return 0 ;;
    *) return 1 ;;
  esac
}

is_server_address() {
  host="$(address_host "${1:-}")"
  [ -n "$host" ] || return 1
  is_local_host_value "$host" && return 1
  is_placeholder_host_value "$host" && return 1
  return 0
}

check_path() {
  name="$1"
  path="$2"
  type="$3"

  if { [ "$type" = "dir" ] && [ -d "$path" ]; } || { [ "$type" = "file" ] && [ -f "$path" ]; }; then
    add_check error 1 "path-$name" "$path exists."
  else
    add_check error 0 "path-$name" "$path is missing." "Restore $path before building the private deployment package."
  fi
}

check_tcp_connect() {
  host="$1"
  port="$2"
  python_bin=""
  if command -v python3 >/dev/null 2>&1; then
    python_bin="python3"
  elif command -v python >/dev/null 2>&1; then
    python_bin="python"
  fi

  if [ -z "$python_bin" ]; then
    return 2
  fi

  "$python_bin" - "$host" "$port" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.settimeout(3)
try:
    sock.connect((host, port))
except OSError:
    raise SystemExit(1)
finally:
    sock.close()
PY
}

check_live_postgres() {
  db_host="$(env_or_default GOTP_DB_PSQL_HOST postgres)"
  db_port="$(env_or_default GOTP_DB_PSQL_PORT 5432)"
  db_name="$(env_or_default GOTP_DB_PSQL_DBNAME aetherlink_iot)"
  db_user="$(env_or_default GOTP_DB_PSQL_USERNAME postgres)"
  db_password="$(env_or_default GOTP_DB_PSQL_PASSWORD "")"

  case "$db_port" in
    ''|*[!0-9]*)
      add_check error 0 postgres-live-port "GOTP_DB_PSQL_PORT=$db_port is not a valid TCP port." "Set GOTP_DB_PSQL_PORT to a value between 1 and 65535."
      return
      ;;
  esac
  if [ "$db_port" -lt 1 ] || [ "$db_port" -gt 65535 ]; then
    add_check error 0 postgres-live-port "GOTP_DB_PSQL_PORT=$db_port is not a valid TCP port." "Set GOTP_DB_PSQL_PORT to a value between 1 and 65535."
    return
  fi

  if command -v pg_isready >/dev/null 2>&1; then
    if pg_isready -h "$db_host" -p "$db_port" -U "$db_user" -d "$db_name" >/dev/null 2>&1; then
      add_check error 1 postgres-live-pg-isready "pg_isready reached $db_host:$db_port/$db_name."
    else
      add_check error 0 postgres-live-pg-isready "pg_isready could not reach $db_host:$db_port/$db_name." "Start PostgreSQL or fix GOTP_DB_PSQL_HOST/PORT/DBNAME/USERNAME."
    fi
  else
    add_check warning 0 postgres-live-pg-isready "pg_isready is not installed; falling back to TCP reachability only." "Install PostgreSQL client tools for a stronger live DB preflight."
  fi

  if command -v psql >/dev/null 2>&1 && [ "$db_password" ]; then
    if PGPASSWORD="$db_password" psql "host=$db_host port=$db_port user=$db_user dbname=$db_name connect_timeout=3 sslmode=prefer" -Atqc "SELECT 1" >/dev/null 2>&1; then
      add_check error 1 postgres-live-select "psql SELECT 1 succeeded."
    else
      add_check error 0 postgres-live-select "psql SELECT 1 failed." "Fix PostgreSQL credentials, database name, network route, or pg_hba.conf."
    fi
    return
  fi

  if check_tcp_connect "$db_host" "$db_port"; then
    add_check error 1 postgres-live-tcp "TCP connection to $db_host:$db_port succeeded."
  else
    rc=$?
    if [ "$rc" -eq 2 ]; then
      add_check warning 0 postgres-live-tcp "TCP DB reachability could not be checked because Python is unavailable." "Install python3 or PostgreSQL client tools for live DB preflight."
    else
      add_check error 0 postgres-live-tcp "TCP connection to $db_host:$db_port failed." "Start PostgreSQL, expose the port, or set GOTP_DB_PSQL_HOST to the address reachable from this machine."
    fi
  fi

  if ! command -v psql >/dev/null 2>&1; then
    add_check warning 0 postgres-live-auth "psql is not installed, so credentials were not verified." "Install PostgreSQL client tools to let doctor run SELECT 1."
  fi
}

if [ "${AETHERLINK_DOCTOR_LIBRARY_ONLY:-0}" = "1" ]; then
  return 0 2>/dev/null || exit 0
fi

printf 'AetherLink IoT deployment doctor\n'

if [ -f .env ]; then
  add_check error 1 env-file ".env exists."
  check_env_syntax
  check_env_example_keys
else
  add_check error 0 env-file ".env does not exist." "Run ./deploy/init.sh once to create .env with generated secrets."
fi

if command -v docker >/dev/null 2>&1; then
  add_check error 1 docker-cli "Docker CLI found."
  if docker compose version >/dev/null 2>&1; then
    add_check error 1 docker-compose "Docker Compose v2 plugin is available."
  else
    add_check error 0 docker-compose "Docker Compose v2 plugin is unavailable." "Install or enable the Docker Compose v2 plugin."
  fi
  if docker info >/dev/null 2>&1; then
    add_check error 1 docker-daemon "Docker daemon is reachable."
  else
    add_check error 0 docker-daemon "Docker daemon is not reachable." "Start Docker Desktop or the Docker Engine service."
  fi
else
  add_check error 0 docker-cli "Docker CLI was not found." "Install Docker Desktop or Docker Engine."
  add_check error 0 docker-compose "Docker Compose could not be checked because Docker is missing." "Install Docker first."
  add_check error 0 docker-daemon "Docker daemon could not be checked because Docker is missing." "Install and start Docker first."
fi

check_path env-example .env.example file
check_path compose-file docker-compose.yml file
check_path backend backend dir
check_path frontend frontend dir
check_path mqtt-broker mqtt-broker dir
check_path backend-dockerfile backend/Dockerfile file
check_path frontend-dockerfile frontend/Dockerfile file
check_path mqtt-broker-dockerfile mqtt-broker/Dockerfile file
check_path backend-sql backend/sql dir
check_path postgres-migrations deploy/postgres/00-run-migrations.sh file
check_path gmqtt-config mqtt-broker/cmd/gmqttd/default_config.yml file
check_path gmqtt-aetherlink-example mqtt-broker/cmd/gmqttd/aetherlink.example.yml file

offline_image_count="$(
  find deploy/images images -type f \( -name '*.tar' -o -name '*.tar.gz' -o -name '*.tgz' \) 2>/dev/null | wc -l | tr -d ' '
)"
if [ "${offline_image_count:-0}" -gt 0 ]; then
  add_check warning 1 package-boundary-source-build "Offline image archive count: ${offline_image_count}."
else
  add_check warning 0 package-boundary-source-build "Offline image archive count: 0. This package builds or pulls Docker images on the target machine." "For air-gapped installs, prepare image tarballs under deploy/images or use a private registry before running init."
fi

if [ -f .env ]; then
  for name in POSTGRES_PASSWORD GOTP_DB_PSQL_PASSWORD REDIS_PASSWORD GOTP_DB_REDIS_PASSWORD MQTT_ROOT_PASSWORD MQTT_PLUGIN_PASSWORD GOTP_MQTT_PASS GOTP_JWT_KEY; do
    value="$(env_or_default "$name" "")"
    if is_placeholder "$value"; then
      add_check error 0 "env-$name" "$name is missing or still a placeholder." "Regenerate .env with ./deploy/init.sh or edit $name manually."
    else
      add_check error 1 "env-$name" "$name is set."
    fi
  done

  check_secret_length GOTP_JWT_KEY "$(env_or_default GOTP_JWT_KEY "")" 32
  check_secret_length POSTGRES_PASSWORD "$(env_or_default POSTGRES_PASSWORD "")" 16
  check_secret_length REDIS_PASSWORD "$(env_or_default REDIS_PASSWORD "")" 16
  check_secret_length MQTT_ROOT_PASSWORD "$(env_or_default MQTT_ROOT_PASSWORD "")" 16
  check_secret_length MQTT_PLUGIN_PASSWORD "$(env_or_default MQTT_PLUGIN_PASSWORD "")" 16
  check_weak_secret POSTGRES_PASSWORD "$(env_or_default POSTGRES_PASSWORD "")"
  check_weak_secret REDIS_PASSWORD "$(env_or_default REDIS_PASSWORD "")"
  check_weak_secret MQTT_ROOT_PASSWORD "$(env_or_default MQTT_ROOT_PASSWORD "")"
  check_weak_secret MQTT_PLUGIN_PASSWORD "$(env_or_default MQTT_PLUGIN_PASSWORD "")"

  postgres_password="$(env_or_default POSTGRES_PASSWORD "")"
  gotp_postgres_password="$(env_or_default GOTP_DB_PSQL_PASSWORD "")"
  if [ "$postgres_password" = "$gotp_postgres_password" ]; then
    add_check error 1 postgres-password-match "POSTGRES_PASSWORD and GOTP_DB_PSQL_PASSWORD match."
  else
    add_check error 0 postgres-password-match "POSTGRES_PASSWORD and GOTP_DB_PSQL_PASSWORD do not match." "Set both values to the same generated password."
  fi

  postgres_db="$(env_or_default POSTGRES_DB "")"
  gotp_postgres_db="$(env_or_default GOTP_DB_PSQL_DBNAME "")"
  if [ -n "$postgres_db" ] && [ -n "$gotp_postgres_db" ] && [ "$postgres_db" = "$gotp_postgres_db" ]; then
    add_check error 1 postgres-database-match "POSTGRES_DB and GOTP_DB_PSQL_DBNAME match."
  else
    add_check error 0 postgres-database-match "POSTGRES_DB and GOTP_DB_PSQL_DBNAME do not match or are empty." "Set POSTGRES_DB and GOTP_DB_PSQL_DBNAME to the same non-empty database name."
  fi

  postgres_user="$(env_or_default POSTGRES_USER "")"
  gotp_postgres_user="$(env_or_default GOTP_DB_PSQL_USERNAME "")"
  if [ -n "$postgres_user" ] && [ -n "$gotp_postgres_user" ] && [ "$postgres_user" = "$gotp_postgres_user" ]; then
    add_check error 1 postgres-username-match "POSTGRES_USER and GOTP_DB_PSQL_USERNAME match."
  else
    add_check error 0 postgres-username-match "POSTGRES_USER and GOTP_DB_PSQL_USERNAME do not match or are empty." "Set POSTGRES_USER and GOTP_DB_PSQL_USERNAME to the same non-empty username."
  fi

  redis_password="$(env_or_default REDIS_PASSWORD "")"
  gotp_redis_password="$(env_or_default GOTP_DB_REDIS_PASSWORD "")"
  if [ "$redis_password" = "$gotp_redis_password" ]; then
    add_check error 1 redis-password-match "REDIS_PASSWORD and GOTP_DB_REDIS_PASSWORD match."
  else
    add_check error 0 redis-password-match "REDIS_PASSWORD and GOTP_DB_REDIS_PASSWORD do not match." "Set both values to the same generated password."
  fi

  mqtt_root_password="$(env_or_default MQTT_ROOT_PASSWORD "")"
  gotp_mqtt_password="$(env_or_default GOTP_MQTT_PASS "")"
  if [ "$mqtt_root_password" = "$gotp_mqtt_password" ]; then
    add_check error 1 mqtt-password-match "MQTT_ROOT_PASSWORD and GOTP_MQTT_PASS match."
  else
    add_check error 0 mqtt-password-match "MQTT_ROOT_PASSWORD and GOTP_MQTT_PASS do not match." "Set both values to the same generated password."
  fi

  mqtt_plugin_password="$(env_or_default MQTT_PLUGIN_PASSWORD "")"
  if [ -n "$mqtt_root_password" ] && [ -n "$mqtt_plugin_password" ] && [ "$mqtt_root_password" != "$mqtt_plugin_password" ]; then
    add_check error 1 mqtt-plugin-password-distinct "MQTT_ROOT_PASSWORD and MQTT_PLUGIN_PASSWORD are distinct."
  else
    add_check error 0 mqtt-plugin-password-distinct "MQTT_ROOT_PASSWORD and MQTT_PLUGIN_PASSWORD are identical or empty." "Use separate generated passwords for the root MQTT account and broker plugin."
  fi

  mqtt_user="$(env_or_default GOTP_MQTT_USER "")"
  if [ "$mqtt_user" = "root" ]; then
    add_check error 1 mqtt-backend-user "GOTP_MQTT_USER is the broker root integration identity."
  else
    add_check error 0 mqtt-backend-user "GOTP_MQTT_USER must be root for the current broker integration." "Set GOTP_MQTT_USER=root."
  fi

  mqtt_broker_id="$(env_or_default MQTT_BROKER_ID "")"
  if [ "${#mqtt_broker_id}" -le 128 ] && printf '%s' "$mqtt_broker_id" | grep -Eq '^[A-Za-z0-9._:-]+$'; then
    add_check error 1 mqtt-broker-id "MQTT_BROKER_ID is a valid stable broker identity."
  else
    add_check error 0 mqtt-broker-id "MQTT_BROKER_ID is missing or invalid." "Use 1-128 characters from letters, digits, dot, underscore, colon, and hyphen; keep it stable across restarts."
  fi

  frontend_port="$(env_or_default FRONTEND_PORT 8080)"
  backend_port="$(env_or_default BACKEND_PORT 9999)"
  mqtt_port="$(env_or_default MQTT_PORT 1883)"
  broker_metrics_port="$(env_or_default BROKER_METRICS_PORT 8082)"
  check_port_duplicates \
    "FRONTEND_PORT=$frontend_port" \
    "BACKEND_PORT=$backend_port" \
    "MQTT_PORT=$mqtt_port" \
    "BROKER_METRICS_PORT=$broker_metrics_port"
  check_port FRONTEND_PORT "$frontend_port"
  check_port BACKEND_PORT "$backend_port"
  check_port MQTT_PORT "$mqtt_port"
  check_port BROKER_METRICS_PORT "$broker_metrics_port"
fi

public_url="${AETHERLINK_PUBLIC_URL:-$(env_or_default AETHERLINK_PUBLIC_URL http://localhost:8080)}"
case "$public_url" in
  http://*|https://*) add_check error 1 public-url "AETHERLINK_PUBLIC_URL=$public_url." ;;
  *) add_check error 0 public-url "AETHERLINK_PUBLIC_URL=$public_url is not a full URL." "Use a full URL such as http://192.168.1.10:8080." ;;
esac

mqtt_address="${AETHERLINK_MQTT_ACCESS_ADDRESS:-$(env_or_default AETHERLINK_MQTT_ACCESS_ADDRESS localhost:1883)}"
mqtt_endpoint_valid=0
mqtt_host=""
mqtt_public_port=""
if parse_mqtt_endpoint "$mqtt_address"; then
  mqtt_endpoint_valid=1
  mqtt_host="$MQTT_ENDPOINT_HOST"
  mqtt_public_port="$MQTT_ENDPOINT_PORT"
  add_check error 1 mqtt-address "AETHERLINK_MQTT_ACCESS_ADDRESS=$mqtt_address is a valid MQTT host:port endpoint."
else
  add_check error 0 mqtt-address "AETHERLINK_MQTT_ACCESS_ADDRESS=$mqtt_address is not a valid MQTT host:port endpoint." "Use a hostname, IPv4 address, or bracketed IPv6 address plus a port from 1 to 65535, for example broker.example.com:1883 or [2001:db8::1]:1883."
fi

if [ "$SERVER_MODE" = "1" ]; then
  if ! is_server_address "$public_url"; then
    add_check error 0 server-public-url-not-local "Server mode public URL is missing, local-only, or a placeholder: $public_url." "Set --public-url or AETHERLINK_PUBLIC_URL to an IP/domain users can open, for example http://192.168.1.10:8080."
  else
    add_check error 1 server-public-url-not-local "Server mode public URL is a non-local, non-placeholder address."
  fi

  if [ "$mqtt_endpoint_valid" = "1" ]; then
    if ! is_server_address "$mqtt_address"; then
      add_check error 0 server-mqtt-address-not-local "Server mode MQTT address is missing, local-only, or a placeholder: $mqtt_address." "Set --mqtt-address or AETHERLINK_MQTT_ACCESS_ADDRESS to an IP/domain devices can reach, for example 192.168.1.10:1883."
    else
      add_check error 1 server-mqtt-address-not-local "Server mode MQTT address is a non-local, non-placeholder endpoint."
    fi
  fi
fi

if [ -f .env ]; then
  gotp_ota_address="$(env_or_default GOTP_OTA_DOWNLOAD_ADDRESS "")"
  gotp_mqtt_address="$(env_or_default GOTP_MQTT_ACCESS_ADDRESS "")"
  if [ "$public_url" = "$gotp_ota_address" ]; then
    add_check error 1 public-url-ota-match "AETHERLINK_PUBLIC_URL matches GOTP_OTA_DOWNLOAD_ADDRESS."
  else
    add_check error 0 public-url-ota-match "AETHERLINK_PUBLIC_URL does not match GOTP_OTA_DOWNLOAD_ADDRESS." "Set GOTP_OTA_DOWNLOAD_ADDRESS to the same public URL used by the frontend."
  fi
  if [ "$mqtt_address" = "$gotp_mqtt_address" ]; then
    add_check error 1 mqtt-address-backend-match "AETHERLINK_MQTT_ACCESS_ADDRESS matches GOTP_MQTT_ACCESS_ADDRESS."
  else
    add_check error 0 mqtt-address-backend-match "AETHERLINK_MQTT_ACCESS_ADDRESS does not match GOTP_MQTT_ACCESS_ADDRESS." "Set GOTP_MQTT_ACCESS_ADDRESS to the same host:port shown to devices."
  fi

  service_port="$(env_or_default GOTP_SERVICE_HTTP_PORT 9999)"
  if [ "$service_port" = "9999" ]; then
    add_check error 1 backend-container-port "GOTP_SERVICE_HTTP_PORT=9999."
  else
    add_check error 0 backend-container-port "GOTP_SERVICE_HTTP_PORT=$service_port." "Keep GOTP_SERVICE_HTTP_PORT=9999 because docker-compose.yml maps host BACKEND_PORT to container 9999."
  fi

  if [ -z "$PERFORMANCE_TIER" ]; then
    PERFORMANCE_TIER="$(env_or_default AETHERLINK_PERFORMANCE_TIER light)"
  fi
  resolved_performance_tier="$(normalize_performance_tier "$PERFORMANCE_TIER")"
  case "$resolved_performance_tier" in
    light|standard|production) add_check error 1 performance-tier "AETHERLINK_PERFORMANCE_TIER=$resolved_performance_tier." ;;
    *) add_check error 0 performance-tier "AETHERLINK_PERFORMANCE_TIER=$resolved_performance_tier." "Use light, standard, or production." ;;
  esac

  frontend_public_port="$(url_port "$public_url")"
  frontend_port="$(env_or_default FRONTEND_PORT 8080)"
  if [ "$frontend_public_port" = "$frontend_port" ]; then
    add_check warning 1 public-url-port-match "AETHERLINK_PUBLIC_URL port matches FRONTEND_PORT."
  else
    add_check warning 0 public-url-port-match "AETHERLINK_PUBLIC_URL port $frontend_public_port vs FRONTEND_PORT $frontend_port." "Use the exposed FRONTEND_PORT unless a reverse proxy maps it differently."
  fi

  if [ "$mqtt_endpoint_valid" = "1" ]; then
    mqtt_port="$(env_or_default MQTT_PORT 1883)"
    if [ "$mqtt_public_port" = "$mqtt_port" ]; then
      add_check warning 1 mqtt-address-port-match "AETHERLINK_MQTT_ACCESS_ADDRESS port matches MQTT_PORT."
    else
      add_check warning 0 mqtt-address-port-match "AETHERLINK_MQTT_ACCESS_ADDRESS port $mqtt_public_port vs MQTT_PORT $mqtt_port." "Use the exposed MQTT_PORT unless a load balancer maps it differently."
    fi
  fi

  if [ "$LIVE_DB" = "1" ]; then
    check_live_postgres
  fi
fi

if [ "$mqtt_endpoint_valid" = "1" ]; then
  if is_local_host_value "$mqtt_host"; then
    add_check warning 1 mqtt-public-exposure "MQTT access host is $mqtt_host."
  else
    add_check warning 0 mqtt-public-exposure "MQTT access host is $mqtt_host." "If MQTT is reachable outside this machine, confirm broker authentication/ACL and network firewall rules before production use."
  fi
fi

if command -v docker >/dev/null 2>&1 && [ -f .env ]; then
  if docker compose config >/dev/null 2>&1; then
    add_check error 1 compose-config "docker compose config succeeded."
  else
    add_check error 0 compose-config "docker compose config failed." "Fix docker-compose.yml or .env, then rerun doctor."
  fi
fi

disk_kb="$(df -Pk . 2>/dev/null | awk 'NR==2 {print $4}' || true)"
if [ "$disk_kb" ]; then
  if [ "$disk_kb" -ge 8388608 ]; then
    add_check warning 1 disk-free "Free disk is at least 8GB."
  else
    add_check warning 0 disk-free "Free disk is below 8GB." "Free at least 8GB before building images."
  fi
else
  add_check warning 1 disk-free "Disk free space could not be checked."
fi

if [ -r /proc/meminfo ]; then
  mem_kb="$(awk '/MemTotal/ {print $2}' /proc/meminfo)"
  if [ "$mem_kb" -ge 2097152 ]; then
    add_check warning 1 memory "Physical memory is at least 2GB."
  else
    add_check warning 0 memory "Physical memory is below 2GB." "Use at least 2GB RAM for the lightweight stack."
  fi
else
  add_check warning 1 memory "Memory could not be checked on this platform."
fi

printf '\nDoctor summary: %s error(s), %s warning(s).\n' "$ERROR_COUNT" "$WARNING_COUNT"
printf '\nWhat to do next:\n'
if [ "$ERROR_COUNT" -gt 0 ]; then
  printf -- '- Must fix the [ERROR] items above before startup can be trusted.\n'
  if [ "$SERVER_MODE" = "1" ]; then
    printf -- '- After fixing them, rerun: sh ./deploy/init.sh --doctor --server --public-url http://YOUR-IP:8080 --mqtt-address YOUR-IP:1883\n'
  else
    printf -- '- After fixing them, rerun: sh ./deploy/init.sh --doctor\n'
  fi
  printf -- '- If the problem is a port conflict, either stop the other service or edit FRONTEND_PORT, BACKEND_PORT, MQTT_PORT, or BROKER_METRICS_PORT in .env.\n'
  printf -- '- If the problem is an address mismatch, keep AETHERLINK_PUBLIC_URL and GOTP_OTA_DOWNLOAD_ADDRESS the same, and keep AETHERLINK_MQTT_ACCESS_ADDRESS and GOTP_MQTT_ACCESS_ADDRESS the same.\n'
elif [ "$WARNING_COUNT" -gt 0 ]; then
  printf -- '- Startup is not blocked by doctor errors, but review the [WARN] items before production use.\n'
  printf -- '- To start anyway, run: sh ./deploy/init.sh\n'
else
  printf -- '- Preflight is clean. To start, run: sh ./deploy/init.sh\n'
fi

if [ "$ERROR_COUNT" -gt 0 ]; then
  exit 1
fi

exit 0
