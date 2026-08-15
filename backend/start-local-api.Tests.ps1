Describe "start-local-api.ps1" {
    $scriptPath = Join-Path $PSScriptRoot "start-local-api.ps1"
    $configPath = Join-Path $PSScriptRoot "configs\\conf-localdev.yml"
    $tempEnvPath = Join-Path $PSScriptRoot "tmp-start-local-api.env"

    It "prints the startup command when a db password argument is provided" {
        $oldEnv = $env:GOTP_DB_PSQL_PASSWORD
        try {
            Remove-Item Env:GOTP_DB_PSQL_PASSWORD -ErrorAction SilentlyContinue
            $output = & $scriptPath -ConfigPath $configPath -DbPassword "temp-test-password" -PrintCommandOnly -NoPrompt 2>&1
            $LASTEXITCODE | Should Be 0
            ($output -join "`n") | Should Match "env:GOTP_DB_PSQL_PASSWORD"
            ($output -join "`n") | Should Match "go run main.go -config"
        } finally {
            $env:GOTP_DB_PSQL_PASSWORD = $oldEnv
        }
    }

    It "fails fast when no db password source is available and prompting is disabled" {
        $oldEnv = $env:GOTP_DB_PSQL_PASSWORD
        try {
            Remove-Item Env:GOTP_DB_PSQL_PASSWORD -ErrorAction SilentlyContinue
            { & $scriptPath -ConfigPath $configPath -PrintCommandOnly -NoPrompt } | Should Throw "No PostgreSQL password source available. Provide -DbPassword, export GOTP_DB_PSQL_PASSWORD, or set db.psql.password in conf-localdev.yml."
        } finally {
            $env:GOTP_DB_PSQL_PASSWORD = $oldEnv
        }
    }

    It "returns a non-zero process exit code when no db password source is available" {
        $oldEnv = $env:GOTP_DB_PSQL_PASSWORD
        try {
            Remove-Item Env:GOTP_DB_PSQL_PASSWORD -ErrorAction SilentlyContinue
            $output = powershell -ExecutionPolicy Bypass -File $scriptPath -ConfigPath $configPath -PrintCommandOnly -NoPrompt 2>&1
            $LASTEXITCODE | Should Not Be 0
            ($output -join "`n") | Should Match "No PostgreSQL password source available"
        } finally {
            $env:GOTP_DB_PSQL_PASSWORD = $oldEnv
        }
    }

    It "uses GOTP_DB_PSQL_PASSWORD from an env file when process env is empty" {
        $oldEnv = $env:GOTP_DB_PSQL_PASSWORD
        try {
            Remove-Item Env:GOTP_DB_PSQL_PASSWORD -ErrorAction SilentlyContinue
            Set-Content -LiteralPath $tempEnvPath -Encoding UTF8 -Value "GOTP_DB_PSQL_PASSWORD=from-env-file"
            $output = & $scriptPath -ConfigPath $configPath -EnvFilePath $tempEnvPath -PrintCommandOnly -NoPrompt 2>&1
            $LASTEXITCODE | Should Be 0
            ($output -join "`n") | Should Match "envfile:"
            ($output -join "`n") | Should Match "go run main.go -config"
        } finally {
            Remove-Item -LiteralPath $tempEnvPath -ErrorAction SilentlyContinue
            $env:GOTP_DB_PSQL_PASSWORD = $oldEnv
        }
    }

    It "propagates a non-zero exit code from the backend process" {
        $oldPath = $env:PATH
        $tempGoDir = Join-Path ([System.IO.Path]::GetTempPath()) "aetherlink-start-local-api-go-$PID"
        try {
            New-Item -ItemType Directory -Path $tempGoDir -Force | Out-Null
            Set-Content -LiteralPath (Join-Path $tempGoDir "go.cmd") -Encoding ASCII -Value @(
                "@echo off",
                "exit /b 23"
            )
            $env:PATH = "$tempGoDir;$oldPath"

            $output = powershell -ExecutionPolicy Bypass -File $scriptPath `
                -ConfigPath $configPath -DbPassword "temp-test-password" -NoPrompt -SkipPreflight 2>&1

            $LASTEXITCODE | Should Be 23
            ($output -join "`n") | Should Match "startup command"
        } finally {
            $env:PATH = $oldPath
            Remove-Item -LiteralPath $tempGoDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}
