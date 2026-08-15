# AetherLink IoT Backend - local API preflight
# Usage:
#   .\local-api-preflight.ps1
#   .\local-api-preflight.ps1 -ConfigPath .\configs\conf-localdev.yml

param(
    [string]$ConfigPath = ".\configs\conf-localdev.yml",
    [string]$EnvFilePath = "..\\.env"
)

$ErrorActionPreference = "Stop"

function Read-ConfigText {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "配置文件不存在: $Path"
    }
    return Get-Content -LiteralPath $Path -Encoding UTF8
}

function Get-MqttEnabledValue {
    param([string[]]$Lines)

    $inMqttSection = $false
    foreach ($line in $Lines) {
        if ($line -match '^\s*mqtt:\s*$') {
            $inMqttSection = $true
            continue
        }
        if ($inMqttSection -and $line -match '^[A-Za-z0-9_-]+:\s*') {
            break
        }
        if ($inMqttSection -and $line -match '^\s*enabled:\s*(true|false)\s*$') {
            return $Matches[1].Trim()
        }
    }
    return "true"
}

function Get-EnvFileValue {
    param(
        [string]$Path,
        [string]$Key
    )

    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return ""
    }

    $lines = Get-Content -LiteralPath $Path -Encoding UTF8
    foreach ($line in $lines) {
        if ($line -match '^\s*#') { continue }
        if ($line -match "^\s*$([regex]::Escape($Key))=(.*)\s*$") {
            return $Matches[1].Trim().Trim('"')
        }
    }

    return ""
}

function Get-DbPasswordStatus {
    param(
        [string[]]$Lines,
        [string]$EnvFilePathValue
    )

    $passwordLine = $Lines | Where-Object { $_ -match '^\s*password:\s*' } | Select-Object -First 1
    $configUsesPlaceholder = $true
    if ($passwordLine) {
        $configUsesPlaceholder = $passwordLine -match 'CHANGE_ME_LOCALDEV_POSTGRES_PASSWORD'
    }

    $envPasswordConfigured = -not [string]::IsNullOrWhiteSpace($env:GOTP_DB_PSQL_PASSWORD)
    $envFilePassword = Get-EnvFileValue -Path $EnvFilePathValue -Key "GOTP_DB_PSQL_PASSWORD"
    $envFileConfigured = -not [string]::IsNullOrWhiteSpace($envFilePassword)
    $configPasswordConfigured = -not $configUsesPlaceholder
    $effectiveSource = if ($envPasswordConfigured) {
        "env:GOTP_DB_PSQL_PASSWORD"
    } elseif ($envFileConfigured) {
        "envfile:$EnvFilePathValue"
    } elseif ($configPasswordConfigured) {
        "config:conf-localdev.yml"
    } else {
        "missing"
    }

    return @{
        EnvConfigured = $envPasswordConfigured
        EnvFileConfigured = $envFileConfigured
        ConfigConfigured = $configPasswordConfigured
        EffectiveConfigured = ($envPasswordConfigured -or $envFileConfigured -or $configPasswordConfigured)
        EffectiveSource = $effectiveSource
    }
}

function Test-Port {
    param([int]$Port)

    try {
        return [bool](Test-NetConnection -ComputerName 127.0.0.1 -Port $Port -WarningAction SilentlyContinue | Select-Object -ExpandProperty TcpTestSucceeded)
    } catch {
        return $false
    }
}

function Test-Health {
    try {
        $resp = Invoke-WebRequest -Uri "http://127.0.0.1:9999/health" -UseBasicParsing -TimeoutSec 3
        return "HTTP $($resp.StatusCode)"
    } catch {
        return "unreachable"
    }
}

$resolvedConfigPath = if ([System.IO.Path]::IsPathRooted($ConfigPath)) { $ConfigPath } else { Join-Path $PSScriptRoot $ConfigPath }
$resolvedEnvFilePath = if ([string]::IsNullOrWhiteSpace($EnvFilePath)) {
    ""
} elseif ([System.IO.Path]::IsPathRooted($EnvFilePath)) {
    $EnvFilePath
} else {
    Join-Path $PSScriptRoot $EnvFilePath
}

$configLines = Read-ConfigText -Path $resolvedConfigPath
$mqttEnabled = Get-MqttEnabledValue -Lines $configLines
$dbPasswordStatus = Get-DbPasswordStatus -Lines $configLines -EnvFilePathValue $resolvedEnvFilePath
$mqttPortReady = Test-Port -Port 1883
$postgresReady = Test-Port -Port 5432
$redisReady = Test-Port -Port 6379
$healthResult = Test-Health

Write-Host ""
Write-Host "=== AetherLink Local API Preflight ===" -ForegroundColor Cyan
Write-Host "ConfigPath: $resolvedConfigPath"
Write-Host "mqtt.enabled: $mqttEnabled"
Write-Host "db password in config file: $($dbPasswordStatus.ConfigConfigured)"
Write-Host "db password env override: $($dbPasswordStatus.EnvConfigured)"
Write-Host "db password env file: $($dbPasswordStatus.EnvFileConfigured)"
Write-Host "db password effective source: $($dbPasswordStatus.EffectiveSource)"
Write-Host "postgres 127.0.0.1:5432: $postgresReady"
Write-Host "redis 127.0.0.1:6379: $redisReady"
Write-Host "mqtt 127.0.0.1:1883: $mqttPortReady"
Write-Host "http://127.0.0.1:9999/health: $healthResult"

Write-Host ""
Write-Host "Next action hints:" -ForegroundColor Yellow
if (-not $dbPasswordStatus.EffectiveConfigured) {
    Write-Host "- PostgreSQL password is still missing; set db.psql.password in conf-localdev.yml, export GOTP_DB_PSQL_PASSWORD, or place it in the root .env."
} elseif ($dbPasswordStatus.EnvConfigured -and -not $dbPasswordStatus.ConfigConfigured) {
    Write-Host "- PostgreSQL password will come from GOTP_DB_PSQL_PASSWORD; if auth still fails, verify that environment variable value."
} elseif ($dbPasswordStatus.EnvFileConfigured -and -not $dbPasswordStatus.EnvConfigured -and -not $dbPasswordStatus.ConfigConfigured) {
    Write-Host "- PostgreSQL password will come from the root .env file; if auth still fails, verify GOTP_DB_PSQL_PASSWORD in that file."
}
if ($mqttEnabled -eq "true" -and -not $mqttPortReady) {
    Write-Host "- mqtt.enabled=true but 1883 is unavailable; start a local broker or set mqtt.enabled=false for API-only validation."
}
if (-not $postgresReady) {
    Write-Host "- PostgreSQL port is unavailable; verify the local PostgreSQL service and listener."
}
if (-not $redisReady) {
    Write-Host "- Redis port is unavailable; verify the local Redis service."
}
if ($healthResult -eq "unreachable" -and $dbPasswordStatus.EffectiveConfigured -and $postgresReady -and $redisReady -and ($mqttEnabled -eq "false" -or $mqttPortReady)) {
    Write-Host "- Dependency ports look ready but health is still down; next run: go run main.go -config $resolvedConfigPath"
}
