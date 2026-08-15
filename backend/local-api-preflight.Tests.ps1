Describe "local-api-preflight.ps1" {
    $scriptPath = Join-Path $PSScriptRoot "local-api-preflight.ps1"
    $configPath = Join-Path $PSScriptRoot "configs\\conf-localdev.yml"
    $tempEnvPath = Join-Path $PSScriptRoot "tmp-local-api-preflight.env"

    It "reports envfile db password source when GOTP_DB_PSQL_PASSWORD exists in an env file" {
        $oldEnv = $env:GOTP_DB_PSQL_PASSWORD
        try {
            Remove-Item Env:GOTP_DB_PSQL_PASSWORD -ErrorAction SilentlyContinue
            Set-Content -LiteralPath $tempEnvPath -Encoding UTF8 -Value "GOTP_DB_PSQL_PASSWORD=from-env-file"
            $output = powershell -ExecutionPolicy Bypass -File $scriptPath -ConfigPath $configPath -EnvFilePath $tempEnvPath 2>&1
            ($output -join "`n") | Should Match "db password effective source: envfile:"
        } finally {
            Remove-Item -LiteralPath $tempEnvPath -ErrorAction SilentlyContinue
            $env:GOTP_DB_PSQL_PASSWORD = $oldEnv
        }
    }
}
