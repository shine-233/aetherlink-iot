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

assert_not_contains() {
  file="$1"
  text="$2"
  if grep -F -- "$text" "$file" >/dev/null; then
    fail "$file must not contain: $text"
  fi
}

SH_PACKAGE="$ROOT_DIR/deploy/package.sh"
PS_PACKAGE="$ROOT_DIR/deploy/package.ps1"

# frontend/build is hand-written Vite configuration source. Only the MQTT
# broker's generated build directory is excluded from source packages.
assert_contains "$SH_PACKAGE" "--exclude='mqtt-broker/build'"
assert_not_contains "$SH_PACKAGE" "--exclude='build'"
assert_contains "$SH_PACKAGE" '"retained_source_paths"'
assert_contains "$SH_PACKAGE" '"frontend/build"'
assert_contains "$SH_PACKAGE" "--exclude='*/.env*'"

assert_contains "$PS_PACKAGE" '$excludedPaths = @('
assert_contains "$PS_PACKAGE" '"mqtt-broker/build"'
assert_not_contains "$PS_PACKAGE" '  "build",'
assert_contains "$PS_PACKAGE" 'retained_source_paths = @("frontend/build")'
# The package script must run under Windows PowerShell 5.1 as well as
# PowerShell 7; the .NET Core-only GetRelativePath API is not available there.
assert_contains "$PS_PACKAGE" 'function Get-AetherLinkRelativePath'
assert_not_contains "$PS_PACKAGE" '[System.IO.Path]::GetRelativePath'
assert_contains "$PS_PACKAGE" '$leafName.StartsWith(".env."'
assert_contains "$PS_PACKAGE" 'Follow the first-device onboarding flow'
assert_contains "$PS_PACKAGE" 'required_external_inputs = @('
assert_contains "$PS_PACKAGE" 'server_mode_command_windows'
assert_contains "$PS_PACKAGE" 'AETHERLINK_PUBLIC_URL: provide the real browser address'
assert_contains "$SH_PACKAGE" "--exclude='_localrun'"
assert_contains "$SH_PACKAGE" "--exclude='*.log'"
assert_contains "$SH_PACKAGE" "--exclude='*.tsbuildinfo'"
assert_contains "$PS_PACKAGE" '"_localrun"'
assert_contains "$PS_PACKAGE" 'EndsWith(".log"'
assert_contains "$PS_PACKAGE" '"*.tsbuildinfo"'
assert_contains "$SH_PACKAGE" '"required_external_inputs"'
assert_contains "$SH_PACKAGE" '"server_mode_command_windows"'
assert_contains "$SH_PACKAGE" 'AETHERLINK_PUBLIC_URL: provide the real browser address'

# PackageName is a filename component, not a path. Traversal must fail before
# either entry point reaches its destructive staging cleanup.
TEST_ROOT="${TMPDIR:-/tmp}/aetherlink-package-containment-$$"
mkdir -p "$TEST_ROOT/sh-output" "$TEST_ROOT/ps-output"
printf 'keep\n' >"$TEST_ROOT/sentinel"
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM
if PACKAGE_NAME='../outside' OUTPUT_DIR="$TEST_ROOT/sh-output" sh "$SH_PACKAGE" >"$TEST_ROOT/sh.out" 2>"$TEST_ROOT/sh.err"; then
  fail "package.sh must reject a path-like package name"
fi
assert_contains "$TEST_ROOT/sh.err" "Package refused: package name must be a single path segment."
[ "$(cat "$TEST_ROOT/sentinel")" = "keep" ] || fail "package.sh changed a path outside its output directory"
[ -z "$(find "$TEST_ROOT" -mindepth 1 -maxdepth 1 -name 'outside-*' -print -quit)" ] || fail "package.sh created a path outside its output directory"

if command -v powershell >/dev/null 2>&1; then
  if powershell -NoProfile -NonInteractive -File "$PS_PACKAGE" -OutputDir "$TEST_ROOT/ps-output" -PackageName '..\outside' >"$TEST_ROOT/ps.out" 2>"$TEST_ROOT/ps.err"; then
    fail "package.ps1 must reject a path-like package name"
  fi
  assert_contains "$TEST_ROOT/ps.err" "Package refused: package name must be a single path segment."
  [ "$(cat "$TEST_ROOT/sentinel")" = "keep" ] || fail "package.ps1 changed a path outside its output directory"
  [ -z "$(find "$TEST_ROOT" -mindepth 1 -maxdepth 1 -name 'outside-*' -print -quit)" ] || fail "package.ps1 created a path outside its output directory"
fi

for required in \
  "$ROOT_DIR/frontend/build/config/index.ts" \
  "$ROOT_DIR/frontend/build/plugins/index.ts" \
  "$ROOT_DIR/frontend/vite.config.ts"
do
  [ -f "$required" ] || fail "required package source is missing: $required"
done

echo "Deployment package source boundary contract: 36 passed"
