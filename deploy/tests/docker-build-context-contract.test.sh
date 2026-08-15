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

BACKEND_IGNORE="$ROOT_DIR/backend/.dockerignore"
FRONTEND_IGNORE="$ROOT_DIR/frontend/.dockerignore"
BROKER_IGNORE="$ROOT_DIR/mqtt-broker/.dockerignore"
FRONTEND_DOCKERFILE="$ROOT_DIR/frontend/Dockerfile"
BACKEND_MAIN="$ROOT_DIR/backend/main.go"
BACKEND_OPTIONS="$ROOT_DIR/backend/internal/app/options.go"

# Private keys and developer-only configuration must never enter a build
# context. The backend treats RSA login encryption as an optional capability.
assert_contains "$BACKEND_IGNORE" 'configs/rsa_key/*.pem'
assert_contains "$BACKEND_IGNORE" 'configs/conf-localdev.yml'
assert_contains "$BACKEND_MAIN" 'app.WithOptionalRsaDecrypt("./configs/rsa_key/private_key.pem")'
assert_contains "$BACKEND_OPTIONS" 'errors.Is(err, os.ErrNotExist)'

# Vite reads every matching .env file during a build. Container builds instead
# receive an explicit set of public browser settings through Docker ARG/ENV.
assert_contains "$FRONTEND_IGNORE" '.env'
assert_contains "$FRONTEND_IGNORE" '.env.*'
assert_contains "$FRONTEND_IGNORE" '!.env.example'
assert_contains "$FRONTEND_DOCKERFILE" 'ARG VITE_BASE_URL=/'
assert_contains "$FRONTEND_DOCKERFILE" 'ARG VITE_ENABLE_THINGSVIS_COMPAT=N'
assert_contains "$FRONTEND_DOCKERFILE" 'VITE_ENABLE_THINGSVIS_COMPAT=${VITE_ENABLE_THINGSVIS_COMPAT}'

# Runtime output and reports remain outside all three module build contexts.
for ignore_file in "$BACKEND_IGNORE" "$FRONTEND_IGNORE" "$BROKER_IGNORE"
do
  assert_contains "$ignore_file" '_localrun'
  assert_contains "$ignore_file" 'reports'
  assert_contains "$ignore_file" 'test-results'
done

# frontend/build is hand-written Vite configuration source, not generated
# output. A broad build exclusion would silently break the frontend image.
assert_not_contains "$FRONTEND_IGNORE" 'build'
[ -f "$ROOT_DIR/frontend/build/config/index.ts" ] || fail 'frontend/build/config/index.ts must remain source input'
[ -f "$ROOT_DIR/frontend/build/plugins/index.ts" ] || fail 'frontend/build/plugins/index.ts must remain source input'

echo "Docker build context contract: 20 assertions passed"
