# Purpose: backup / restore gate (ROADMAP P0.1).
# Contract:
#   - Integrity work (manifest hashing and verification) is real and runs anywhere.
#   - Anything needing dump/restore tooling or a live database is PENDING (exit 2), never a pass.
#   - -CheckOnly never writes and never touches a database.
# Exit codes: 0=pass, 1=violation (integrity or policy), 2=live evidence incomplete (PENDING).
[CmdletBinding()]
param(
  [ValidateSet('plan','backup','restore','manifest','verify')]
  [string]$Action = 'plan',
  [string]$ArtifactPath = '.framework-backup',
  [string]$PsqlPath = 'psql',
  [string]$PgDumpPath = 'pg_dump',
  [string]$Dsn = $env:AETHERLINK_BACKUP_DSN,
  [switch]$CheckOnly
)

$ErrorActionPreference = 'Stop'

$script:Failures = 0
$script:Pending = 0

function Write-Check([string]$Level, [string]$Name, [string]$Detail) {
  Write-Output ("[{0}] {1} :: {2}" -f $Level, $Name, $Detail)
}
function Fail([string]$Name, [string]$Detail) { $script:Failures++; Write-Check 'FAIL' $Name $Detail }
function Pend([string]$Name, [string]$Detail) { $script:Pending++; Write-Check 'PENDING' $Name $Detail }
function Pass([string]$Name, [string]$Detail) { Write-Check 'PASS' $Name $Detail }
function Warn([string]$Name, [string]$Detail) { Write-Check 'WARN' $Name $Detail }

$ManifestName = 'backup-manifest.json'

# Refuse a filesystem root or a drive-qualified bare root as artifact path.
# Note: any Write-Output inside a function lands in its return value, so this sets a script-scope
# flag instead of returning a bool. Returning a bool here silently swallowed the FAIL line and
# let a filesystem root be treated as safe.
function Test-ArtifactPathSafe([string]$Path) {
  if ([string]::IsNullOrWhiteSpace($Path)) {
    Fail 'artifact-path' 'empty artifact path'
    $script:PathSafe = $false
    return
  }
  if ([IO.Path]::IsPathRooted($Path) -and $Path -eq [IO.Path]::GetPathRoot($Path)) {
    Fail 'artifact-path' 'refusing a filesystem root as artifact path'
    $script:PathSafe = $false
    return
  }
  Pass 'artifact-path' $Path
  $script:PathSafe = $true
}

# Tooling and DSN presence. Absent tooling means the dump/restore path cannot be proven.
function Test-Tooling([string]$Dump, [string]$Psql, [string]$Connection) {
  $dumpCmd = Get-Command $Dump -ErrorAction SilentlyContinue
  $psqlCmd = Get-Command $Psql -ErrorAction SilentlyContinue
  if ($null -eq $dumpCmd) { Pend 'tooling-pg_dump' "$Dump not found in PATH" } else { Pass 'tooling-pg_dump' $dumpCmd.Source }
  if ($null -eq $psqlCmd) { Pend 'tooling-psql' "$Psql not found in PATH" } else { Pass 'tooling-psql' $psqlCmd.Source }
  if ([string]::IsNullOrWhiteSpace($Connection) -or $Connection -like '*CHANGE_ME*') {
    Pend 'backup-dsn' 'AETHERLINK_BACKUP_DSN not supplied; no database operation attempted'
  } else {
    Pass 'backup-dsn' 'dsn supplied (value not echoed)'
  }
}

# Write a SHA-256 manifest for every file in the artifact directory.
# The manifest itself is excluded so re-running is idempotent.
function Write-BackupManifest([string]$Path) {
  if (-not (Test-Path $Path)) { Fail 'manifest-write' "artifact path missing: $Path"; return }
  $files = Get-ChildItem -Path $Path -File -Recurse | Where-Object { $_.Name -ne $ManifestName }
  $entries = @()
  foreach ($file in $files) {
    $relative = $file.FullName.Substring((Resolve-Path $Path).Path.Length).TrimStart('\', '/')
    $hash = (Get-FileHash -Path $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    $entries += [pscustomobject]@{ path = $relative; bytes = $file.Length; sha256 = $hash }
  }
  $manifest = [pscustomobject]@{
    schema    = 'aetherlink.backup.manifest.v1'
    generated = (Get-Date).ToUniversalTime().ToString('o')
    count     = $entries.Count
    files     = $entries
  }
  $target = Join-Path $Path $ManifestName
  $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $target -Encoding UTF8
  Pass 'manifest-write' "$($entries.Count) file(s) hashed -> $ManifestName"
}

# Verify every manifest entry: missing file or hash mismatch is a hard FAIL.
function Test-BackupManifest([string]$Path) {
  $target = Join-Path $Path $ManifestName
  if (-not (Test-Path $target)) {
    Pend 'manifest-verify' "no $ManifestName; artifact integrity unproven"
    return
  }
  $manifest = Get-Content -LiteralPath $target -Raw | ConvertFrom-Json
  $root = (Resolve-Path $Path).Path
  $checked = 0
  foreach ($entry in $manifest.files) {
    $full = Join-Path $root $entry.path
    if (-not (Test-Path $full)) { Fail 'manifest-verify' "missing artifact: $($entry.path)"; continue }
    $hash = (Get-FileHash -Path $full -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($hash -ne $entry.sha256) { Fail 'manifest-verify' "hash mismatch: $($entry.path)"; continue }
    $checked++
  }
  if ($script:Failures -eq 0) { Pass 'manifest-verify' "$checked file(s) match manifest" }
}

Write-Output "AetherLink backup/restore :: Action=$Action Artifact=$ArtifactPath CheckOnly=$CheckOnly"
$script:PathSafe = $false
Test-ArtifactPathSafe $ArtifactPath
$safe = $script:PathSafe

switch ($Action) {
  'plan' {
    Test-Tooling $PgDumpPath $PsqlPath $Dsn
    if ($safe -and (Test-Path $ArtifactPath)) { Test-BackupManifest $ArtifactPath }
    elseif ($safe) { Pend 'artifact-state' "artifact path does not exist: $ArtifactPath" }
  }
  'manifest' {
    if (-not $safe) { break }
    if ($CheckOnly) { Pend 'manifest-write' 'CheckOnly=true; refusing to write'; break }
    if (-not (Test-Path $ArtifactPath)) { New-Item -ItemType Directory -Path $ArtifactPath -Force | Out-Null }
    Write-BackupManifest $ArtifactPath
  }
  'verify' {
    if (-not $safe) { break }
    Test-BackupManifest $ArtifactPath
  }
  'backup' {
    Test-Tooling $PgDumpPath $PsqlPath $Dsn
    Pend 'backup-exec' 'dump/restore execution not implemented; this gate only proves readiness and integrity'
  }
  'restore' {
    Test-Tooling $PgDumpPath $PsqlPath $Dsn
    if ($safe -and (Test-Path $ArtifactPath)) { Test-BackupManifest $ArtifactPath }
    Pend 'restore-exec' 'restore execution not implemented; refusal keeps a half-verified restore from being reported as success'
  }
}

Write-Output ''
if ($script:Failures -gt 0) {
  Write-Output "VERDICT=BLOCKED :: $($script:Failures) violation(s)"
  exit 1
}
if ($script:Pending -gt 0) {
  Write-Output "VERDICT=PENDING :: $($script:Pending) check(s) not executed"
  exit 2
}
Write-Output 'VERDICT=PASS :: all executed checks pass'
exit 0
