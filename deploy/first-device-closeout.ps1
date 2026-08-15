param(
  [string]$ArchiveRoot = "",
  [string]$StartupManifest = "",
  [string]$SuccessProof = "",
  [string]$ApiE2EArchive = "",
  [string]$Output = "",
  [string]$OperatorRole = "",
  [string]$Notes = "",
  [switch]$Help
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

if ($Help) {
  Write-Host "AetherLink IoT first-device closeout manifest helper"
  Write-Host "Usage:"
  Write-Host "  .\deploy\first-device-closeout.ps1"
  Write-Host "  .\deploy\first-device-closeout.ps1 -StartupManifest verification\startup-...\manifest.json -SuccessProof path\to\proof.json"
  Write-Host "  .\deploy\first-device-closeout.ps1 -ArchiveRoot verification\startup-... -SuccessProof path\to\proof.json -ApiE2EArchive verification\..."
  Write-Host ""
  Write-Host "This helper pre-fills a closeout manifest. It never marks verdict as passed."
  exit 0
}

function Resolve-AetherLinkPath {
  param([string]$Path)

  if (-not $Path) {
    return ""
  }
  if ([System.IO.Path]::IsPathRooted($Path)) {
    return [System.IO.Path]::GetFullPath($Path)
  }
  return [System.IO.Path]::GetFullPath((Join-Path $Root $Path))
}

function Get-AetherLinkRelativePath {
  param([string]$Path)

  if (-not $Path) {
    return ""
  }
  try {
    return [System.IO.Path]::GetRelativePath($Root.Path, $Path)
  } catch {
    return $Path
  }
}

function Get-AetherLinkJsonValue {
  param(
    [object]$Object,
    [string]$Name,
    [object]$Default = $null
  )

  if ($null -eq $Object) {
    return $Default
  }

  $property = $Object.PSObject.Properties[$Name]
  if ($null -eq $property) {
    return $Default
  }
  return $property.Value
}

function Get-AetherLinkJsonString {
  param(
    [object]$Object,
    [string]$Name
  )

  $value = Get-AetherLinkJsonValue $Object $Name ""
  if ($null -eq $value) {
    return ""
  }
  return [string]$value
}

function Get-AetherLinkGitCommit {
  try {
    $commit = git rev-parse HEAD 2>$null
    if ($LASTEXITCODE -eq 0) {
      return ($commit | Select-Object -First 1)
    }
  } catch {
  }
  return ""
}

function Find-AetherLinkLatestStartupManifest {
  $verificationRoot = Join-Path $Root "verification"
  if (-not (Test-Path -LiteralPath $verificationRoot)) {
    return ""
  }

  $latest = Get-ChildItem -LiteralPath $verificationRoot -Directory -Filter "startup-*" -ErrorAction SilentlyContinue |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1
  if (-not $latest) {
    return ""
  }

  $manifest = Join-Path $latest.FullName "manifest.json"
  if (Test-Path -LiteralPath $manifest) {
    return $manifest
  }
  return ""
}

$templatePath = Join-Path $Root "verification\templates\first-device-closeout-manifest.template.json"
if (-not (Test-Path -LiteralPath $templatePath)) {
  throw "Missing template: $templatePath"
}

$startupManifestPath = Resolve-AetherLinkPath $StartupManifest
if (-not $startupManifestPath) {
  $startupManifestPath = Find-AetherLinkLatestStartupManifest
}

$archiveRootPath = Resolve-AetherLinkPath $ArchiveRoot
if (-not $archiveRootPath -and $startupManifestPath) {
  $archiveRootPath = Split-Path -Parent $startupManifestPath
}
if (-not $archiveRootPath) {
  $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
  $archiveRootPath = Join-Path $Root "verification\first-device-closeout-$timestamp"
}

New-Item -ItemType Directory -Force -Path $archiveRootPath | Out-Null

$outputPath = Resolve-AetherLinkPath $Output
if (-not $outputPath) {
  $outputPath = Join-Path $archiveRootPath "first-device-closeout-manifest.json"
}

$manifest = Get-Content -LiteralPath $templatePath -Raw | ConvertFrom-Json
$manifest.timestamp = (Get-Date).ToString("o")
$manifest.git_commit = Get-AetherLinkGitCommit
$manifest.operator.role = $OperatorRole
$manifest.operator.notes = $Notes

$blockingGaps = New-Object System.Collections.Generic.List[string]

if ($startupManifestPath -and (Test-Path -LiteralPath $startupManifestPath)) {
  $startup = Get-Content -LiteralPath $startupManifestPath -Raw | ConvertFrom-Json
  $manifest.startup_verification.archive = Get-AetherLinkRelativePath (Split-Path -Parent $startupManifestPath)
  $manifest.startup_verification.manifest = Get-AetherLinkRelativePath $startupManifestPath
  $manifest.startup_verification.ok = [bool]$startup.ok
  $manifest.target_url = [string]$startup.target_url
  $manifest.first_device_url = [string]$startup.first_device_url
  if (-not $manifest.first_device_url -and $manifest.target_url) {
    $manifest.first_device_url = "$($manifest.target_url.TrimEnd('/'))/first-device"
  }
  $manifest.backend_url = [string]$startup.backend_url
  $manifest.mqtt_access_address = [string]$startup.mqtt_access_address
  $manifest.delivery.first_device_url = $manifest.first_device_url
  $manifest.delivery.proof_url = if ($manifest.target_url) { "$($manifest.target_url.TrimEnd('/'))/home?onboarding=first-device&focus=proof" } else { "" }
  if (-not $startup.ok) {
    $blockingGaps.Add("startup_verification_not_ok")
  }
} else {
  $blockingGaps.Add("missing_startup_verification_manifest")
}

$successProofPath = Resolve-AetherLinkPath $SuccessProof
if ($successProofPath -and (Test-Path -LiteralPath $successProofPath)) {
  $proof = Get-Content -LiteralPath $successProofPath -Raw | ConvertFrom-Json
  $manifest.success_proof.downloaded = $true
  $manifest.success_proof.file = Get-AetherLinkRelativePath $successProofPath
  $manifest.success_proof.schema = [string]$proof.schema
  $manifest.success_proof.generated_at = Get-AetherLinkJsonString $proof "generated_at"
  $manifest.success_proof.ready = [bool](Get-AetherLinkJsonValue $proof "ready" $false)
  $manifest.success_proof.conclusion = Get-AetherLinkJsonString $proof "conclusion"
  $manifest.success_proof.next_action = Get-AetherLinkJsonString $proof "next_action"
  $manifest.success_proof.current_blocker = $proof.current_blocker
  $manifest.success_proof.handoff_summary = Get-AetherLinkJsonValue $proof "handoff_summary" $manifest.success_proof.handoff_summary
  $manifest.success_proof.proof_items = @(Get-AetherLinkJsonValue $proof "proof_items" @())

  $delivery = Get-AetherLinkJsonValue $proof "delivery" $null
  $manifest.delivery.first_device_url = Get-AetherLinkJsonString $delivery "first_device_url"
  $manifest.delivery.proof_url = Get-AetherLinkJsonString $delivery "proof_url"
  $manifest.delivery.generated_from_page = Get-AetherLinkJsonString $delivery "generated_from_page"
  $manifest.delivery.proof_file_hint = Get-AetherLinkJsonString $delivery "proof_file_hint"
  if (-not $manifest.delivery.first_device_url) {
    $manifest.delivery.first_device_url = $manifest.first_device_url
  }
  if (-not $manifest.delivery.proof_url -and $manifest.target_url) {
    $manifest.delivery.proof_url = "$($manifest.target_url.TrimEnd('/'))/home?onboarding=first-device&focus=proof"
  }
  if ($manifest.delivery.first_device_url) {
    $manifest.first_device_url = $manifest.delivery.first_device_url
  }

  $manifest.first_device.device_id = [string]$proof.device.id
  $manifest.first_device.device_name = [string]$proof.device.name
  $manifest.first_device.device_number = [string]$proof.device.number
  $manifest.first_device.device_created = [bool]$proof.device.id
  $manifest.first_device.product_or_template_created = [bool]$proof.device.config_id
  $manifest.first_device.protocol = [string]$proof.connection.protocol
  $manifest.first_device.connection_endpoint = [string]$proof.connection.endpoint
  $manifest.first_device.report_entry = [string]$proof.connection.report_entry
  $manifest.first_device.control_entry = [string]$proof.connection.control_topic
  $manifest.first_device.credential_state = "redacted"

  $sampleCommandState = Get-AetherLinkJsonString $proof.handoff_summary "sample_command_state"
  $manifest.publish_test.tester = "Web MQTT/HTTP online tester"
  $manifest.publish_test.sample_copied = $sampleCommandState -eq "present"
  $manifest.publish_test.browser_test_sent = [bool]$proof.browser_test.sent_at
  $manifest.publish_test.browser_test_confirmed = $proof.browser_test.status -eq "confirmed"
  $manifest.publish_test.status = Get-AetherLinkJsonString $proof.browser_test "status"
  $manifest.publish_test.message = Get-AetherLinkJsonString $proof.browser_test "message"
  $manifest.publish_test.sent_at = [string]$proof.browser_test.sent_at
  $manifest.publish_test.telemetry_key = [string]$proof.browser_test.telemetry_key
  $manifest.publish_test.telemetry_value = [string]$proof.browser_test.telemetry_value
  $manifest.publish_test.raw_evidence = Get-AetherLinkRelativePath $successProofPath

  $latestTelemetry = Get-AetherLinkJsonValue $proof "latest_telemetry" $null
  $manifest.latest_telemetry.available = [bool](Get-AetherLinkJsonValue $latestTelemetry "available" $false)
  $manifest.latest_telemetry.source = Get-AetherLinkJsonString $latestTelemetry "source"
  $manifest.latest_telemetry.key = Get-AetherLinkJsonString $latestTelemetry "key"
  $manifest.latest_telemetry.value = Get-AetherLinkJsonString $latestTelemetry "value"
  $manifest.latest_telemetry.observed_at = Get-AetherLinkJsonString $latestTelemetry "observed_at"
  $manifest.latest_telemetry.online = [bool](Get-AetherLinkJsonValue $latestTelemetry "online" $false)

  $manifest.runtime_confirmation.device_online = [bool]$proof.device.online
  $manifest.runtime_confirmation.latest_telemetry_visible = [bool](
    (Get-AetherLinkJsonValue $latestTelemetry "available" $false) -or $proof.chart.primary_key
  )
  $manifest.runtime_confirmation.first_chart_generated = [bool]$proof.chart.ready
  $manifest.runtime_confirmation.ready_banner_visible = [bool]$proof.ready
  $manifest.runtime_confirmation.raw_evidence = Get-AetherLinkRelativePath $successProofPath

  $deploymentHealthRows = @(Get-AetherLinkJsonValue $proof "deployment_health" @())
  $deploymentHealthOk = @($deploymentHealthRows | Where-Object { $_.ok }).Count
  $manifest.deployment_health.total = $deploymentHealthRows.Count
  $manifest.deployment_health.ok = $deploymentHealthOk
  $manifest.deployment_health.failed = [Math]::Max($deploymentHealthRows.Count - $deploymentHealthOk, 0)
  $manifest.deployment_health.rows = $deploymentHealthRows

  if ($proof.current_blocker) {
    $blockingGaps.Add("success_proof_current_blocker")
  }
  if (-not $proof.ready) {
    $blockingGaps.Add("success_proof_not_ready")
  }
} else {
  $blockingGaps.Add("missing_downloaded_success_proof")
}

$apiArchivePath = Resolve-AetherLinkPath $ApiE2EArchive
if ($apiArchivePath) {
  $manifest.api_e2e_playwright.archive = Get-AetherLinkRelativePath $apiArchivePath
} else {
  $blockingGaps.Add("missing_api_e2e_playwright_archive")
}

$manifest.verdict = "unknown"
$manifest.blocking_gaps = @($blockingGaps)
$manifest | ConvertTo-Json -Depth 12 | Set-Content -Encoding UTF8 -Path $outputPath

Write-Host "Created first-device closeout manifest:"
Write-Host $outputPath
Write-Host "Verdict remains unknown until real runtime and API/E2E/Playwright evidence are reviewed."
