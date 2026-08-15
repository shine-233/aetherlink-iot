param(
  [string]$PublicUrl = $env:AETHERLINK_PUBLIC_URL,
  [string]$MqttAddress = $env:AETHERLINK_MQTT_ACCESS_ADDRESS,
  [string]$BindAddress = $env:AETHERLINK_BIND_ADDRESS,
  [string]$PerformanceTier = $env:AETHERLINK_PERFORMANCE_TIER,
  [switch]$Server,
  [switch]$NoBuild,
  [switch]$SkipVerify,
  [switch]$Doctor,
  [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
  Write-Host "Usage: .\deploy\init.ps1 [-Doctor] [-Server] [-NoBuild] [-SkipVerify] [-PublicUrl <url>] [-MqttAddress <host:port>] [-BindAddress <ip>] [-PerformanceTier light|standard|production]"
  Write-Host "  -Doctor      Run deployment doctor only; do not start containers."
  Write-Host "  -Server      Treat this as a server/private deployment; localhost public addresses become blocking errors."
  Write-Host "  -NoBuild     Start existing images without rebuilding."
  Write-Host "  -SkipVerify  Start containers without running startup health archive."
  Write-Host "  -PerformanceTier  Apply Compose resource presets: light, standard, or production."
  Write-Host "  -BindAddress  Host interface for published ports; server mode defaults a loopback value to 0.0.0.0."
  exit 0
}

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

function New-AetherLinkSecret {
  param([int]$Bytes = 32)

  $buffer = [byte[]]::new($Bytes)
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try {
    $rng.GetBytes($buffer)
  } finally {
    $rng.Dispose()
  }
  return [Convert]::ToBase64String($buffer).TrimEnd("=").Replace("+", "_").Replace("/", "-")
}

function Set-AetherLinkEnvValue {
  param(
    [string]$Content,
    [string]$Name,
    [string]$Value
  )

  $pattern = "(?m)^$([regex]::Escape($Name))=.*$"
  if ($Content -match $pattern) {
    return [regex]::Replace($Content, $pattern, "$Name=$Value")
  }
  return $Content.TrimEnd() + "`n$Name=$Value`n"
}

function Get-AetherLinkEnvValue {
  param(
    [string]$EnvPath,
    [string]$Name
  )

  if (-not (Test-Path -LiteralPath $EnvPath)) {
    return ""
  }

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

function Resolve-AetherLinkPerformanceTier {
  param([string]$Tier)

  $resolved = "light"
  if ($Tier) {
    $resolved = $Tier.Trim().ToLowerInvariant()
  }
  if (@("light", "standard", "production") -notcontains $resolved) {
    throw "Invalid performance tier '$Tier'. Use light, standard, or production."
  }
  return $resolved
}

function Set-AetherLinkPerformanceTierEnvValues {
  param(
    [string]$Content,
    [string]$Tier
  )

  $resolved = Resolve-AetherLinkPerformanceTier $Tier
  $presets = @{
    light = @{
      AETHERLINK_POSTGRES_CPUS = "0.40"
      AETHERLINK_POSTGRES_MEM_LIMIT = "512m"
      AETHERLINK_REDIS_CPUS = "0.20"
      AETHERLINK_REDIS_MEM_LIMIT = "128m"
      AETHERLINK_MQTT_CPUS = "0.30"
      AETHERLINK_MQTT_MEM_LIMIT = "128m"
      AETHERLINK_BACKEND_CPUS = "0.70"
      AETHERLINK_BACKEND_MEM_LIMIT = "768m"
      AETHERLINK_FRONTEND_CPUS = "0.20"
      AETHERLINK_FRONTEND_MEM_LIMIT = "128m"
    }
    standard = @{
      AETHERLINK_POSTGRES_CPUS = "0.80"
      AETHERLINK_POSTGRES_MEM_LIMIT = "1g"
      AETHERLINK_REDIS_CPUS = "0.30"
      AETHERLINK_REDIS_MEM_LIMIT = "256m"
      AETHERLINK_MQTT_CPUS = "0.60"
      AETHERLINK_MQTT_MEM_LIMIT = "256m"
      AETHERLINK_BACKEND_CPUS = "1.50"
      AETHERLINK_BACKEND_MEM_LIMIT = "1536m"
      AETHERLINK_FRONTEND_CPUS = "0.30"
      AETHERLINK_FRONTEND_MEM_LIMIT = "192m"
    }
    production = @{
      AETHERLINK_POSTGRES_CPUS = "1.50"
      AETHERLINK_POSTGRES_MEM_LIMIT = "2g"
      AETHERLINK_REDIS_CPUS = "0.50"
      AETHERLINK_REDIS_MEM_LIMIT = "512m"
      AETHERLINK_MQTT_CPUS = "1.00"
      AETHERLINK_MQTT_MEM_LIMIT = "512m"
      AETHERLINK_BACKEND_CPUS = "2.50"
      AETHERLINK_BACKEND_MEM_LIMIT = "3072m"
      AETHERLINK_FRONTEND_CPUS = "0.50"
      AETHERLINK_FRONTEND_MEM_LIMIT = "256m"
    }
  }

  $content = Set-AetherLinkEnvValue $Content "AETHERLINK_PERFORMANCE_TIER" $resolved
  foreach ($entry in $presets[$resolved].GetEnumerator()) {
    $content = Set-AetherLinkEnvValue $content $entry.Key $entry.Value
  }
  return $content
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

function Request-AetherLinkServerAddresses {
  param([string]$Prompt)

  Write-Host $Prompt
  if (-not $PublicUrl) {
    $script:PublicUrl = Read-Host "Browser URL, for example http://192.168.1.10:8080"
  }
  if (-not $MqttAddress) {
    $script:MqttAddress = Read-Host "Device MQTT address, for example 192.168.1.10:1883"
  }
  if (-not $script:PublicUrl -or -not $script:MqttAddress) {
    throw "Server/private deployment needs both PublicUrl and MqttAddress. Restart and enter both values."
  }
  $script:Server = $true
}

function Resolve-AetherLinkFirstRunAddresses {
  $envPath = Join-Path $Root ".env"

  if (($PublicUrl -and -not $MqttAddress) -or ($MqttAddress -and -not $PublicUrl)) {
    Request-AetherLinkServerAddresses "Only one public address was provided. Fill the missing value before .env is updated."
    return
  }

  if ($PublicUrl -and $MqttAddress) {
    $script:Server = $true
    return
  }

  if (-not (Test-Path -LiteralPath $envPath)) {
    if ($Server) {
      Request-AetherLinkServerAddresses "No .env file was found. Enter the public addresses devices and browsers will use."
      return
    }

    Write-Host "No .env file was found. Choose how devices will reach this install before startup."
    Write-Host "  L = Local only: browser and first device run on this Windows PC."
    Write-Host "  S = Server/private deployment: browser or devices connect from another machine."
    $mode = Read-Host "Choose L or S [L]"
    if ($mode -and $mode.Trim().ToUpperInvariant().StartsWith("S")) {
      Request-AetherLinkServerAddresses "Server/private deployment selected."
      return
    }
    Write-Host "Using local-only defaults: http://localhost:8080 and localhost:1883."
    return
  }

  $currentPublicUrl = Get-AetherLinkEnvValue $envPath "AETHERLINK_PUBLIC_URL"
  $currentMqttAddress = Get-AetherLinkEnvValue $envPath "AETHERLINK_MQTT_ACCESS_ADDRESS"
  $usesLocalPublicUrl = Test-AetherLinkLocalAddress $currentPublicUrl
  $usesLocalMqttAddress = Test-AetherLinkLocalAddress $currentMqttAddress
  if (-not $usesLocalPublicUrl -and -not $usesLocalMqttAddress) {
    return
  }

  if ($Server) {
    Request-AetherLinkServerAddresses ".env exists but its public addresses still look local-only. Enter server/private addresses."
    return
  }

  Write-Host ".env already exists and its public addresses still look local-only."
  Write-Host "  Browser URL: $currentPublicUrl"
  Write-Host "  Device MQTT: $currentMqttAddress"
  Write-Host "  K = Keep local-only: browser and first device run on this Windows PC."
  Write-Host "  S = Switch to server/private addresses before startup."
  $existingMode = Read-Host "Choose K or S [K]"
  if ($existingMode -and $existingMode.Trim().ToUpperInvariant().StartsWith("S")) {
    Request-AetherLinkServerAddresses "Existing .env public addresses will be updated before startup. Secrets and volumes are kept."
  } else {
    Write-Host "Keeping existing local-only addresses from .env."
  }
}

function Initialize-AetherLinkEnvFile {
  Copy-Item ".env.example" ".env"

  $postgresPassword = New-AetherLinkSecret
  $redisPassword = New-AetherLinkSecret
  $mqttRootPassword = New-AetherLinkSecret
  $mqttPluginPassword = New-AetherLinkSecret
  $jwtKey = New-AetherLinkSecret 48
  $content = Get-Content -Raw ".env"

  $content = Set-AetherLinkEnvValue $content "POSTGRES_PASSWORD" $postgresPassword
  $content = Set-AetherLinkEnvValue $content "GOTP_DB_PSQL_PASSWORD" $postgresPassword
  $content = Set-AetherLinkEnvValue $content "REDIS_PASSWORD" $redisPassword
  $content = Set-AetherLinkEnvValue $content "GOTP_DB_REDIS_PASSWORD" $redisPassword
  $content = Set-AetherLinkEnvValue $content "MQTT_ROOT_PASSWORD" $mqttRootPassword
  $content = Set-AetherLinkEnvValue $content "MQTT_PLUGIN_PASSWORD" $mqttPluginPassword
  $content = Set-AetherLinkEnvValue $content "GOTP_MQTT_USER" "root"
  $content = Set-AetherLinkEnvValue $content "GOTP_MQTT_PASS" $mqttRootPassword
  $content = Set-AetherLinkEnvValue $content "GOTP_JWT_KEY" $jwtKey
  $content = Set-AetherLinkPerformanceTierEnvValues $content $PerformanceTier
  $content = Set-AetherLinkEnvValue $content "AETHERLINK_SERVER_MODE" $(if ($Server) { "1" } else { "0" })

  if ($PublicUrl) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_PUBLIC_URL" $PublicUrl
    $content = Set-AetherLinkEnvValue $content "GOTP_OTA_DOWNLOAD_ADDRESS" $PublicUrl
  }

  if ($MqttAddress) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_MQTT_ACCESS_ADDRESS" $MqttAddress
    $content = Set-AetherLinkEnvValue $content "GOTP_MQTT_ACCESS_ADDRESS" $MqttAddress
  }

  if ($BindAddress) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_BIND_ADDRESS" $BindAddress
  } elseif ($Server) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_BIND_ADDRESS" "0.0.0.0"
  }

  Set-Content -Path ".env" -Value $content -Encoding utf8
  Write-Host "Created .env with generated local secrets."
}

function Sync-AetherLinkAddressEnvFile {
  if (-not (Test-Path ".env")) {
    return
  }
  $content = Get-Content -Raw ".env"
  $effectivePerformanceTier = $PerformanceTier
  if (-not $effectivePerformanceTier) {
    $effectivePerformanceTier = Get-AetherLinkEnvValue ".env" "AETHERLINK_PERFORMANCE_TIER"
  }
  if (-not $effectivePerformanceTier) {
    $effectivePerformanceTier = "light"
  }
  $content = Set-AetherLinkPerformanceTierEnvValues $content $effectivePerformanceTier
  if ($Server) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_SERVER_MODE" "1"
  }

  if ($PublicUrl) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_PUBLIC_URL" $PublicUrl
    $content = Set-AetherLinkEnvValue $content "GOTP_OTA_DOWNLOAD_ADDRESS" $PublicUrl
  }
  if ($MqttAddress) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_MQTT_ACCESS_ADDRESS" $MqttAddress
    $content = Set-AetherLinkEnvValue $content "GOTP_MQTT_ACCESS_ADDRESS" $MqttAddress
  }

  if ($BindAddress) {
    $content = Set-AetherLinkEnvValue $content "AETHERLINK_BIND_ADDRESS" $BindAddress
  } elseif ($Server) {
    $currentBindAddress = Get-AetherLinkEnvValue ".env" "AETHERLINK_BIND_ADDRESS"
    if (-not $currentBindAddress -or (Test-AetherLinkLocalAddress $currentBindAddress)) {
      $content = Set-AetherLinkEnvValue $content "AETHERLINK_BIND_ADDRESS" "0.0.0.0"
    }
  }

  Set-Content -Path ".env" -Value $content -Encoding utf8
  Write-Host "Updated .env from explicit startup arguments."
}

Resolve-AetherLinkFirstRunAddresses

if (-not (Test-Path ".env")) {
  Initialize-AetherLinkEnvFile
} else {
  Sync-AetherLinkAddressEnvFile
}

$doctorParams = @{}
if ($PublicUrl) {
  $doctorParams["PublicUrl"] = $PublicUrl
}
if ($MqttAddress) {
  $doctorParams["MqttAddress"] = $MqttAddress
}
if ($PerformanceTier) {
  $doctorParams["PerformanceTier"] = Resolve-AetherLinkPerformanceTier $PerformanceTier
}
if ($Server) {
  $doctorParams["Server"] = $true
}

& (Join-Path $PSScriptRoot "doctor.ps1") @doctorParams
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

if ($Doctor) {
  Write-Host "Doctor-only mode finished. No containers were started."
  return
}

Write-Host "AetherLink IoT one-click startup"
Write-Host "Frontend: $($(if ($PublicUrl) { $PublicUrl } else { 'http://localhost:8080' }))"
Write-Host "MQTT: $($(if ($MqttAddress) { $MqttAddress } else { 'localhost:1883' }))"
$currentBindAddress = Get-AetherLinkEnvValue ".env" "AETHERLINK_BIND_ADDRESS"
Write-Host "Bind: $($(if ($currentBindAddress) { $currentBindAddress } else { '127.0.0.1' }))"
$currentPerformanceTier = Get-AetherLinkEnvValue ".env" "AETHERLINK_PERFORMANCE_TIER"
$currentPublicUrl = Get-AetherLinkEnvValue ".env" "AETHERLINK_PUBLIC_URL"
$firstDeviceUrl = Get-AetherLinkFirstDeviceUrl $currentPublicUrl
Write-Host "Performance tier: $(Resolve-AetherLinkPerformanceTier $currentPerformanceTier)"
Write-Host "Options: -Server blocks localhost public addresses; -NoBuild skips image rebuild; -SkipVerify skips startup health archive; -Doctor only runs preflight."

docker compose config | Out-Null
if ($NoBuild) {
  docker compose up -d
} else {
  docker compose up -d --build
}
docker compose ps

if (-not $SkipVerify) {
  $verifyArgs = @()
  if ($Server) {
    $verifyArgs += "-Server"
  }
  & (Join-Path $PSScriptRoot "verify.ps1") @verifyArgs
}

Write-Host ""
Write-Host "AetherLink IoT is starting."
Write-Host "Frontend: see AETHERLINK_PUBLIC_URL in .env, default http://localhost:8080"
Write-Host "MQTT: see AETHERLINK_MQTT_ACCESS_ADDRESS in .env, default localhost:1883"
Write-Host "Open: $firstDeviceUrl"
Write-Host "Next: follow 接入第一台设备: check deployment health -> generate the first device -> send the first telemetry -> download the success proof."
Write-Host "If startup is stuck, check verification/startup-*/manifest.json and rerun .\deploy\init.ps1 -Doctor."
