#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
DIST_DIR="$ROOT_DIR/frontend/dist"
INDEX_FILE="$DIST_DIR/index.html"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

[ -d "$DIST_DIR" ] || fail "frontend/dist is missing; run the frontend build or let the Dockerfile build stage create it."
[ -f "$INDEX_FILE" ] || fail "frontend/dist/index.html is missing."
[ -f "$DIST_DIR/rdi/logo.png" ] || fail "frontend/dist/rdi/logo.png is missing."

# Validate active Vite asset references. The legacy EasyWasmPlayer line is
# intentionally commented out and is not part of the built entry contract.
grep -E '<script[^>]+type="module"[^>]+src="/assets/index-[^"]+\.js"' "$INDEX_FILE" >/dev/null ||
  fail "dist entry script is missing or does not use a hashed Vite asset."
grep -E '<link[^>]+rel="modulepreload"[^>]+href="/assets/vendor-[^"]+\.js"' "$INDEX_FILE" >/dev/null ||
  fail "dist vendor modulepreload is missing."
grep -E '<link[^>]+rel="stylesheet"[^>]+href="/assets/(vendor|index)-[^"]+\.css"' "$INDEX_FILE" >/dev/null ||
  fail "dist stylesheet asset is missing."

for asset in $(sed -nE 's/.*(src|href)="(\/assets\/[^"?]+)".*/\2/p' "$INDEX_FILE"); do
  [ -f "$DIST_DIR/${asset#/}" ] || fail "dist index references missing asset: $asset"
done

echo "Frontend dist contract: entrypoint, logo, hashed JS/CSS references, and active assets passed"
