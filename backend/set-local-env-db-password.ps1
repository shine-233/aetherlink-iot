param(
    [string]$EnvTemplatePath = "..\\.env.example",
    [string]$EnvFilePath = "..\\.env",
    [Parameter(Mandatory = $true)]
    [string]$DbPassword
)

$ErrorActionPreference = "Stop"

function Resolve-LocalPath {
    param([string]$Path)

    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $Path
    }
    return Join-Path $PSScriptRoot $Path
}

function Set-EnvValue {
    param(
        [string]$Content,
        [string]$Name,
        [string]$Value
    )

    $pattern = "(?m)^$([regex]::Escape($Name))=.*$"
    if ($Content -match $pattern) {
        return [regex]::Replace($Content, $pattern, "${Name}=${Value}")
    }

    return $Content.TrimEnd() + "`n${Name}=${Value}`n"
}

$resolvedTemplatePath = Resolve-LocalPath -Path $EnvTemplatePath
$resolvedEnvPath = Resolve-LocalPath -Path $EnvFilePath

if (Test-Path -LiteralPath $resolvedEnvPath) {
    $content = Get-Content -LiteralPath $resolvedEnvPath -Encoding UTF8 -Raw
} else {
    if (-not (Test-Path -LiteralPath $resolvedTemplatePath)) {
        throw "Env template file not found: $resolvedTemplatePath"
    }
    $content = Get-Content -LiteralPath $resolvedTemplatePath -Encoding UTF8 -Raw
}

$content = Set-EnvValue -Content $content -Name "POSTGRES_PASSWORD" -Value $DbPassword
$content = Set-EnvValue -Content $content -Name "GOTP_DB_PSQL_PASSWORD" -Value $DbPassword

Set-Content -LiteralPath $resolvedEnvPath -Encoding UTF8 -Value $content
Write-Output "Updated $resolvedEnvPath"
