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
  [switch]$CheckOnly
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

# 4. Live probes: absent or unreachable target is PENDING, never a pass.
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
