#!/usr/bin/env sh
set -eu
ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
fail(){ echo "FAIL: $1" >&2; exit 1; }
contains(){ grep -F -- "$2" "$1" >/dev/null || fail "$1 must contain: $2"; }
not_contains(){ ! grep -F -- "$2" "$1" >/dev/null || fail "$1 must not contain: $2"; }
for f in deploy/backup.sh deploy/restore.sh deploy/backup.ps1 deploy/restore.ps1; do [ -f "$ROOT_DIR/$f" ] || fail "missing $f"; done
contains "$ROOT_DIR/deploy/backup.sh" "pg_dump -Fc"
contains "$ROOT_DIR/deploy/backup.sh" "database.dump.sha256"
contains "$ROOT_DIR/deploy/backup.sh" '"includes_cluster_globals": false'
contains "$ROOT_DIR/deploy/backup.sh" "Backup refused: output directory is not empty"
contains "$ROOT_DIR/deploy/backup.sh" "external blocker: Docker is required"
not_contains "$ROOT_DIR/deploy/backup.sh" 'rm -f "$DUMP_FILE" "$HASH_FILE" "$MANIFEST_FILE"'
contains "$ROOT_DIR/deploy/backup.ps1" "Backup refused: output directory is not empty"
contains "$ROOT_DIR/deploy/backup.ps1" "external blocker: Docker is required"
not_contains "$ROOT_DIR/deploy/backup.ps1" 'Remove-Item $dump,$hashFile,$manifest'
contains "$ROOT_DIR/deploy/restore.sh" "--confirm-restore"
contains "$ROOT_DIR/deploy/restore.sh" "SHA-256 mismatch"
contains "$ROOT_DIR/deploy/restore.sh" "pg_restore --clean --if-exists --exit-on-error --single-transaction --no-owner"
contains "$ROOT_DIR/deploy/restore.sh" "ANALYZE;"
contains "$ROOT_DIR/deploy/backup.ps1" "RedirectStandardOutput"
contains "$ROOT_DIR/deploy/restore.ps1" "RedirectStandardInput"
contains "$ROOT_DIR/deploy/restore.ps1" "ConfirmRestore"
contains "$ROOT_DIR/deploy/restore.ps1" "Get-FileHash"
contains "$ROOT_DIR/deploy/restore.ps1" "pg_restore --clean --if-exists --exit-on-error --single-transaction --no-owner"
contains "$ROOT_DIR/deploy/restore.ps1" "ANALYZE;"
for f in "$ROOT_DIR/deploy/backup.sh" "$ROOT_DIR/deploy/restore.sh" "$ROOT_DIR/deploy/backup.ps1" "$ROOT_DIR/deploy/restore.ps1"; do not_contains "$f" "Get-Content -LiteralPath .env"; not_contains "$f" "cat .env"; done
echo "Backup and restore contract: 23 passed"
