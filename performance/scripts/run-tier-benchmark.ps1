param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("1c2g", "2c4g", "4c8g")]
  [string]$Tier,

  [string]$TargetUrl = "http://localhost:8080",
  [string]$BackendUrl = "http://localhost:9999",
  [switch]$RunApiE2E
)

$ErrorActionPreference = "Stop"

$ProjectRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$TierConfigPath = Join-Path $ProjectRoot "performance\tiers.json"
$ScenarioRoot = Join-Path $ProjectRoot "performance\scenarios"
$Timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$ArchiveRoot = Join-Path $ProjectRoot "verification\performance\$Timestamp\$Tier"
$RawRoot = Join-Path $ArchiveRoot "raw"

New-Item -ItemType Directory -Force -Path $RawRoot | Out-Null

$tierConfig = Get-Content -Raw -Encoding UTF8 -Path $TierConfigPath | ConvertFrom-Json
$tierProfile = $tierConfig.tiers.$Tier
if ($null -eq $tierProfile) {
  throw "Tier '$Tier' is missing from $TierConfigPath."
}

$scenarioCatalog = @(Get-ChildItem -Path $ScenarioRoot -Filter "*.json" | Sort-Object Name | ForEach-Object {
  Get-Content -Raw -Encoding UTF8 -Path $_.FullName | ConvertFrom-Json
})

$gitCommit = $null
if (Get-Command git -ErrorAction SilentlyContinue) {
  $gitCommit = (& git -C $ProjectRoot rev-parse --short HEAD 2>$null)
}

$manifest = [ordered]@{
  schema = "aetherlink.performance.benchmark.v1"
  tier = $Tier
  tier_profile = $tierProfile
  started_at = (Get-Date).ToString("o")
  target_url = $TargetUrl
  backend_url = $BackendUrl
  git_commit = $gitCommit
  raw_root = $RawRoot
  scenario_catalog = $scenarioCatalog
  execution_mode = "evidence-scaffold"
  load_generation_executed = $false
  executed_scenarios = @()
  capacity_claim_status = "unknown"
  review_required = $true
  commands = @()
  blocking_gaps = @(
    "This script captures evidence but does not execute the tier duration, API concurrency, or MQTT client load declared in tiers.json.",
    "The scenario catalog records intended scenarios; catalog inclusion is not scenario execution evidence.",
    "Review raw/resource-snapshot.json and confirm resource limits were enforced.",
    "Fill measured device count, message rate, latency, error rate, CPU, memory, DB, and broker metrics before making capacity claims.",
    "Archive API/E2E/Playwright evidence separately when release behavior is in scope."
  )
}

function Invoke-And-Capture {
  param(
    [string]$Name,
    [scriptblock]$Script
  )

  $outFile = Join-Path $RawRoot "$Name.out.txt"
  $errFile = Join-Path $RawRoot "$Name.err.txt"
  $start = Get-Date
  try {
    & $Script 1>$outFile 2>$errFile
    $exitCode = $LASTEXITCODE
    if ($null -eq $exitCode) {
      $exitCode = 0
    }
  } catch {
    $_ | Out-String | Set-Content -Encoding UTF8 -Path $errFile
    $exitCode = 1
  }

  $manifest["commands"] += [ordered]@{
    name = $Name
    started_at = $start.ToString("o")
    finished_at = (Get-Date).ToString("o")
    exit_code = $exitCode
    stdout = $outFile
    stderr = $errFile
  }
}

& (Join-Path $PSScriptRoot "capture-resource-snapshot.ps1") -OutputPath $RawRoot -TargetUrl $TargetUrl -BackendUrl $BackendUrl

Invoke-And-Capture -Name "backend-health" -Script {
  Invoke-WebRequest -UseBasicParsing -Uri "$BackendUrl/health" | Select-Object StatusCode, StatusDescription
}

Invoke-And-Capture -Name "deployment-health" -Script {
  Invoke-WebRequest -UseBasicParsing -Uri "$BackendUrl/api/v1/deployment/health" | Select-Object StatusCode, Content
}

if ($RunApiE2E) {
  Invoke-And-Capture -Name "automation-api-e2e" -Script {
    Push-Location (Join-Path $ProjectRoot "automation_tests")
    try {
      npm run preflight:api-e2e
      if ($LASTEXITCODE -ne 0) {
        throw "API/E2E preflight failed with exit code $LASTEXITCODE."
      }
      node .\run_tests.js --include-e2e --archive
    } finally {
      Pop-Location
    }
  }
}

$manifest["finished_at"] = (Get-Date).ToString("o")
$manifest | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -Path (Join-Path $ArchiveRoot "manifest.json")

if (Get-Command node -ErrorAction SilentlyContinue) {
  node (Join-Path $PSScriptRoot "summarize-tier-report.js") --archive $ArchiveRoot
}

Write-Host "Benchmark archive: $ArchiveRoot"
