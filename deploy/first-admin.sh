#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

BACKEND_URL="${AETHERLINK_FIRST_ADMIN_BACKEND_URL:-}"
EMAIL="${AETHERLINK_FIRST_ADMIN_EMAIL:-}"
PASSWORD="${AETHERLINK_FIRST_ADMIN_PASSWORD:-}"
HTTP_CONNECT_TIMEOUT_SECONDS="${AETHERLINK_HTTP_CONNECT_TIMEOUT_SECONDS:-5}"
SETUP_REQUEST_TIMEOUT_SECONDS="${AETHERLINK_FIRST_ADMIN_SETUP_TIMEOUT_SECONDS:-15}"
INIT_REQUEST_TIMEOUT_SECONDS="${AETHERLINK_FIRST_ADMIN_INIT_TIMEOUT_SECONDS:-30}"

usage() {
  cat <<'EOF'
Usage: sh ./deploy/first-admin.sh [--backend-url URL] [--email EMAIL]

Environment:
  AETHERLINK_FIRST_ADMIN_BACKEND_URL
  AETHERLINK_FIRST_ADMIN_EMAIL
  AETHERLINK_FIRST_ADMIN_PASSWORD
EOF
}

require_arg() {
  name="$1"
  value="${2:-}"
  if [ -z "$value" ]; then
    echo "$name requires a value." >&2
    usage >&2
    exit 2
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --backend-url)
      require_arg "$1" "${2:-}"
      BACKEND_URL="$2"
      shift 2
      ;;
    --email)
      require_arg "$1" "${2:-}"
      EMAIL="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

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

json_number_field() {
  field="$1"
  sed -n "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\\([0-9][0-9]*\\).*/\\1/p" | head -n 1
}

json_bool_field() {
  field="$1"
  sed -n "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\\(true\\|false\\).*/\\1/p" | head -n 1
}

json_string_field() {
  field="$1"
  sed -n "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" | head -n 1
}

validate_email() {
  printf '%s' "$1" | grep -Eq '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'
}

validate_password() {
  value="$1"
  length=${#value}
  [ "$length" -ge 8 ] && [ "$length" -le 20 ] || return 1
  printf '%s' "$value" | grep -Eq '^[!-~]+$' || return 1
  printf '%s' "$value" | grep -Eq '[A-Z]' || return 1
  printf '%s' "$value" | grep -Eq '[a-z]' || return 1
  printf '%s' "$value" | grep -Eq '[0-9]' || return 1
  printf '%s' "$value" | grep -Eq '[^A-Za-z0-9]' || return 1
}

print_init_failure_hint() {
  code="$1"
  echo "Super admin init failed with business code ${code:-unknown}." >&2
  case "$code" in
    200055|200056|200057)
      echo "Market registration check failed. Open the frontend first-run page to complete the market return flow, or check market configuration before retrying this script." >&2
      ;;
  esac
}

prompt_password() {
  if [ -n "$PASSWORD" ]; then
    return
  fi
  if [ ! -t 0 ]; then
    echo "Password prompt requires an interactive terminal. Set AETHERLINK_FIRST_ADMIN_PASSWORD for one-time trusted automation." >&2
    exit 1
  fi

  echo "Password rules: 8-20 chars, uppercase, lowercase, number, and special char."
  printf 'Super admin password: '
  stty -echo
  IFS= read -r first_password
  stty echo
  printf '\nConfirm password: '
  stty -echo
  IFS= read -r second_password
  stty echo
  printf '\n'

  if [ "$first_password" != "$second_password" ]; then
    echo "Passwords do not match. No account was created." >&2
    exit 1
  fi
  PASSWORD="$first_password"
}

if ! command -v curl >/dev/null 2>&1; then
  echo "curl was not found. Install curl or use deploy/first-admin.ps1 on Windows." >&2
  exit 1
fi

BACKEND_PORT="${BACKEND_PORT:-$(read_env_value BACKEND_PORT)}"
BACKEND_URL="${BACKEND_URL:-http://localhost:${BACKEND_PORT:-9999}}"
PUBLIC_URL="${AETHERLINK_PUBLIC_URL:-$(read_env_value AETHERLINK_PUBLIC_URL)}"
PUBLIC_URL="${PUBLIC_URL:-http://localhost:8080}"
SETUP_URL="$(join_url "$BACKEND_URL" /api/v1/tenant/setup-state)"
INIT_URL="$(join_url "$BACKEND_URL" /api/v1/tenant/super-admin/init)"

tmp_body="$(mktemp)"
tmp_req="$(mktemp)"
trap 'rm -f "$tmp_body" "$tmp_req"' EXIT HUP INT TERM

echo "Checking first-start state: $SETUP_URL"
http_code="$(curl -sS \
  --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
  --max-time "$SETUP_REQUEST_TIMEOUT_SECONDS" \
  -o "$tmp_body" -w '%{http_code}' -H 'Accept-Language: en_US' "$SETUP_URL" || true)"
if [ "$http_code" != "200" ]; then
  echo "Setup-state request failed with HTTP $http_code." >&2
  cat "$tmp_body" >&2
  exit 1
fi

business_code="$(json_number_field code <"$tmp_body")"
if [ "$business_code" != "200" ]; then
  echo "Setup-state API returned business code ${business_code:-unknown}." >&2
  cat "$tmp_body" >&2
  exit 1
fi

has_admin="$(json_bool_field has_admin <"$tmp_body")"
next_step="$(json_string_field next_step <"$tmp_body")"
if [ "$has_admin" != "false" ] || [ "$next_step" != "create_super_admin" ]; then
  echo "No super admin was created."
  echo "Current next step: ${next_step:-unknown}"
  echo "Open: $PUBLIC_URL"
  exit 0
fi

while [ -z "$EMAIL" ]; do
  printf 'Super admin email: '
  IFS= read -r EMAIL
done
if ! validate_email "$EMAIL"; then
  echo "Email format is invalid. No account was created." >&2
  exit 1
fi

prompt_password
if [ -z "$PASSWORD" ]; then
  echo "Password is required. No account was created." >&2
  exit 1
fi
if ! validate_password "$PASSWORD"; then
  echo "Password does not meet the local rule: 8-20 visible ASCII chars with uppercase, lowercase, number, and special char. No account was created." >&2
  exit 1
fi

printf '{"email":"%s","password":"%s"}' "$(json_escape "$EMAIL")" "$(json_escape "$PASSWORD")" >"$tmp_req"

echo "Creating first super admin..."
http_code="$(curl -sS \
  --connect-timeout "$HTTP_CONNECT_TIMEOUT_SECONDS" \
  --max-time "$INIT_REQUEST_TIMEOUT_SECONDS" \
  -o "$tmp_body" -w '%{http_code}' -X POST "$INIT_URL" \
  -H 'Accept-Language: en_US' \
  -H 'Content-Type: application/json' \
  --data @"$tmp_req" || true)"
if [ "$http_code" != "200" ]; then
  echo "Super admin init request failed with HTTP $http_code." >&2
  echo "Response body was not printed to avoid exposing sensitive data." >&2
  exit 1
fi

business_code="$(json_number_field code <"$tmp_body")"
if [ "$business_code" != "200" ]; then
  print_init_failure_hint "${business_code:-unknown}"
  echo "Response body was not printed to avoid exposing sensitive data." >&2
  exit 1
fi

echo "First super admin created."
echo "Open: $PUBLIC_URL"
echo "Next: sign in as the super admin, create the tenant admin at /management/user?setup=tenant-admin, then sign in as the tenant admin."
echo "After that, follow 接入第一台设备: check deployment health, generate the first device, send one test telemetry message, confirm latest telemetry plus the first chart, then download the success proof."
