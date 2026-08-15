param(
  [string]$TargetUrl = "",
  [string]$BackendUrl = "",
  [string]$BrokerMetricsUrl = "",
  [int]$TimeoutSeconds = 180,
  [int]$IntervalSeconds = 5,
  [string]$ArchiveRoot = "",
  [switch]$Server
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

function Read-AetherLinkEnvFile {
  param([string]$Path)

  $values = @{}
  if (-not (Test-Path -LiteralPath $Path)) {
    return $values
  }

  Get-Content -LiteralPath $Path | ForEach-Object {
    $line = $_.Trim()
    if (-not $line -or $line.StartsWith("#") -or -not $line.Contains("=")) {
      return
    }

    $name, $value = $line.Split("=", 2)
    $values[$name.Trim()] = $value.Trim().Trim('"').Trim("'")
  }
  return $values
}

function Join-AetherLinkUrl {
  param(
    [string]$BaseUrl,
    [string]$Path
  )

  return "$($BaseUrl.TrimEnd('/'))/$($Path.TrimStart('/'))"
}

function Get-AetherLinkEnvOrDefault {
  param(
    [hashtable]$Values,
    [string]$Name,
    [string]$Default
  )

  if ($Values.ContainsKey($Name) -and $Values[$Name]) {
    return $Values[$Name]
  }
  return $Default
}

function Get-AetherLinkAddressHost {
  param([string]$Value)

  if ([string]::IsNullOrWhiteSpace($Value)) { return "" }
  $trimmed = $Value.Trim()
  if ($trimmed -match "^[a-zA-Z][a-zA-Z0-9+.-]*://") {
    try { return ([System.Uri]::new($trimmed)).Host.ToLowerInvariant() } catch { return "" }
  }
  if ($trimmed -match "^\[(?<host>[^\]]+)\]:[0-9]+$") { return $Matches.host.ToLowerInvariant() }
  if ($trimmed -match "^(?<host>[^:\s]+):[0-9]+$") { return $Matches.host.ToLowerInvariant() }
  return ""
}

function Test-AetherLinkLocalHost {
  param([string]$HostName)

  if ([string]::IsNullOrWhiteSpace($HostName)) { return $false }
  $normalizedHost = $HostName.Trim().TrimEnd(".").Trim([char[]]"[]").ToLowerInvariant()
  return @("localhost", "127.0.0.1", "0.0.0.0", "::", "::1") -contains $normalizedHost
}

function Test-AetherLinkPlaceholderHost {
  param([string]$HostName)

  if ([string]::IsNullOrWhiteSpace($HostName)) { return $true }
  $normalizedHost = $HostName.Trim().TrimEnd(".").Trim([char[]]"[]").ToLowerInvariant()
  if (@("example.com", "example.net", "example.org") -contains $normalizedHost) { return $true }
  return $normalizedHost -match "^(?:your[-_]?ip|your[-_]?domain|change[-_]?me|placeholder|todo)$"
}

function Test-AetherLinkServerAddress {
  param([string]$Value)

  $addressHost = Get-AetherLinkAddressHost $Value
  if ([string]::IsNullOrWhiteSpace($addressHost)) { return $false }
  return -not (Test-AetherLinkLocalHost $addressHost) -and -not (Test-AetherLinkPlaceholderHost $addressHost)
}

function Read-AetherLinkHttpErrorBody {
  param([object]$Response)

  if (-not $Response) {
    return ""
  }

  try {
    if ($Response.Content) {
      return $Response.Content.ToString()
    }
  } catch {
  }

  try {
    $stream = $Response.GetResponseStream()
    if ($stream) {
      $reader = [System.IO.StreamReader]::new($stream)
      return $reader.ReadToEnd()
    }
  } catch {
  }

  return ""
}

function Get-AetherLinkDeploymentHealthFailures {
  param([string]$Body)

  if ([string]::IsNullOrWhiteSpace($Body)) {
    return @("health-payload-missing")
  }

  try {
    $report = $Body | ConvertFrom-Json
  } catch {
    return @("health-payload-invalid-json")
  }

  $failed = [System.Collections.Generic.List[string]]::new()
  $reportProperties = @($report.PSObject.Properties.Name)
  $hasSupportedContract = $false

  $legacyFields = @("frontend_proxy", "api")
  $hasLegacyContract = @($legacyFields | Where-Object { $reportProperties -contains $_ }).Count -gt 0
  if ($hasLegacyContract) {
    $hasSupportedContract = $true
    foreach ($field in $legacyFields) {
      if ($reportProperties -notcontains $field) {
        $failed.Add($field) | Out-Null
        continue
      }

      $check = $report.$field
      $checkProperties = if ($null -ne $check) { @($check.PSObject.Properties.Name) } else { @() }
      if ($checkProperties -notcontains "ok" -or $check.ok -isnot [bool] -or $check.ok -ne $true) {
        $failed.Add($field) | Out-Null
      }
    }
  }

  if ($reportProperties -contains "checks") {
    $hasSupportedContract = $true
    $checks = $report.checks
    $checkProperties = if ($null -ne $checks -and $checks -isnot [array] -and $checks -isnot [string]) {
      @($checks.PSObject.Properties)
    } else {
      @()
    }

    if ($checkProperties.Count -eq 0) {
      $failed.Add("checks") | Out-Null
    } else {
      foreach ($property in $checkProperties) {
        $check = $property.Value
        $valueProperties = if ($null -ne $check) { @($check.PSObject.Properties.Name) } else { @() }
        if ($valueProperties -notcontains "ok" -or $check.ok -isnot [bool] -or $check.ok -ne $true) {
          $failed.Add($property.Name) | Out-Null
        }
      }
    }
  }

  if (-not $hasSupportedContract) {
    $failed.Add("health-payload-contract") | Out-Null
  }

  return @($failed | Select-Object -Unique)
}

function Invoke-AetherLinkHttpCheck {
  param(
    [string]$Name,
    [string]$Url,
    [int[]]$AcceptStatus = @(200)
  )

  $outFile = Join-Path $RawRoot "$Name.out.txt"
  $errFile = Join-Path $RawRoot "$Name.err.txt"
  $startedAt = Get-Date
  try {
    $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 10
    $body = if ($response.Content) { $response.Content.ToString() } else { "" }
    $body | Set-Content -Encoding UTF8 -Path $outFile
    "" | Set-Content -Encoding UTF8 -Path $errFile
    $failedChecks = if ($Name -eq "deployment-health") { @(Get-AetherLinkDeploymentHealthFailures -Body $body) } else { @() }
    $ok = $AcceptStatus -contains [int]$response.StatusCode
    if ($Name -eq "deployment-health") {
      $ok = $ok -and $failedChecks.Count -eq 0
    }
    return [ordered]@{
      name = $Name
      url = $Url
      started_at = $startedAt.ToString("o")
      finished_at = (Get-Date).ToString("o")
      ok = $ok
      status_code = [int]$response.StatusCode
      failed_checks = $failedChecks
      stdout = $outFile
      stderr = $errFile
    }
  } catch {
    $statusCode = 0
    $body = ""
    if ($_.Exception.Response) {
      try {
        $statusCode = [int]$_.Exception.Response.StatusCode
      } catch {
        $statusCode = 0
      }
      $body = Read-AetherLinkHttpErrorBody -Response $_.Exception.Response
    }
    $_ | Out-String | Set-Content -Encoding UTF8 -Path $errFile
    $body | Set-Content -Encoding UTF8 -Path $outFile
    $failedChecks = if ($Name -eq "deployment-health") { @(Get-AetherLinkDeploymentHealthFailures -Body $body) } else { @() }
    return [ordered]@{
      name = $Name
      url = $Url
      started_at = $startedAt.ToString("o")
      finished_at = (Get-Date).ToString("o")
      ok = $false
      status_code = $statusCode
      failed_checks = $failedChecks
      stdout = $outFile
      stderr = $errFile
    }
  }
}

function Wait-AetherLinkHttpCheck {
  param(
    [string]$Name,
    [string]$Url,
    [int[]]$AcceptStatus = @(200)
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $attempts = @()
  do {
    $attempt = Invoke-AetherLinkHttpCheck -Name $Name -Url $Url -AcceptStatus $AcceptStatus
    $attempts += $attempt
    if ($attempt.ok) {
      return [ordered]@{
        name = $Name
        ok = $true
        attempts = $attempts.Count
        final = $attempt
      }
    }
    Start-Sleep -Seconds $IntervalSeconds
  } while ((Get-Date) -lt $deadline)

  return [ordered]@{
    name = $Name
    ok = $false
    attempts = $attempts.Count
    final = $attempts[-1]
  }
}

$envValues = Read-AetherLinkEnvFile -Path ".env"

if (-not $TargetUrl) {
  $TargetUrl = $env:AETHERLINK_PUBLIC_URL
}
if (-not $TargetUrl) {
  $TargetUrl = $envValues["AETHERLINK_PUBLIC_URL"]
}
if (-not $TargetUrl) {
  $TargetUrl = "http://localhost:$(Get-AetherLinkEnvOrDefault $envValues 'FRONTEND_PORT' '8080')"
}

if (-not $BackendUrl) {
  $BackendUrl = "http://localhost:$(Get-AetherLinkEnvOrDefault $envValues 'BACKEND_PORT' '9999')"
}

if (-not $BrokerMetricsUrl) {
  $BrokerMetricsUrl = "http://localhost:$(Get-AetherLinkEnvOrDefault $envValues 'BROKER_METRICS_PORT' '8082')/metrics"
}

$mqttAccessAddress = $env:AETHERLINK_MQTT_ACCESS_ADDRESS
if (-not $mqttAccessAddress) {
  $mqttAccessAddress = Get-AetherLinkEnvOrDefault $envValues "AETHERLINK_MQTT_ACCESS_ADDRESS" "localhost:1883"
}
$firstDeviceUrl = Join-AetherLinkUrl $TargetUrl "/first-device"
$firstUseNextSteps = @(
  "Open: $firstDeviceUrl",
  "Create the super admin and tenant admin if first-run setup is still pending.",
  "Follow 接入第一台设备: check deployment health, generate the first device, send the first telemetry, then download the success proof.",
  "Use device MQTT address: $mqttAccessAddress"
)

$serverMode = $Server -or $env:AETHERLINK_SERVER_MODE -eq "1"
if ($serverMode) {
  $serverPublicOk = Test-AetherLinkServerAddress $TargetUrl
  $serverMqttOk = Test-AetherLinkServerAddress $mqttAccessAddress
  if (-not $serverPublicOk -or -not $serverMqttOk) {
    Write-Error ("Server verification requires a non-local, non-placeholder AETHERLINK_PUBLIC_URL and AETHERLINK_MQTT_ACCESS_ADDRESS. PublicUrl='{0}', MqttAddress='{1}'." -f $TargetUrl, $mqttAccessAddress)
    exit 2
  }
}

if (-not $ArchiveRoot) {
  $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
  $ArchiveRoot = Join-Path $Root "verification\startup-$timestamp"
} elseif (-not [System.IO.Path]::IsPathRooted($ArchiveRoot)) {
  $ArchiveRoot = Join-Path $Root $ArchiveRoot
}

$ArchiveRoot = [System.IO.Path]::GetFullPath($ArchiveRoot)
$RawRoot = Join-Path $ArchiveRoot "raw"
New-Item -ItemType Directory -Force -Path $RawRoot | Out-Null

$composeOut = Join-Path $RawRoot "docker-compose-ps.out.txt"
$composeErr = Join-Path $RawRoot "docker-compose-ps.err.txt"
try {
  docker compose ps 1>$composeOut 2>$composeErr
  $composeExitCode = $LASTEXITCODE
} catch {
  $_ | Out-String | Set-Content -Encoding UTF8 -Path $composeErr
  $composeExitCode = 1
}

$checks = @(
  (Wait-AetherLinkHttpCheck -Name "frontend-root" -Url $TargetUrl),
  (Wait-AetherLinkHttpCheck -Name "backend-health" -Url (Join-AetherLinkUrl $BackendUrl "/health")),
  (Wait-AetherLinkHttpCheck -Name "deployment-health" -Url (Join-AetherLinkUrl $BackendUrl "/api/v1/deployment/health")),
  (Wait-AetherLinkHttpCheck -Name "broker-metrics" -Url $BrokerMetricsUrl)
)

$ok = ($composeExitCode -eq 0) -and (($checks | Where-Object { -not $_.ok }).Count -eq 0)
$manifest = [ordered]@{
  kind = "startup-verification"
  started_at = (Get-Date).ToString("o")
  target_url = $TargetUrl
  first_device_url = $firstDeviceUrl
  backend_url = $BackendUrl
  broker_metrics_url = $BrokerMetricsUrl
  mqtt_access_address = $mqttAccessAddress
  first_use_next_steps = $firstUseNextSteps
  timeout_seconds = $TimeoutSeconds
  interval_seconds = $IntervalSeconds
  docker_compose_ps = [ordered]@{
    exit_code = $composeExitCode
    stdout = $composeOut
    stderr = $composeErr
  }
  checks = $checks
  ok = $ok
  finished_at = (Get-Date).ToString("o")
}

$manifest | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -Path (Join-Path $ArchiveRoot "manifest.json")

function Write-AetherLinkStartupTroubleshooting {
  param(
    [array]$Checks,
    [int]$ComposeExitCode,
    [string]$Archive
  )

  Write-Host "Startup verification failed. Archive: $Archive"
  Write-Host ""
  Write-Host "Failed checks:"
  if ($ComposeExitCode -ne 0) {
    Write-Host "- docker compose ps failed with exit code $ComposeExitCode"
  }

  $failedChecks = @($Checks | Where-Object { -not $_.ok })
  if ($failedChecks.Count -eq 0 -and $ComposeExitCode -eq 0) {
    Write-Host "- no failed HTTP check was recorded; inspect manifest.json for details"
  }

  foreach ($check in $failedChecks) {
    $final = $check.final
    $statusCode = if ($final) { $final.status_code } else { "unknown" }
    $url = if ($final) { $final.url } else { "unknown" }
    $failedDeps = if ($final -and $final.failed_checks -and $final.failed_checks.Count -gt 0) {
      " failed dependency: $($final.failed_checks -join ', ')"
    } else {
      ""
    }
    Write-Host "- $($check.name): status=$statusCode url=$url$failedDeps"
  }

  Write-Host ""
  Write-Host "Next commands:"
  Write-Host "  docker compose ps"
  Write-Host "  docker compose logs frontend --tail=80"
  Write-Host "  docker compose logs backend --tail=80"
  Write-Host "  docker compose logs mqtt-broker --tail=80"
  Write-Host "  .\deploy\init.ps1 -Doctor"
}

if (-not $ok) {
  Write-AetherLinkStartupTroubleshooting -Checks $checks -ComposeExitCode $composeExitCode -Archive $ArchiveRoot
  exit 1
}

Write-Host "Startup verification passed. Archive: $ArchiveRoot"
