# Purpose: release gate (ROADMAP P0.1). Runs static invariants first, then live probes.
# Contract:
#   - A failed static invariant is FAIL (exit 1) and never excused by other passes.
#   - Missing live environment is never reported as success; it is PENDING (exit 2).
#   - -CheckOnly runs static checks only and performs no network access.
# Exit codes: 0=pass, 1=static invariant failed, 2=live evidence incomplete (PENDING).
[CmdletBinding()]
param(
  [string]$ApiBaseUrl = $env:API_BASE_URL,
  [string]$FrontendUrl = $env:FRONTEND_URL,
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path,
  [switch]$CheckOnly,
  # Migration-chain verification target. A scratch database is created and dropped;
  # no existing database is touched. Variable names follow the automation_tests
  # convention: AETHERLINK_DB_*.
  [string]$MigrationDbHost = $(if ($env:AETHERLINK_DB_HOST) { $env:AETHERLINK_DB_HOST } else { '127.0.0.1' }),
  [string]$MigrationDbPort = $(if ($env:AETHERLINK_DB_PORT) { $env:AETHERLINK_DB_PORT } else { '55433' }),
  [string]$MigrationDbUser = $(if ($env:AETHERLINK_DB_USER) { $env:AETHERLINK_DB_USER } else { 'postgres' }),
  [string]$MigrationDbPassword = $env:AETHERLINK_DB_PASSWORD,
  [string]$PsqlPath = $env:AETHERLINK_PSQL,
  # Explicit toolchain overrides. Needed because a CI/agent shell may have a minimal
  # PATH where psql and go exist on disk but are not resolvable by name.
  [string]$GoPath = $env:AETHERLINK_GO
)

$ErrorActionPreference = 'Stop'

$script:Failures = 0
$script:Pending = 0

function Write-Check([string]$Level, [string]$Name, [string]$Detail) {
  Write-Output ("[{0}] {1} :: {2}" -f $Level, $Name, $Detail)
}

function Fail([string]$Name, [string]$Detail) {
  $script:Failures++
  Write-Check 'FAIL' $Name $Detail
}

function Pend([string]$Name, [string]$Detail) {
  $script:Pending++
  Write-Check 'PENDING' $Name $Detail
}

function Pass([string]$Name, [string]$Detail) {
  Write-Check 'PASS' $Name $Detail
}

function Warn([string]$Name, [string]$Detail) {
  Write-Check 'WARN' $Name $Detail
}

# 1. Migration ceiling: VERSION_NUMBER must equal the max numbered file in backend/sql.
# AGENTS.md requires enumerating the chain and checking global.go before touching migrations.
function Test-MigrationCeiling([string]$Root) {
  $globalGo = Join-Path $Root 'backend/pkg/global/global.go'
  $sqlDir = Join-Path $Root 'backend/sql'
  if (-not (Test-Path $globalGo)) { Fail 'migration-ceiling' "missing $globalGo"; return }
  if (-not (Test-Path $sqlDir)) { Fail 'migration-ceiling' "missing $sqlDir"; return }

  $raw = Get-Content -Raw $globalGo
  $match = [regex]::Match($raw, 'VERSION_NUMBER\s*=\s*(\d+)')
  if (-not $match.Success) { Fail 'migration-ceiling' 'VERSION_NUMBER not found'; return }
  $declared = [int]$match.Groups[1].Value

  $numbers = @()
  foreach ($file in Get-ChildItem -Path $sqlDir -Filter '*.sql' -File) {
    $name = [IO.Path]::GetFileNameWithoutExtension($file.Name)
    if ($name -match '^\d+$') { $numbers += [int]$name }
  }
  if ($numbers.Count -eq 0) { Fail 'migration-ceiling' 'no numbered migration found'; return }
  $max = ($numbers | Sort-Object | Select-Object -Last 1)

  if ($declared -ne $max) {
    Fail 'migration-ceiling' "VERSION_NUMBER=$declared but max migration=$max"
    return
  }
  Pass 'migration-ceiling' "VERSION_NUMBER=$declared matches max migration=$max"
}

# 2. Packaging docs: deploy/package.sh|ps1 copies these by name; a miss breaks the release bundle.
function Test-ReleaseDocs([string]$Root) {
  $required = @('README.md', 'START-HERE.md', 'VALIDATION.md', 'SECURITY.md', 'THIRD_PARTY_NOTICES.md')
  $missing = @()
  foreach ($name in $required) {
    if (-not (Test-Path (Join-Path $Root $name))) { $missing += $name }
  }
  if ($missing.Count -gt 0) { Fail 'release-docs' ('missing: ' + ($missing -join ', ')); return }
  Pass 'release-docs' ('packaging docs present: ' + ($required -join ', '))
}

# 3. Worktree state: dirty tree warns instead of blocking, but must be stated explicitly.
function Test-WorktreeState([string]$Root) {
  $git = Get-Command git -ErrorAction SilentlyContinue
  if ($null -eq $git) { Pend 'worktree-state' 'git not available in this session; snapshot unproven'; return }
  $status = & git -C $Root status --porcelain 2>$null
  if ($LASTEXITCODE -ne 0) { Pend 'worktree-state' 'git status failed; snapshot unproven'; return }
  $dirty = @($status | Where-Object { $_ })
  if ($dirty.Count -gt 0) {
    Warn 'worktree-state' "$($dirty.Count) uncommitted/untracked entries; release snapshot unproven"
    return
  }
  Pass 'worktree-state' 'working tree clean'
}

# 4. Migration chain: apply 1..N on a scratch database and check the landing point.
#
# Why this belongs in the gate instead of "we ran it once by hand":
# Test-MigrationCeiling only compares VERSION_NUMBER against the highest numbered
# file in backend/sql. It proves the numbers agree; it does NOT prove the migrations
# run. Measured lesson: this chain had only ever been verified up to 93.sql while
# migrations were already at 109 - 16 migrations had never run on any clean
# database. A broken migration is the most expensive kind of failure: missing
# features are survivable, "cannot install" is not, and it only surfaces at the
# customer site.
#
# Contract:/n#   - No psql / no go / no password / -CheckOnly -> PENDING, never a pass.
#   - A migration that actually fails -> FAIL (blocks the release): that is not
#     "environment not ready", that is "the product is broken".
#   - Only the scratch database created here is touched, and it is always dropped.
function Test-MigrationChain([string]$Root) {
  $name = 'migration-chain'

  if ($CheckOnly) {
    Pend $name 'CheckOnly=true; scratch-database run intentionally skipped'
    return
  }
  if ([string]::IsNullOrWhiteSpace($MigrationDbPassword)) {
    Pend $name 'AETHERLINK_DB_PASSWORD not set; cannot create a scratch database'
    return
  }

  $psql = $PsqlPath
  if ([string]::IsNullOrWhiteSpace($psql)) {
    # The @() wrapper is load-bearing: when exactly one psql is found, the pipeline
    # returns a *string*, not an array, and indexing a string yields its first
    # character ('C') - so Test-Path fails and psql looks missing. A string's .Count
    # is 1, so an emptiness check does not catch it either. Only reproducible with
    # a single match.
    $candidates = @(
      Get-ChildItem -Path 'C:\Program Files\PostgreSQL' -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending |
        ForEach-Object { Join-Path $_.FullName 'bin\psql.exe' } |
        Where-Object { Test-Path $_ }
    )
    if ($candidates.Count -gt 0) { $psql = $candidates[0] }
  }
  if ([string]::IsNullOrWhiteSpace($psql) -or -not (Test-Path $psql)) {
    Pend $name 'psql not found (set AETHERLINK_PSQL); scratch-database run not executed'
    return
  }
  # Resolve the go toolchain: explicit override, then PATH, then the default install
  # location. A shell with a minimal PATH (common in CI and agent sandboxes) can have
  # go on disk yet fail Get-Command, which would silently degrade this check to
  # PENDING forever - i.e. the gate would look configured but never actually run.
  $go = $GoPath
  if ([string]::IsNullOrWhiteSpace($go)) {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if ($null -ne $goCmd) { $go = $goCmd.Source }
  }
  if ([string]::IsNullOrWhiteSpace($go)) {
    # The @() must wrap the WHOLE pipeline, not just its input: a pipeline that yields
    # a single item returns that item unwrapped, and indexing a string gives its first
    # character ('C'). Putting @() only around the input array does NOT protect you.
    $goCandidates = @(
      @(
        (Join-Path 'C:\Program Files\Go\bin' 'go.exe'),
        $(if ($env:GOROOT) { Join-Path $env:GOROOT 'bin\go.exe' } else { '' })
      ) | Where-Object { $_ -and (Test-Path $_) }
    )
    if ($goCandidates.Count -gt 0) { $go = $goCandidates[0] }
  }
  if ([string]::IsNullOrWhiteSpace($go) -or -not (Test-Path $go)) {
    Pend $name 'go toolchain not found (set AETHERLINK_GO); cannot build cmd/migchaincheck'
    return
  }

  $scratchDb = 'aetherlink_preflight_migchain'
  $dsn = "host=$MigrationDbHost port=$MigrationDbPort user=$MigrationDbUser password=$MigrationDbPassword dbname=$scratchDb sslmode=disable"
  $previousPassword = $env:PGPASSWORD
  $previousTimescale = $env:AETHERLINK_TIMESCALE_MODE
  $created = $false
  try {
    $env:PGPASSWORD = $MigrationDbPassword
    # Drop-then-create so every run starts truly empty; a leftover database from an
    # interrupted run must not silently turn this into an incremental-upgrade check.
    & $psql -h $MigrationDbHost -p $MigrationDbPort -U $MigrationDbUser -d postgres -q -c "DROP DATABASE IF EXISTS $scratchDb;" 2>&1 | Out-Null
    & $psql -h $MigrationDbHost -p $MigrationDbPort -U $MigrationDbUser -d postgres -q -c "CREATE DATABASE $scratchDb;" 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
      Pend $name "cannot reach $MigrationDbHost`:$MigrationDbPort or create scratch database; migration chain unproven"
      return
    }
    $created = $true

    Push-Location (Join-Path $Root 'backend')
    try {
      # Match the local/test stack: Timescale off. 57.sql's hypertable conversion is
      # skipped while the rest of the chain continues.
      $env:AETHERLINK_TIMESCALE_MODE = 'off'
      $output = & $go run ./cmd/migchaincheck -dsn $dsn 2>&1
      $exit = $LASTEXITCODE
    } finally {
      Pop-Location
    }

    $verdict = ($output | Where-Object { $_ -like 'VERDICT=*' } | Select-Object -Last 1)
    if ($exit -eq 0) {
      Pass $name ("scratch database full chain 1..N applied; " + $verdict)
    } elseif ($exit -eq 2) {
      Pend $name ("environment not satisfied: " + $verdict)
    } else {
      Fail $name ("migration chain failed on a clean database: " + $verdict)
    }
  } finally {
    if ($created) {
      & $psql -h $MigrationDbHost -p $MigrationDbPort -U $MigrationDbUser -d postgres -q -c "DROP DATABASE IF EXISTS $scratchDb;" 2>&1 | Out-Null
    }
    $env:PGPASSWORD = $previousPassword
    $env:AETHERLINK_TIMESCALE_MODE = $previousTimescale
  }
}

# 5. Live probes: absent or unreachable target is PENDING, never a pass.
function Test-EnvironmentProbe([string]$Name, [string]$Url) {
  if ([string]::IsNullOrWhiteSpace($Url) -or $Url -like '*CHANGE_ME*') {
    Pend $Name 'no target supplied; live check not executed'
    return
  }
  if ($CheckOnly) {
    Pend $Name "target=$Url but CheckOnly=true; live check intentionally skipped"
    return
  }
  try {
    $uri = [Uri]$Url
    $port = 80
    if ($uri.Port -gt 0) { $port = $uri.Port }
    elseif ($uri.Scheme -eq 'https') { $port = 443 }
    $client = New-Object Net.Sockets.TcpClient
    $task = $client.ConnectAsync($uri.Host, $port)
    if (-not $task.Wait(3000)) { throw 'connect timeout' }
    if (-not $client.Connected) { throw 'not connected' }
    $client.Close()
    Pass $Name "tcp reachable $($uri.Host):$port"
  } catch {
    Pend $Name "target=$Url unreachable ($($_.Exception.Message)); cannot prove deployment"
  }
}

Write-Output "AetherLink release preflight :: RepoRoot=$RepoRoot CheckOnly=$CheckOnly"
Test-MigrationCeiling $RepoRoot
Test-ReleaseDocs $RepoRoot
Test-WorktreeState $RepoRoot
Test-MigrationChain $RepoRoot
Test-EnvironmentProbe 'api-reachable' $ApiBaseUrl
Test-EnvironmentProbe 'frontend-reachable' $FrontendUrl

Write-Output ''
if ($script:Failures -gt 0) {
  Write-Output "VERDICT=BLOCKED :: $($script:Failures) static invariant(s) failed"
  exit 1
}
if ($script:Pending -gt 0) {
  Write-Output "VERDICT=PENDING :: static invariants pass, $($script:Pending) live check(s) not executed"
  exit 2
}
Write-Output 'VERDICT=PASS :: all static invariants and live checks pass'
exit 0
