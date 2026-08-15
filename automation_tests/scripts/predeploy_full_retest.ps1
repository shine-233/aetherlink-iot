<#
Purpose: run one isolated, release-style API/E2E retest without sharing the
default MQTT port, automation reports, auth state, or local .env.local.

The PostgreSQL password is read only from the current process environment as
AETHERLINK_PREFLIGHT_DB_PASSWORD. It is never accepted as a command-line
argument and is never written to reports, manifests, or generated files. The
explicit isolated database is created only when it is missing; this script
never drops or overwrites an existing database.
#>

[CmdletBinding()]
param(
    [string]$ProjectRoot = '',
    [string]$RunDir = '',
    [string]$DatabaseName = 'aetherlink_iot_predeploy_retest_20260814_r9d',
    [Parameter(Mandatory = $true)]
    [string]$AccountSource,
    [Parameter(Mandatory = $true)]
    [string]$BrokerDirectory,
    [Parameter(Mandatory = $true)]
    [string]$BackendBinary,
    [Parameter(Mandatory = $true)]
    [string]$BrokerBinary,
    [Parameter(Mandatory = $true)]
    [string]$EmulatorBinary,
    [string]$PreviewDistDir = '',
    [string]$ReadyCheckEmulatorBinary = '',
    [int]$BrokerPort = 11086,
    [int]$BackendPort = 19999
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($ProjectRoot)) {
    $ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
}
$ProjectRoot = [System.IO.Path]::GetFullPath($ProjectRoot)
Set-Location -LiteralPath $ProjectRoot

function Resolve-RunPath {
    param([string]$Value)
    if ([System.IO.Path]::IsPathRooted($Value)) {
        return [System.IO.Path]::GetFullPath($Value)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot $Value))
}

function Get-EnvValue {
    param([string]$Name)
    $item = Get-ChildItem Env: -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -eq $Name } |
        Select-Object -First 1
    if ($null -ne $item) {
        return [string]$item.Value
    }
    return ''
}

function Set-ChildEnvironment {
    param(
        [hashtable]$Environment,
        [string[]]$Names
    )
    $result = @{}
    foreach ($name in $Names) {
        $result[$name] = Get-EnvValue $name
    }
    return $result
}

function Wait-TcpPort {
    param([int]$Port, [int]$Attempts = 120)
    for ($index = 0; $index -lt $Attempts; $index++) {
        if (Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "TCP port $Port did not become ready"
}

function Wait-TcpPortClosed {
    param([int]$Port, [int]$Attempts = 60)
    for ($index = 0; $index -lt $Attempts; $index++) {
        if (-not (Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue)) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "TCP port $Port did not close"
}

function Wait-Health {
    param([string]$Url, [int]$Attempts = 160)
    for ($index = 0; $index -lt $Attempts; $index++) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                return
            }
        } catch {
        }
        Start-Sleep -Milliseconds 500
    }
    throw "HTTP health endpoint did not become ready: $Url"
}

function Invoke-NodeChecked {
    param(
        [string]$Script,
        [string[]]$Arguments,
        [string]$StdoutPath,
        [string]$StderrPath
    )
    $previousErrorActionPreference = $ErrorActionPreference
    try {
        # Node's diagnostic warnings are intentionally captured in the
        # per-step stderr file. Only the process exit code is a failure gate;
        # PowerShell must not turn a harmless console.warn into an exception.
        $ErrorActionPreference = 'Continue'
        & node $Script @Arguments 1> $StdoutPath 2> $StderrPath
        $nodeExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($nodeExitCode -ne 0) {
        throw "$Script failed with exit code $nodeExitCode"
    }
}

function Ensure-IsolatedDatabase {
    param(
        [string]$Name,
        [string]$PsqlPath,
        [string]$ProvisionLogPath
    )

    if ($Name -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        throw "DatabaseName must be a simple PostgreSQL identifier: $Name"
    }
    if (-not (Test-Path -LiteralPath $PsqlPath)) {
        throw "psql executable is missing: $PsqlPath"
    }

    $checkSql = "SELECT 1 FROM pg_database WHERE datname = '$Name';"
    $previousErrorActionPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $existing = & $PsqlPath -X -v ON_ERROR_STOP=1 `
            -h $env:GOTP_DB_PSQL_HOST -p $env:GOTP_DB_PSQL_PORT `
            -U $env:GOTP_DB_PSQL_USERNAME -d 'postgres' -At -q -c $checkSql `
            2>> $ProvisionLogPath
        $checkExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($checkExitCode -ne 0) {
        throw "Could not inspect PostgreSQL databases before the isolated retest: $Name"
    }

    if (($existing -join '').Trim() -eq '1') {
        return [pscustomobject]@{
            database = $Name
            status = 'existing'
            created = $false
        }
    }

    $createSql = 'CREATE DATABASE "' + $Name + '";'
    try {
        $ErrorActionPreference = 'Continue'
        & $PsqlPath -X -v ON_ERROR_STOP=1 `
            -h $env:GOTP_DB_PSQL_HOST -p $env:GOTP_DB_PSQL_PORT `
            -U $env:GOTP_DB_PSQL_USERNAME -d 'postgres' -q -c $createSql `
            2>> $ProvisionLogPath
        $createExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($createExitCode -ne 0) {
        throw "Could not create isolated PostgreSQL database: $Name"
    }

    return [pscustomobject]@{
        database = $Name
        status = 'created'
        created = $true
    }
}

$dbPassword = Get-EnvValue 'AETHERLINK_PREFLIGHT_DB_PASSWORD'
if ([string]::IsNullOrWhiteSpace($dbPassword)) {
    throw 'AETHERLINK_PREFLIGHT_DB_PASSWORD must be set in the current process environment'
}

$runPath = Resolve-RunPath $RunDir
$brokerPath = Resolve-RunPath $BrokerDirectory
$backendPath = Resolve-RunPath $BackendBinary
$brokerBinaryPath = Resolve-RunPath $BrokerBinary
$emulatorPath = Resolve-RunPath $EmulatorBinary
$readyCheckEmulatorPath = Resolve-RunPath (Join-Path $ProjectRoot 'backend\cmd\aetherlink-device-autotest\_localrun\ready-check-command-emulator.exe')
if (-not [string]::IsNullOrWhiteSpace($ReadyCheckEmulatorBinary)) {
    $readyCheckEmulatorPath = Resolve-RunPath $ReadyCheckEmulatorBinary
}
$previewDistPath = if ([string]::IsNullOrWhiteSpace($PreviewDistDir)) {
    Resolve-RunPath 'frontend\dist'
} else {
    Resolve-RunPath $PreviewDistDir
}
$cleanupExecutablePaths = @($backendPath, $brokerBinaryPath, $emulatorPath) |
    Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
    ForEach-Object { [System.IO.Path]::GetFullPath($_) }
$cleanupExecutableNames = @($backendPath, $brokerBinaryPath, $emulatorPath) |
    Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
    ForEach-Object { [System.IO.Path]::GetFileName($_) } |
    Select-Object -Unique
$accountSourcePath = Resolve-RunPath $AccountSource
$automationPath = Join-Path $ProjectRoot 'automation_tests'
$accountPath = Join-Path $runPath 'account'
$reportPath = Join-Path $runPath 'reports'
$verificationPath = Join-Path $runPath 'verification'
$authPath = Join-Path $runPath 'auth'

foreach ($path in @($runPath, $accountPath, $reportPath, $verificationPath, $authPath)) {
    New-Item -ItemType Directory -Path $path -Force | Out-Null
}

foreach ($path in @($brokerBinaryPath, $backendPath, $emulatorPath, $accountSourcePath)) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Required retest input is missing: $path"
    }
}
if (-not (Test-Path -LiteralPath (Join-Path $previewDistPath 'index.html'))) {
    throw "PreviewDistDir must contain index.html: $previewDistPath"
}

$accountKeys = @(
    'SUPER_ADMIN_EMAIL', 'SUPER_ADMIN_PASSWORD',
    'TENANT_ADMIN_EMAIL', 'TENANT_ADMIN_PASSWORD',
    'TENANT_ADMIN_B_EMAIL', 'TENANT_ADMIN_B_PASSWORD',
    'TENANT_USER_EMAIL', 'TENANT_USER_PASSWORD',
    'READONLY_USER_EMAIL', 'READONLY_USER_PASSWORD',
    'EMAIL_CHANGE_TENANT_EMAIL', 'EMAIL_CHANGE_TENANT_PASSWORD'
)
foreach ($line in (Get-Content -LiteralPath $accountSourcePath)) {
    $trimmed = $line.Trim()
    if ([string]::IsNullOrWhiteSpace($trimmed) -or $trimmed.StartsWith('#')) {
        continue
    }
    # Accept both dotenv account sources (KEY=value) and the PowerShell
    # automation-env.ps1 form ($env:KEY = "value").  The latter is already
    # generated by prepare_local_accounts.js, so rejecting it here silently
    # drops the credentials and produces a misleading "super admin exists"
    # failure against a seeded isolated database.
    $match = [regex]::Match($trimmed, '^(?:\$env:)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$')
    if (-not $match.Success -or $accountKeys -notcontains $match.Groups[1].Value) {
        continue
    }
    $value = $match.Groups[2].Value.Trim()
    if (($value.StartsWith('"') -and $value.EndsWith('"')) -or
        ($value.StartsWith("'") -and $value.EndsWith("'"))) {
        $value = $value.Substring(1, $value.Length - 2)
    }
    Set-Item -Path ('Env:' + $match.Groups[1].Value) -Value $value
}

$fixturePid = 'SYN' + (Get-Date -Format 'yyMMddHH') + ([guid]::NewGuid().ToString('N').Substring(0, 1).ToUpperInvariant())
if ($fixturePid.Length -ne 12) {
    throw "Generated synthetic PID has unexpected length: $($fixturePid.Length)"
}

$env:GOTP_DB_PSQL_HOST = '127.0.0.1'
$env:GOTP_DB_PSQL_PORT = '5432'
$env:GOTP_DB_PSQL_DBNAME = $DatabaseName
$env:GOTP_DB_PSQL_USERNAME = 'postgres'
$env:GOTP_DB_PSQL_PASSWORD = $dbPassword
$env:AETHERLINK_DB_HOST = '127.0.0.1'
$env:AETHERLINK_DB_PORT = '5432'
$env:AETHERLINK_DB_NAME = $DatabaseName
$env:AETHERLINK_DB_USER = 'postgres'
$env:AETHERLINK_DB_PASSWORD = $dbPassword
$env:PGPASSWORD = $dbPassword
$env:PGUSER = 'postgres'
$env:PGHOST = '127.0.0.1'
$env:PGPORT = '5432'
$env:PGDATABASE = $DatabaseName
$env:AETHERLINK_STRICT_DB_TARGET = '1'
$env:AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES = $DatabaseName
$env:AETHERLINK_SYNTHETIC_RDI_ALLOWED_PORT = '5432'
$env:AETHERLINK_SYNTHETIC_RDI_ALLOW = '1'
$env:GOTP_DB_REDIS_ADDR = '127.0.0.1:6379'
$env:GOTP_DB_REDIS_DB = '11'
$env:GOTP_DB_REDIS_PASSWORD = ''
$env:GOTP_SERVICE_HTTP_HOST = '127.0.0.1'
$env:GOTP_SERVICE_HTTP_PORT = [string]$BackendPort
$env:GOTP_DEPLOYMENT_PUBLIC_URL = "http://127.0.0.1:$BackendPort"
$env:GOTP_JWT_KEY = 'predeploy-r9d-jwt-' + [guid]::NewGuid().ToString('N')
$env:GOTP_MARKET_ENABLED = 'false'
$env:GOTP_GRPC_TPTODB_TYPE = 'NONE'
$env:GOTP_MQTT_ENABLED = 'true'
$env:GOTP_MQTT_ACCESS_ADDRESS = "127.0.0.1:$BrokerPort"
$env:GOTP_MQTT_BROKER = "127.0.0.1:$BrokerPort"
$env:GOTP_MQTT_USER = 'root'
$env:GOTP_MQTT_PASS = 'predeploy-r9d-mqtt-' + [guid]::NewGuid().ToString('N')
$env:GOTP_MQTT_CLIENT_ID = 'aetherlink-predeploy-r9d-backend'
$env:GOTP_UPLINK_ENABLE = 'true'
$env:GOTP_MQTT_SESSION_REVOCATIONS_BROKER_ID = 'predeploy-retest-r9d'
$env:GMQTT_DB_PSQL_PSQLADDR = '127.0.0.1'
$env:GMQTT_DB_PSQL_PSQLPORT = '5432'
$env:GMQTT_DB_PSQL_PSQLUSER = 'postgres'
$env:GMQTT_DB_PSQL_PSQLPASS = $dbPassword
$env:GMQTT_DB_PSQL_PSQLDB = $DatabaseName
$env:GMQTT_DB_PSQL_SSLMODE = 'disable'
$env:GMQTT_DB_REDIS_CONN = '127.0.0.1:6379'
$env:GMQTT_DB_REDIS_DB_NUM = '11'
$env:GMQTT_DB_REDIS_PASSWORD = ''
$env:GMQTT_MQTT_BROKER = "tcp://127.0.0.1:$BrokerPort"
$env:GMQTT_MQTT_PASSWORD = $env:GOTP_MQTT_PASS
$env:GMQTT_MQTT_PLUGIN_PASSWORD = $env:GOTP_MQTT_PASS
$env:GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID = 'predeploy-retest-r9d'
$env:AETHERLINK_RDI_FIXTURE_MODE = 'synthetic-rdi'
$env:AETHERLINK_RDI_FIXTURE_PID = $fixturePid
$env:SYNTHETIC_RDI_PID = $fixturePid
$env:SYNTHETIC_RDI_DEVICE_ID = ''
$env:SYNTHETIC_RDI_BROKER = "127.0.0.1:$BrokerPort"
$env:SYNTHETIC_RDI_EMULATOR_BIN = $emulatorPath
$env:SYNTHETIC_RDI_REPORT_DIR = Join-Path $runPath 'synthetic-rdi'
$env:AETHERLINK_PSQL_PATH = 'C:\Program Files\PostgreSQL\17\bin\psql.exe'
$databaseProvision = Ensure-IsolatedDatabase `
    -Name $DatabaseName `
    -PsqlPath $env:AETHERLINK_PSQL_PATH `
    -ProvisionLogPath (Join-Path $runPath 'database-provision.stderr.log')
$databaseProvision |
    ConvertTo-Json -Compress |
    Set-Content -LiteralPath (Join-Path $runPath 'database-provision.json') -Encoding UTF8
$env:API_BASE_URL = "http://127.0.0.1:$BackendPort/api/v1"
$env:API_TARGET = "http://127.0.0.1:$BackendPort"
$env:HEALTH_URL = "http://127.0.0.1:$BackendPort/health"
$env:FRONTEND_URL = 'http://127.0.0.1:9725'
$env:PREVIEW_URL = 'http://127.0.0.1:9725'
$env:PREFLIGHT_PROFILE = 'full'
$env:PREVIEW_PORT = '9725'
$env:PREVIEW_PROXY_HOST = '127.0.0.1'
$env:PREVIEW_PROXY_PORT = '9725'
$env:PREVIEW_DIST_DIR = $previewDistPath
$env:PLAYWRIGHT_USE_PREVIEW_PROXY = '1'
$env:PLAYWRIGHT_REUSE_EXISTING_SERVER = '0'
$env:PLAYWRIGHT_BROWSER_CHANNEL = 'msedge'
$env:AETHERLINK_RUNTIME_CONFIG_SKIP_ENV_FILE = '1'
$env:AUTOMATION_REPORT_DIR = $reportPath
$env:AUTOMATION_VERIFICATION_DIR = $verificationPath
$env:E2E_AUTH_DIR = $authPath
$env:AUTOMATION_ACCOUNT_DIR = $accountPath
$env:AUTOMATION_MQTT_SERVER = '127.0.0.1'
$env:AUTOMATION_MQTT_PORT = [string]$BrokerPort
# Ready Check uses the generic command emulator contract (-config plus
# -mode command-emulator).  The synthetic RDI protocol emulator has a
# deliberately different CLI and must never be passed here.
if (Test-Path -LiteralPath $readyCheckEmulatorPath) {
    $env:AUTOMATION_READY_CHECK_EMULATOR_BIN = $readyCheckEmulatorPath
    $env:AUTOMATION_READY_CHECK_BUILD_EMULATOR = '0'
} else {
    Remove-Item Env:AUTOMATION_READY_CHECK_EMULATOR_BIN -ErrorAction SilentlyContinue
    $env:AUTOMATION_READY_CHECK_BUILD_EMULATOR = '1'
}

$brokerEnvironment = Set-ChildEnvironment @{} @(
    'GMQTT_DB_PSQL_PSQLADDR', 'GMQTT_DB_PSQL_PSQLPORT', 'GMQTT_DB_PSQL_PSQLUSER',
    'GMQTT_DB_PSQL_PSQLPASS', 'GMQTT_DB_PSQL_PSQLDB', 'GMQTT_DB_PSQL_SSLMODE',
    'GMQTT_DB_REDIS_CONN', 'GMQTT_DB_REDIS_DB_NUM', 'GMQTT_DB_REDIS_PASSWORD',
    'GMQTT_MQTT_BROKER', 'GMQTT_MQTT_PASSWORD', 'GMQTT_MQTT_PLUGIN_PASSWORD',
    'GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID'
)

# The checked-in/staged GMQTT config carries a safe default listener port, but
# this runner also accepts an isolated -BrokerPort. Materialize a per-run
# broker directory and rewrite only the MQTT listener there so the command-line
# port, broker config, backend target, and automation client cannot drift.
$brokerRuntimePath = Join-Path $runPath 'broker-runtime'
New-Item -ItemType Directory -Path $brokerRuntimePath -Force | Out-Null
foreach ($brokerFile in @('gmqttd.exe', 'gmqttd.yml', 'aetherlink.yml')) {
    Copy-Item -LiteralPath (Join-Path $brokerPath $brokerFile) `
        -Destination (Join-Path $brokerRuntimePath $brokerFile) -Force
}
$brokerConfigPath = Join-Path $brokerRuntimePath 'gmqttd.yml'
$brokerConfig = Get-Content -LiteralPath $brokerConfigPath -Raw
$brokerListenerReplacement = '${1}' + $BrokerPort + '${2}'
$brokerConfig = $brokerConfig -replace `
    '(?m)(listeners:\s*-\s*address:\s*"127\.0\.0\.1:)\d+("\s*)', `
    $brokerListenerReplacement
Set-Content -LiteralPath $brokerConfigPath -Value $brokerConfig -Encoding UTF8

$backendEnvironment = Set-ChildEnvironment @{} @(
    'GOTP_DB_PSQL_HOST', 'GOTP_DB_PSQL_PORT', 'GOTP_DB_PSQL_DBNAME', 'GOTP_DB_PSQL_USERNAME',
    'GOTP_DB_PSQL_PASSWORD', 'GOTP_DB_REDIS_ADDR', 'GOTP_DB_REDIS_DB', 'GOTP_DB_REDIS_PASSWORD',
    'GOTP_SERVICE_HTTP_HOST', 'GOTP_SERVICE_HTTP_PORT', 'GOTP_DEPLOYMENT_PUBLIC_URL',
    'GOTP_JWT_KEY', 'GOTP_MARKET_ENABLED', 'GOTP_GRPC_TPTODB_TYPE', 'GOTP_MQTT_ENABLED',
    'GOTP_MQTT_ACCESS_ADDRESS', 'GOTP_MQTT_BROKER', 'GOTP_MQTT_USER', 'GOTP_MQTT_PASS',
    'GOTP_MQTT_CLIENT_ID', 'GOTP_UPLINK_ENABLE', 'GOTP_MQTT_SESSION_REVOCATIONS_BROKER_ID'
)
$previewEnvironment = Set-ChildEnvironment @{} @(
    'PREVIEW_PROXY_HOST', 'PREVIEW_PROXY_PORT', 'PREVIEW_DIST_DIR',
    'API_TARGET', 'THINGSVIS_API_TARGET', 'VITE_THINGSVIS_API_URL'
)

$brokerJob = $null
$backendJob = $null
$previewJob = $null
$exitCode = 1
try {
    if (Get-NetTCPConnection -LocalPort $BrokerPort -State Listen -ErrorAction SilentlyContinue) {
        throw "$BrokerPort is already listening"
    }
    if (Get-NetTCPConnection -LocalPort $BackendPort -State Listen -ErrorAction SilentlyContinue) {
        throw "$BackendPort is already listening"
    }

    $brokerJob = Start-Job -ScriptBlock {
        param($dir, $exe, $runtimeEnvironment, $stdoutPath, $stderrPath)
        Set-Location -LiteralPath $dir
        foreach ($item in $runtimeEnvironment.GetEnumerator()) {
            Set-Item -Path ('Env:' + $item.Key) -Value ([string]$item.Value)
        }
        & $exe start -c gmqttd.yml 1> $stdoutPath 2> $stderrPath
    } -ArgumentList $brokerRuntimePath, (Join-Path $brokerRuntimePath 'gmqttd.exe'), $brokerEnvironment,
        (Join-Path $runPath 'gmqtt.stdout.log'), (Join-Path $runPath 'gmqtt.stderr.log')
    Wait-TcpPort -Port $BrokerPort

    $backendJob = Start-Job -ScriptBlock {
        param($dir, $exe, $runtimeEnvironment, $stdoutPath, $stderrPath)
        Set-Location -LiteralPath $dir
        foreach ($item in $runtimeEnvironment.GetEnumerator()) {
            Set-Item -Path ('Env:' + $item.Key) -Value ([string]$item.Value)
        }
        & $exe -config '.\configs\conf-localdev.yml' 1> $stdoutPath 2> $stderrPath
    } -ArgumentList (Join-Path $ProjectRoot 'backend'), $backendPath, $backendEnvironment,
        (Join-Path $runPath 'backend.stdout.log'), (Join-Path $runPath 'backend.stderr.log')
    Wait-Health -Url $env:HEALTH_URL

    $previewJob = Start-Job -ScriptBlock {
        param($dir, $script, $runtimeEnvironment, $stdoutPath, $stderrPath)
        Set-Location -LiteralPath $dir
        foreach ($item in $runtimeEnvironment.GetEnumerator()) {
            Set-Item -Path ('Env:' + $item.Key) -Value ([string]$item.Value)
        }
        & node $script 1> $stdoutPath 2> $stderrPath
    } -ArgumentList $ProjectRoot, (Join-Path $automationPath 'scripts\serve_preview_with_api_proxy.js'), $previewEnvironment,
        (Join-Path $runPath 'preview.stdout.log'), (Join-Path $runPath 'preview.stderr.log')
    Wait-TcpPort -Port 9725

    Invoke-NodeChecked `
        -Script (Join-Path $automationPath 'scripts\prepare_local_accounts.js') `
        -Arguments @() `
        -StdoutPath (Join-Path $runPath 'prepare-accounts.stdout.log') `
        -StderrPath (Join-Path $runPath 'prepare-accounts.stderr.log')
    Invoke-NodeChecked `
        -Script (Join-Path $automationPath 'scripts\seed_synthetic_rdi_fixture.js') `
        -Arguments @('--seed', '--confirm') `
        -StdoutPath (Join-Path $runPath 'synthetic-seed.json') `
        -StderrPath (Join-Path $runPath 'synthetic-seed.stderr.log')
    $seedEvidence = Get-Content -LiteralPath (Join-Path $runPath 'synthetic-seed.json') -Raw | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace([string]$seedEvidence.id)) {
        throw 'synthetic seed returned no device id'
    }
    $env:SYNTHETIC_RDI_DEVICE_ID = [string]$seedEvidence.id
    Invoke-NodeChecked `
        -Script (Join-Path $automationPath 'scripts\activate_synthetic_rdi_fixture.js') `
        -Arguments @() `
        -StdoutPath (Join-Path $runPath 'synthetic-activation.json') `
        -StderrPath (Join-Path $runPath 'synthetic-activation.stderr.log')
    Invoke-NodeChecked `
        -Script (Join-Path $automationPath 'scripts\preflight_api_e2e.js') `
        -Arguments @() `
        -StdoutPath (Join-Path $runPath 'preflight.stdout.log') `
        -StderrPath (Join-Path $runPath 'preflight.stderr.log')

    # The Playwright runner owns the same 9725 preview port during E2E. Stop
    # this preflight-only proxy before handing the port to the runner.
    if ($previewJob) {
        Stop-Job -Id $previewJob.Id -ErrorAction SilentlyContinue
        Remove-Job -Id $previewJob.Id -Force -ErrorAction SilentlyContinue
        $previewJob = $null
    }
    Wait-TcpPortClosed -Port 9725

    Invoke-NodeChecked `
        -Script (Join-Path $automationPath 'run_tests.js') `
        -Arguments @('--include-e2e', '--workers=1', '--archive') `
        -StdoutPath (Join-Path $runPath 'full-api-e2e.stdout.log') `
        -StderrPath (Join-Path $runPath 'full-api-e2e.stderr.log')
    $exitCode = 0
    Write-Output "PREDEPLOY_TEST_EXIT_CODE=0"
    Write-Output "PREDEPLOY_FIXTURE_PID=$fixturePid"
    Write-Output "PREDEPLOY_RUN_DIR=$runPath"
} catch {
    Write-Output ('PREDEPLOY_RUN_ERROR=' + $_.Exception.Message)
    $exitCode = 1
} finally {
    if ($previewJob) {
        Stop-Job -Id $previewJob.Id -ErrorAction SilentlyContinue
        Remove-Job -Id $previewJob.Id -Force -ErrorAction SilentlyContinue
    }
    if ($backendJob) {
        Stop-Job -Id $backendJob.Id -ErrorAction SilentlyContinue
        Remove-Job -Id $backendJob.Id -Force -ErrorAction SilentlyContinue
    }
    if ($brokerJob) {
        Stop-Job -Id $brokerJob.Id -ErrorAction SilentlyContinue
        Remove-Job -Id $brokerJob.Id -Force -ErrorAction SilentlyContinue
    }
    Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
        Where-Object {
            if ($cleanupExecutableNames -notcontains $_.Name) {
                return $false
            }
            $commandLine = [string]$_.CommandLine
            foreach ($candidatePath in $cleanupExecutablePaths) {
                if ($commandLine.IndexOf($candidatePath, [System.StringComparison]::OrdinalIgnoreCase) -ge 0) {
                    return $true
                }
            }
            return $false
        } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:GOTP_DB_PSQL_PASSWORD, Env:AETHERLINK_DB_PASSWORD,
        Env:GMQTT_DB_PSQL_PSQLPASS, Env:AETHERLINK_PREFLIGHT_DB_PASSWORD -ErrorAction SilentlyContinue
}
exit $exitCode
