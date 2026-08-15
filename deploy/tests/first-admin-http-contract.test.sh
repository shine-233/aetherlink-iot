#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/aetherlink-first-admin-contract.XXXXXX")"
trap 'rm -rf "$TMP_ROOT"' 0 1 2 15

fake_bin="$TMP_ROOT/bin"
mkdir -p "$fake_bin"
cat >"$fake_bin/curl" <<'EOF'
#!/usr/bin/env sh
{
  printf '%s\n' '--- request ---'
  printf '%s\n' "$@"
} >>"$FAKE_CURL_ARGS"

out_file=""
request_url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      out_file="$2"
      shift 2
      ;;
    http://*|https://*)
      request_url="$1"
      shift
      ;;
    *)
      shift
      ;;
  esac
done

case "$request_url" in
  */api/v1/tenant/setup-state)
    printf '%s' '{"code":200,"data":{"has_admin":false,"next_step":"create_super_admin"}}' >"$out_file"
    ;;
  */api/v1/tenant/super-admin/init)
    printf '%s' '{"code":200}' >"$out_file"
    ;;
  *)
    printf '%s' '{"code":500}' >"$out_file"
    ;;
esac
printf '200'
EOF
chmod +x "$fake_bin/curl"

FAKE_CURL_ARGS="$TMP_ROOT/curl.args"
export FAKE_CURL_ARGS
export PATH="$fake_bin:$PATH"
export AETHERLINK_FIRST_ADMIN_BACKEND_URL="http://fixture.invalid"
export AETHERLINK_FIRST_ADMIN_EMAIL="admin@example.com"
export AETHERLINK_FIRST_ADMIN_PASSWORD='Valid1!x'
export AETHERLINK_HTTP_CONNECT_TIMEOUT_SECONDS=2
export AETHERLINK_FIRST_ADMIN_SETUP_TIMEOUT_SECONDS=11
export AETHERLINK_FIRST_ADMIN_INIT_TIMEOUT_SECONDS=29

output="$(sh "$ROOT_DIR/deploy/first-admin.sh")"
case "$output" in
  *"First super admin created."*) ;;
  *)
    echo "FAIL: first-admin script did not complete against the fake client: $output" >&2
    exit 1
    ;;
esac

request_count="$(grep -cFx -- '--- request ---' "$FAKE_CURL_ARGS")"
if [ "$request_count" != "2" ]; then
  echo "FAIL: expected two HTTP requests, got $request_count" >&2
  exit 1
fi
connect_count="$(grep -cFx -- '--connect-timeout' "$FAKE_CURL_ARGS")"
connect_value_count="$(grep -cFx -- '2' "$FAKE_CURL_ARGS")"
if [ "$connect_count" != "2" ] || [ "$connect_value_count" != "2" ]; then
  echo "FAIL: both requests must use the configured connection timeout" >&2
  exit 1
fi
if ! grep -Fx -- '--max-time' "$FAKE_CURL_ARGS" >/dev/null ||
  ! grep -Fx -- '11' "$FAKE_CURL_ARGS" >/dev/null ||
  ! grep -Fx -- '29' "$FAKE_CURL_ARGS" >/dev/null; then
  echo "FAIL: setup and init requests must use their configured total timeouts" >&2
  exit 1
fi

echo "POSIX first-admin HTTP contract: 1 passed"
