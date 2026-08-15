#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/aetherlink-verify-contract.XXXXXX")"
trap 'rm -rf "$TMP_ROOT"' 0 1 2 15

export AETHERLINK_VERIFY_LIBRARY_ONLY=1
export AETHERLINK_VERIFY_ROOT="$ROOT_DIR"
. "$ROOT_DIR/deploy/verify.sh"
unset AETHERLINK_VERIFY_LIBRARY_ONLY

assert_body_ok() {
  file="$1"
  name="$2"
  if ! deployment_health_body_ok "$file"; then
    echo "FAIL: $name should be healthy" >&2
    exit 1
  fi
}

assert_body_failed() {
  file="$1"
  expected="$2"
  name="$3"
  if deployment_health_body_ok "$file"; then
    echo "FAIL: $name should fail closed" >&2
    exit 1
  fi
  failures="$(deployment_failed_checks_json "$file")"
  case "$failures" in
    *"$expected"*) ;;
    *)
      echo "FAIL: $name expected $expected in $failures" >&2
      exit 1
      ;;
  esac
}

healthy="$TMP_ROOT/healthy.json"
failed="$TMP_ROOT/failed.json"
invalid="$TMP_ROOT/invalid.json"
missing_contract="$TMP_ROOT/missing-contract.json"
legacy="$TMP_ROOT/legacy.json"
printf '%s' '{"checks":{"database":{"ok":true},"mqtt":{"ok":true}}}' >"$healthy"
printf '%s' '{"checks":{"database":{"ok":true},"mqtt":{"ok":false}}}' >"$failed"
printf '%s' '{not-json' >"$invalid"
printf '%s' '{}' >"$missing_contract"
printf '%s' '{"frontend_proxy":{"ok":true},"api":{"ok":true}}' >"$legacy"

assert_body_ok "$healthy" "checks healthy"
assert_body_ok "$legacy" "legacy healthy"
assert_body_failed "$failed" "mqtt" "failed dependency"
assert_body_failed "$invalid" "health-payload-invalid-json" "invalid json"
assert_body_failed "$missing_contract" "health-payload-contract" "missing contract"

RAW_ROOT="$TMP_ROOT/raw"
mkdir -p "$RAW_ROOT"
TIMEOUT_SECONDS=0
INTERVAL_SECONDS=1
HTTP_CONNECT_TIMEOUT_SECONDS=3
HTTP_REQUEST_TIMEOUT_SECONDS=7
FIXTURE_BODY="$healthy"

fake_bin="$TMP_ROOT/bin"
mkdir -p "$fake_bin"
cat >"$fake_bin/curl" <<'EOF'
#!/usr/bin/env sh
printf '%s\n' "$@" >"$FAKE_CURL_ARGS"
out_file=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    out_file="$2"
    shift 2
    continue
  fi
  shift
done
cp "$FIXTURE_BODY" "$out_file"
printf '200'
EOF
chmod +x "$fake_bin/curl"
FAKE_CURL_ARGS="$TMP_ROOT/curl.args"
export FAKE_CURL_ARGS FIXTURE_BODY
original_path="$PATH"
PATH="$fake_bin:$PATH"
export PATH
run_http_check bounded-http "http://fixture.invalid"
PATH="$original_path"
export PATH
if ! grep -Fx -- '--connect-timeout' "$FAKE_CURL_ARGS" >/dev/null ||
  ! grep -Fx -- '3' "$FAKE_CURL_ARGS" >/dev/null ||
  ! grep -Fx -- '--max-time' "$FAKE_CURL_ARGS" >/dev/null ||
  ! grep -Fx -- '7' "$FAKE_CURL_ARGS" >/dev/null; then
  echo "FAIL: run_http_check did not pass bounded curl timeouts" >&2
  exit 1
fi

run_http_check() {
  name="$1"
  cp "$FIXTURE_BODY" "${RAW_ROOT}/${name}.out.txt"
  : >"${RAW_ROOT}/${name}.err.txt"
  printf '200' >"${RAW_ROOT}/${name}.status.txt"
  return 0
}

if ! wait_http_check deployment-health "http://fixture.invalid"; then
  echo "FAIL: wait_http_check rejected a healthy deployment body" >&2
  exit 1
fi

FIXTURE_BODY="$failed"
if wait_http_check deployment-health "http://fixture.invalid"; then
  echo "FAIL: wait_http_check accepted a failed dependency" >&2
  exit 1
fi

manifest="$(check_manifest_json deployment-health "http://fixture.invalid")"
case "$manifest" in
  *'"ok": false'*'"mqtt"'*) ;;
  *)
    echo "FAIL: manifest did not preserve the fail-closed result: $manifest" >&2
    exit 1
    ;;
esac

echo "POSIX verify health contract: 7 passed"
