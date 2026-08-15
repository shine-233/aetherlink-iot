param(
  [string]$PublicUrl = "",
  [string]$MqttAddress = "",
  [string]$BindAddress = "",
  [string]$PerformanceTier = "",
  [switch]$Server,
  [switch]$Doctor,
  [switch]$NoBuild,
  [switch]$SkipVerify,
  [switch]$NoPause,
  [switch]$Open,
  [switch]$Help
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

if ($Help) {
  Write-Host "AetherLink IoT Windows starter"
  Write-Host "Usage:"
  Write-Host "  .\deploy\start-windows.ps1"
  Write-Host "  .\deploy\start-windows.ps1 -Doctor"
  Write-Host "  .\deploy\start-windows.ps1 -Open"
  Write-Host "  .\deploy\start-windows.ps1 -PerformanceTier light"
  Write-Host "  .\deploy\start-windows.ps1 -Server -PublicUrl http://1.2.3.4:8080 -MqttAddress 1.2.3.4:1883 -BindAddress 0.0.0.0"
  Write-Host ""
  Write-Host "Double-click deploy\start-windows.cmd for the guided first startup."
  exit 0
}

function Resolve-AetherLinkFirstRunAddresses {
  $envPath = Join-Path $Root ".env"
  if ($PublicUrl -or $MqttAddress) {
    return
  }

  if (Test-Path -LiteralPath $envPath) {
    Resolve-AetherLinkExistingEnvAddresses $envPath
    return
  }

  Write-Host "No .env file was found. Choose how devices will reach this install before startup."
  Write-Host "  L = Local only: browser and first device run on this Windows PC."
  Write-Host "  S = Server/private deployment: browser or devices connect from another machine."
  $mode = Read-Host "Choose L or S [L]"
  if ($mode -and $mode.Trim().ToUpperInvariant().StartsWith("S")) {
    $script:Server = $true
    $script:PublicUrl = Read-Host "Browser URL, for example http://192.168.1.10:8080"
    $script:MqttAddress = Read-Host "Device MQTT address, for example 192.168.1.10:1883"
    if (-not $script:PublicUrl -or -not $script:MqttAddress) {
      throw "Server/private deployment needs both PublicUrl and MqttAddress. Restart and enter both values."
    }
    return
  }

  Write-Host "Using local-only defaults: http://localhost:8080 and localhost:1883."
  Write-Host "If a real device runs on another machine, close this window and rerun with -PublicUrl and -MqttAddress."
}

function Get-AetherLinkEnvValue {
  param(
    [string]$EnvPath,
    [string]$Name
  )

  foreach ($line in Get-Content -LiteralPath $EnvPath) {
    if ($line -match "^\s*$([regex]::Escape($Name))\s*=\s*(.*)\s*$") {
      $value = $Matches[1].Trim()
      if ($value.Length -ge 2) {
        $first = $value.Substring(0, 1)
        $last = $value.Substring($value.Length - 1, 1)
        if (($first -eq '"' -and $last -eq '"') -or ($first -eq "'" -and $last -eq "'")) {
          return $value.Substring(1, $value.Length - 2)
        }
      }
      return $value
    }
  }

  return ""
}

function Test-AetherLinkLocalAddress {
  param([string]$Value)

  if (-not $Value) {
    return $false
  }

  return $Value.Trim().ToLowerInvariant() -match "^(https?://|mqtts?://)?(localhost|127\.0\.0\.1|\[::1\]|::1)(:|/|$)"
}

function Get-AetherLinkFirstDeviceUrl {
  param([string]$PublicUrl)

  $baseUrl = $PublicUrl
  if (-not $baseUrl) {
    $baseUrl = "http://localhost:8080"
  }
  return "$($baseUrl -replace '/+$', '')/first-device"
}

function Resolve-AetherLinkExistingEnvAddresses {
  param([string]$EnvPath)

  $currentPublicUrl = Get-AetherLinkEnvValue $EnvPath "AETHERLINK_PUBLIC_URL"
  $currentMqttAddress = Get-AetherLinkEnvValue $EnvPath "AETHERLINK_MQTT_ACCESS_ADDRESS"
  $usesLocalPublicUrl = Test-AetherLinkLocalAddress $currentPublicUrl
  $usesLocalMqttAddress = Test-AetherLinkLocalAddress $currentMqttAddress

  if (-not $usesLocalPublicUrl -and -not $usesLocalMqttAddress) {
    return
  }

  Write-Host ".env already exists and its public addresses still look local-only."
  Write-Host "  Browser URL: $currentPublicUrl"
  Write-Host "  Device MQTT: $currentMqttAddress"
  Write-Host "  K = Keep local-only: browser and first device run on this Windows PC."
  Write-Host "  S = Switch to server/private addresses before startup."
  $mode = Read-Host "Choose K or S [K]"
  if ($mode -and $mode.Trim().ToUpperInvariant().StartsWith("S")) {
    $script:Server = $true
    $script:PublicUrl = Read-Host "Browser URL, for example http://192.168.1.10:8080"
    $script:MqttAddress = Read-Host "Device MQTT address, for example 192.168.1.10:1883"
    if (-not $script:PublicUrl -or -not $script:MqttAddress) {
      throw "Server/private deployment needs both PublicUrl and MqttAddress. Restart and enter both values."
    }
    Write-Host "Existing .env public addresses will be updated before startup. Secrets and volumes are kept."
    return
  }

  Write-Host "Keeping existing local-only addresses from .env."
  Write-Host "If devices connect from another machine later, rerun this starter and choose S, or pass -PublicUrl and -MqttAddress."
}

function Invoke-AetherLinkInit {
  $initArgs = @()
  $initParams = @{}

  Resolve-AetherLinkFirstRunAddresses

  if ($Doctor) { $initArgs += "-Doctor"; $initParams["Doctor"] = $true }
  if ($Server) { $initArgs += "-Server"; $initParams["Server"] = $true }
  if ($NoBuild) { $initArgs += "-NoBuild"; $initParams["NoBuild"] = $true }
  if ($SkipVerify) { $initArgs += "-SkipVerify"; $initParams["SkipVerify"] = $true }
  if ($PublicUrl) {
    $initArgs += "-PublicUrl"
    $initArgs += $PublicUrl
    $initParams["PublicUrl"] = $PublicUrl
  }
  if ($MqttAddress) {
    $initArgs += "-MqttAddress"
    $initArgs += $MqttAddress
    $initParams["MqttAddress"] = $MqttAddress
  }
  if ($BindAddress) {
    $initArgs += "-BindAddress"
    $initArgs += $BindAddress
    $initParams["BindAddress"] = $BindAddress
  }
  if ($PerformanceTier) {
    $initArgs += "-PerformanceTier"
    $initArgs += $PerformanceTier
    $initParams["PerformanceTier"] = $PerformanceTier
  }

  Write-Host "AetherLink IoT Windows starter"
  Write-Host "Project root: $Root"
  Write-Host "Running: .\deploy\init.ps1 $($initArgs -join ' ')"
  Write-Host ""

  & (Join-Path $PSScriptRoot "init.ps1") @initParams
  return $LASTEXITCODE
}

function Show-AetherLinkNextSteps {
  $envPath = Join-Path $Root ".env"
  $publicUrl = Get-AetherLinkEnvValue $envPath "AETHERLINK_PUBLIC_URL"
  $mqttAddress = Get-AetherLinkEnvValue $envPath "AETHERLINK_MQTT_ACCESS_ADDRESS"

  if (-not $publicUrl) {
    $publicUrl = "http://localhost:8080"
  }
  if (-not $mqttAddress) {
    $mqttAddress = "localhost:1883"
  }
  $firstDeviceUrl = Get-AetherLinkFirstDeviceUrl $publicUrl

  Write-Host "Done. Next:"
  Write-Host "  Open: $firstDeviceUrl"
  Write-Host "  Next: follow 接入第一台设备: check deployment health -> generate the first device -> send the first telemetry -> download the success proof."
  Write-Host "  Device MQTT address: $mqttAddress"
  Write-Host ""
  Write-Host "If the first admin page is unavailable, run: .\deploy\first-admin.ps1"

  if ($Open) {
    Write-Host "Opening browser: $firstDeviceUrl"
    Start-Process $firstDeviceUrl | Out-Null
  }
}

try {
  $exitCode = Invoke-AetherLinkInit
} catch {
  Write-Host ""
  Write-Host "AetherLink startup failed before init finished:"
  Write-Host ($_ | Out-String)
  $exitCode = 1
}

Write-Host ""
if ($exitCode -eq 0) {
  Show-AetherLinkNextSteps
} else {
  Write-Host "Not done yet. Run .\deploy\start-windows.ps1 -Doctor, then check docker compose ps and verification/startup-*/manifest.json."
}

if (-not $NoPause) {
  Write-Host ""
  Read-Host "Press Enter to close this window"
}

exit $exitCode
