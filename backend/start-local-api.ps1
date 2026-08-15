# AetherLink IoT Backend - local API launcher
# Usage examples:
#   .\start-local-api.ps1 -DbPassword "local-password"
#   .\start-local-api.ps1 -PrintCommandOnly -DbPassword "local-password"
#   $env:GOTP_DB_PSQL_PASSWORD="local-password"; .\start-local-api.ps1

param(
    [string]$ConfigPath = ".\configs\conf-localdev.yml",
    [string]$EnvFilePath = "..\\.env",
    [string]$DbPassword = "",
    [switch]$NoPrompt,
    [switch]$PrintCommandOnly,
    [switch]$SkipPreflight
)

$ErrorActionPreference = "Stop"

function Read-ConfigText {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "配置文件不存在: $Path"
    }
    return Get-Content -LiteralPath $Path -Encoding UTF8
}

function Get-ConfigDbPasswordValue {
    param([string[]]$Lines)

    $inDbSection = $false
    $inPsqlSection = $false

    foreach ($line in $Lines) {
        if ($line -match '^\s*db:\s*$') {
            $inDbSection = $true
            continue
        }
        if ($inDbSection -and -not $inPsqlSection -and $line -match '^\s*psql:\s*$') {
            $inPsqlSection = $true
            continue
        }
        if ($inPsqlSection -and $line -match '^\s{2}[A-Za-z0-9_-]+:\s*') {
            break
        }
        if ($inPsqlSection -and $line -match '^\s*password:\s*(.+?)\s*$') {
            return $Matches[1].Trim().Trim('"')
        }
    }

    return ""
}

function ConvertTo-PlainText {
    param([System.Security.SecureString]$SecureValue)

    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureValue)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    } finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
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

function Resolve-DbPasswordSource {
    param(
        [string]$ArgumentPassword,
        [string[]]$ConfigLines,
        [string]$EnvFilePathValue,
        [switch]$DisablePrompt
    )

    if (-not [string]::IsNullOrWhiteSpace($ArgumentPassword)) {
        return @{
            Password = $ArgumentPassword
            Source = "argument"
            ExportEnv = $true
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($env:GOTP_DB_PSQL_PASSWORD)) {
        return @{
            Password = $env:GOTP_DB_PSQL_PASSWORD
            Source = "env:GOTP_DB_PSQL_PASSWORD"
            ExportEnv = $false
        }
    }

    $envFilePassword = Get-EnvFileValue -Path $EnvFilePathValue -Key "GOTP_DB_PSQL_PASSWORD"
    if (-not [string]::IsNullOrWhiteSpace($envFilePassword)) {
        return @{
            Password = $envFilePassword
            Source = "envfile:$EnvFilePathValue"
            ExportEnv = $true
        }
    }

    $configPassword = Get-ConfigDbPasswordValue -Lines $ConfigLines
    if (-not [string]::IsNullOrWhiteSpace($configPassword) -and $configPassword -notmatch '^CHANGE_ME_') {
        return @{
            Password = $configPassword
            Source = "config:conf-localdev.yml"
            ExportEnv = $false
        }
    }

    if ($DisablePrompt) {
        return @{
            Password = ""
            Source = "missing"
            ExportEnv = $false
        }
    }

    $securePassword = Read-Host "Enter local PostgreSQL password for GOTP_DB_PSQL_PASSWORD" -AsSecureString
    $plainPassword = ConvertTo-PlainText -SecureValue $securePassword
    if ([string]::IsNullOrWhiteSpace($plainPassword)) {
        return @{
            Password = ""
            Source = "missing"
            ExportEnv = $false
        }
    }

    return @{
        Password = $plainPassword
        Source = "prompt"
        ExportEnv = $true
    }
}

function Write-MissingPasswordError {
    throw "No PostgreSQL password source available. Provide -DbPassword, export GOTP_DB_PSQL_PASSWORD, or set db.psql.password in conf-localdev.yml."
}

$resolvedConfigPath = if ([System.IO.Path]::IsPathRooted($ConfigPath)) {
    $ConfigPath
} else {
    Join-Path $PSScriptRoot $ConfigPath
}
$resolvedEnvFilePath = if ([string]::IsNullOrWhiteSpace($EnvFilePath)) {
    ""
} elseif ([System.IO.Path]::IsPathRooted($EnvFilePath)) {
    $EnvFilePath
} else {
    Join-Path $PSScriptRoot $EnvFilePath
}

$configLines = Read-ConfigText -Path $resolvedConfigPath
$passwordStatus = Resolve-DbPasswordSource -ArgumentPassword $DbPassword -ConfigLines $configLines -EnvFilePathValue $resolvedEnvFilePath -DisablePrompt:$NoPrompt

if ([string]::IsNullOrWhiteSpace($passwordStatus.Password)) {
    Write-MissingPasswordError
    return
}

if ($passwordStatus.ExportEnv) {
    $env:GOTP_DB_PSQL_PASSWORD = $passwordStatus.Password
}

$commandText = "go run main.go -config $resolvedConfigPath"
$effectiveSource = if ($passwordStatus.Source -eq "argument" -or $passwordStatus.Source -eq "prompt") {
    "env:GOTP_DB_PSQL_PASSWORD"
} else {
    $passwordStatus.Source
}

Write-Output "db password effective source: $effectiveSource"
Write-Output "startup command: $commandText"

if ($PrintCommandOnly) {
    $global:LASTEXITCODE = 0
    return
}

if (-not $SkipPreflight) {
    & powershell -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "local-api-preflight.ps1") -ConfigPath $resolvedConfigPath
}

Push-Location $PSScriptRoot
$goExitCode = 1
try {
    & go run main.go -config $resolvedConfigPath
    if ($? -and $LASTEXITCODE -eq 0) {
        $goExitCode = 0
    } elseif ($null -ne $LASTEXITCODE) {
        $goExitCode = [int]$LASTEXITCODE
    }
} finally {
    Pop-Location
}
exit $goExitCode
