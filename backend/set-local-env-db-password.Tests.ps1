Describe "set-local-env-db-password.ps1" {
    $scriptPath = Join-Path $PSScriptRoot "set-local-env-db-password.ps1"
    $tempRoot = Join-Path $PSScriptRoot "tmp-set-local-env-db-password"
    $templatePath = Join-Path $tempRoot ".env.example"
    $envPath = Join-Path $tempRoot ".env"

    BeforeEach {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Path $tempRoot | Out-Null
        Set-Content -LiteralPath $templatePath -Encoding UTF8 -Value @"
POSTGRES_PASSWORD=change_me_postgres_password
GOTP_DB_PSQL_PASSWORD=change_me_postgres_password
OTHER_VALUE=keep_me
"@
    }

    AfterEach {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }

    It "creates .env from template and writes both postgres password keys" {
        $output = & $scriptPath -EnvTemplatePath $templatePath -EnvFilePath $envPath -DbPassword "local-secret" 2>&1
        $LASTEXITCODE | Should Be 0
        Test-Path -LiteralPath $envPath | Should Be $true
        $content = Get-Content -LiteralPath $envPath -Encoding UTF8 -Raw
        $content | Should Match "POSTGRES_PASSWORD=local-secret"
        $content | Should Match "GOTP_DB_PSQL_PASSWORD=local-secret"
        $content | Should Match "OTHER_VALUE=keep_me"
        ($output -join "`n") | Should Match "Updated"
    }

    It "updates an existing .env without overwriting unrelated values" {
        Set-Content -LiteralPath $envPath -Encoding UTF8 -Value @"
POSTGRES_PASSWORD=old-one
GOTP_DB_PSQL_PASSWORD=old-two
OTHER_VALUE=keep_me
"@

        & $scriptPath -EnvTemplatePath $templatePath -EnvFilePath $envPath -DbPassword "new-secret" | Out-Null

        $content = Get-Content -LiteralPath $envPath -Encoding UTF8 -Raw
        $content | Should Match "POSTGRES_PASSWORD=new-secret"
        $content | Should Match "GOTP_DB_PSQL_PASSWORD=new-secret"
        $content | Should Match "OTHER_VALUE=keep_me"
    }

    It "fails when the template file is missing and .env does not exist" {
        Remove-Item -LiteralPath $templatePath -Force
        { & $scriptPath -EnvTemplatePath $templatePath -EnvFilePath $envPath -DbPassword "local-secret" } | Should Throw "Env template file not found: $templatePath"
    }
}
