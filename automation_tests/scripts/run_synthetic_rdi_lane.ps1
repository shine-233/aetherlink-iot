<#
Purpose: run the isolated, software-only synthetic RDI protocol lane.

The database password may be supplied by the legacy -DbPassword parameter, but
the preferred path is the current-process AETHERLINK_PREFLIGHT_DB_PASSWORD
environment variable. It is never written to source, logs, reports, manifests,
or the generated evidence package.
#>

[CmdletBinding()]
param(
    [string]$DbPassword = '',

    [string]$ProjectRoot = '',

    [string]$EvidenceRoot = '',

    [string]$DatabaseName = 'aetherlink_iot_predeploy_retest_20260814_r9d_synthetic',

    [Parameter(Mandatory = $true)]
    [string]$AccountEnvPath,

    [Parameter(Mandatory = $true)]
    [string]$BrokerDirectory,

    [Parameter(Mandatory = $true)]
    [string]$BackendBinary,

    [Parameter(Mandatory = $true)]
    [string]$BrokerBinary,

    [Parameter(Mandatory = $true)]
    [string]$EmulatorBinary,

    [int]$BrokerPort = 11086,

    [int]$BackendPort = 19999,

    [string]$BrokerId = 'synthetic-rdi-predeploy'
)

$ErrorActionPreference = 'Stop'
$scriptRoot = if (-not [string]::IsNullOrWhiteSpace($PSScriptRoot)) {
    $PSScriptRoot
} else {
    Split-Path -Parent $MyInvocation.MyCommand.Path
}
if ([string]::IsNullOrWhiteSpace($ProjectRoot)) {
    $ProjectRoot = (Resolve-Path (Join-Path $scriptRoot '..\..')).Path
}
Set-Location -LiteralPath $ProjectRoot

if ([string]::IsNullOrWhiteSpace($DbPassword)) {
    $DbPassword = [Environment]::GetEnvironmentVariable('AETHERLINK_PREFLIGHT_DB_PASSWORD')
}
if ([string]::IsNullOrWhiteSpace($DbPassword)) {
    throw 'AETHERLINK_PREFLIGHT_DB_PASSWORD is required at runtime (or pass the legacy -DbPassword parameter); the value must not be stored in a file'
}
if ([string]::IsNullOrWhiteSpace($AccountEnvPath)) {
    throw 'AccountEnvPath is required; pass the current isolated .env.local path explicitly'
}
if ([string]::IsNullOrWhiteSpace($BrokerDirectory)) {
    throw 'BrokerDirectory is required; pass the current broker artifact directory explicitly'
}
if ([string]::IsNullOrWhiteSpace($BackendBinary)) {
    throw 'BackendBinary is required; pass the current backend artifact explicitly'
}
if ([string]::IsNullOrWhiteSpace($BrokerBinary)) {
    throw 'BrokerBinary is required; pass the current broker artifact explicitly'
}
if ([string]::IsNullOrWhiteSpace($EmulatorBinary)) {
    throw 'EmulatorBinary is required; pass the current protocol emulator artifact explicitly'
}
if ($BrokerId -notmatch '^[A-Za-z0-9_.-]+$') {
    throw ('BrokerId contains unsupported characters: ' + $BrokerId)
}

$runStamp = Get-Date -Format 'yyyyMMdd-HHmmss'
if ([string]::IsNullOrWhiteSpace($EvidenceRoot)) {
    $EvidenceRoot = Join-Path $ProjectRoot ('verification\synthetic-rdi-' + $runStamp)
}
if (-not [System.IO.Path]::IsPathRooted($EvidenceRoot)) {
    $EvidenceRoot = Join-Path $ProjectRoot $EvidenceRoot
}
$EvidenceRoot = [System.IO.Path]::GetFullPath($EvidenceRoot)
$rawDir = Join-Path $EvidenceRoot 'raw'
$protocolDir = Join-Path $EvidenceRoot 'protocol-emulator'
New-Item -ItemType Directory -Force -Path $rawDir, $protocolDir | Out-Null
$accountRuntimeDir = Join-Path $ProjectRoot ('automation_tests\.local\synthetic-rdi-runtime-' + $runStamp)
if (Test-Path -LiteralPath $accountRuntimeDir) {
    throw ('Refusing to reuse an existing temporary account directory: ' + $accountRuntimeDir)
}
$existingAccountEnvPath = if ([System.IO.Path]::IsPathRooted($AccountEnvPath)) {
    [System.IO.Path]::GetFullPath($AccountEnvPath)
} else {
    Join-Path $ProjectRoot $AccountEnvPath
}
if (-not (Test-Path -LiteralPath $existingAccountEnvPath)) {
    throw ('The isolated database account environment is missing: ' + $existingAccountEnvPath)
}

$fixturePid = 'SYN' + (Get-Date -Format 'yyMMddHH') + ([guid]::NewGuid().ToString('N').Substring(0, 1).ToUpperInvariant())
$deviceId = ''
$dbName = $DatabaseName.Trim()
if ($dbName -notmatch '^[A-Za-z0-9_]+$') {
    throw ('DatabaseName contains unsupported characters: ' + $dbName)
}
$dbUser = 'postgres'
$dbHost = '127.0.0.1'
$dbPort = '5432'
$brokerAddress = '127.0.0.1:' + $BrokerPort
$apiBase = 'http://127.0.0.1:' + $BackendPort + '/api/v1'
$healthUrl = 'http://127.0.0.1:' + $BackendPort + '/health'
$redisDb = '11'
$mqttRuntimeSecret = 'synthetic-rdi-mqtt-' + ([guid]::NewGuid().ToString('N'))
$jwtRuntimeSecret = 'synthetic-rdi-jwt-' + ([guid]::NewGuid().ToString('N'))
$runtimeSecrets = @($DbPassword, $mqttRuntimeSecret, $jwtRuntimeSecret) | Where-Object { -not [string]::IsNullOrEmpty($_) }

$env:GOTP_DB_PSQL_HOST = $dbHost
$env:GOTP_DB_PSQL_PORT = $dbPort
$env:GOTP_DB_PSQL_DBNAME = $dbName
$env:GOTP_DB_PSQL_USERNAME = $dbUser
$env:GOTP_DB_PSQL_PASSWORD = $DbPassword
$env:AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES = $dbName
$env:AETHERLINK_DB_HOST = $dbHost
$env:AETHERLINK_DB_PORT = $dbPort
$env:AETHERLINK_DB_NAME = $dbName
$env:AETHERLINK_DB_USER = $dbUser
$env:AETHERLINK_DB_PASSWORD = $DbPassword
$env:PGPASSWORD = $DbPassword

$env:GOTP_DB_REDIS_ADDR = '127.0.0.1:6379'
$env:GOTP_DB_REDIS_DB = $redisDb
$env:GOTP_DB_REDIS_PASSWORD = ''
$env:GOTP_SERVICE_HTTP_HOST = '127.0.0.1'
$env:GOTP_SERVICE_HTTP_PORT = [string]$BackendPort
$env:GOTP_DEPLOYMENT_PUBLIC_URL = 'http://127.0.0.1:' + $BackendPort
$env:API_BASE_URL = $apiBase
$env:HEALTH_URL = $healthUrl
$env:API_TARGET = 'http://127.0.0.1:' + $BackendPort
$env:AUTOMATION_ACCOUNT_DIR = $accountRuntimeDir
$env:GOTP_JWT_KEY = $jwtRuntimeSecret
$env:GOTP_MARKET_ENABLED = 'false'
$env:GOTP_GRPC_TPTODB_TYPE = 'NONE'
$env:GOTP_MQTT_ENABLED = 'true'
$env:GOTP_MQTT_ACCESS_ADDRESS = $brokerAddress
$env:GOTP_MQTT_BROKER = $brokerAddress
$env:GOTP_MQTT_USER = 'root'
$env:GOTP_MQTT_PASS = $mqttRuntimeSecret
$env:GOTP_MQTT_CLIENT_ID = 'aetherlink-synthetic-rdi-backend'
$env:GOTP_UPLINK_ENABLE = 'true'
$env:GOTP_MQTT_SESSION_REVOCATIONS_BROKER_ID = $BrokerId

$env:GMQTT_DB_PSQL_PSQLADDR = $dbHost
$env:GMQTT_DB_PSQL_PSQLPORT = $dbPort
$env:GMQTT_DB_PSQL_PSQLUSER = $dbUser
$env:GMQTT_DB_PSQL_PSQLPASS = $DbPassword
$env:GMQTT_DB_PSQL_PSQLDB = $dbName
$env:GMQTT_DB_PSQL_SSLMODE = 'disable'
$env:GMQTT_DB_REDIS_CONN = '127.0.0.1:6379'
$env:GMQTT_DB_REDIS_DB_NUM = $redisDb
$env:GMQTT_DB_REDIS_PASSWORD = ''
$env:GMQTT_MQTT_BROKER = 'tcp://' + $brokerAddress
$env:GMQTT_MQTT_PASSWORD = $mqttRuntimeSecret
$env:GMQTT_MQTT_PLUGIN_PASSWORD = $mqttRuntimeSecret
$env:GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID = $BrokerId

$env:AETHERLINK_RDI_FIXTURE_MODE = 'synthetic-rdi'
$env:AETHERLINK_RDI_FIXTURE_PID = $fixturePid
$env:SYNTHETIC_RDI_PID = $fixturePid
$env:SYNTHETIC_RDI_DEVICE_ID = ''
$env:SYNTHETIC_RDI_BROKER = $brokerAddress
$env:SYNTHETIC_RDI_EMULATOR_BIN = if ([System.IO.Path]::IsPathRooted($EmulatorBinary)) {
    [System.IO.Path]::GetFullPath($EmulatorBinary)
} else {
    Join-Path $ProjectRoot $EmulatorBinary
}
$env:SYNTHETIC_RDI_REPORT_DIR = Join-Path $EvidenceRoot 'live-protocol-api'
$env:AETHERLINK_SYNTHETIC_RDI_ALLOWED_PORT = $dbPort
$env:AETHERLINK_SYNTHETIC_RDI_ALLOW = '1'
$shareApiRoot = Join-Path $EvidenceRoot 'share-link-api'
$shareApiReportDir = Join-Path $shareApiRoot 'reports'
$shareApiVerificationDir = Join-Path $shareApiRoot 'verification'
$env:AUTOMATION_REPORT_DIR = $shareApiReportDir
$env:AUTOMATION_VERIFICATION_DIR = $shareApiVerificationDir

function Ensure-IsolatedDatabase {
    param([string]$Name)

    if ($Name -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        throw ('DatabaseName contains unsupported characters: ' + $Name)
    }

    $psqlCommand = Get-Command psql.exe -ErrorAction SilentlyContinue
    $psql = if ($psqlCommand) {
        $psqlCommand.Source
    } elseif (Test-Path -LiteralPath 'C:\Program Files\PostgreSQL\17\bin\psql.exe') {
        'C:\Program Files\PostgreSQL\17\bin\psql.exe'
    } else {
        throw 'psql.exe is required to provision the isolated synthetic database'
    }

    $escapedName = $Name.Replace("'", "''")
    $exists = & $psql -X -v ON_ERROR_STOP=1 -h $dbHost -p $dbPort -U $dbUser -d 'postgres' -At -q `
        -c ("SELECT 1 FROM pg_database WHERE datname = '" + $escapedName + "';")
    if ($LASTEXITCODE -ne 0) {
        throw ('Could not inspect the isolated synthetic database: ' + $Name)
    }

    if (($exists -join '').Trim() -eq '1') {
        return [pscustomobject]@{
            database = $Name
            status = 'existing'
            created = $false
        }
    }

    $quotedName = $Name.Replace('"', '""')
    & $psql -X -v ON_ERROR_STOP=1 -h $dbHost -p $dbPort -U $dbUser -d 'postgres' -q `
        -c ('CREATE DATABASE "' + $quotedName + '";')
    if ($LASTEXITCODE -ne 0) {
        throw ('Could not create the isolated synthetic database: ' + $Name)
    }

    return [pscustomobject]@{
        database = $Name
        status = 'created'
        created = $true
    }
}

function Assert-BrokerPortContract {
    param(
        [string]$Directory,
        [int]$ExpectedPort
    )

    $configPath = Join-Path $Directory 'gmqttd.yml'
    if (-not (Test-Path -LiteralPath $configPath)) {
        throw ('Broker config is missing: ' + $configPath)
    }

    $listenerLine = Get-Content -LiteralPath $configPath |
        Where-Object { $_ -match '127\.0\.0\.1:(\d+)' } |
        Select-Object -First 1
    $listenerMatch = [regex]::Match([string]$listenerLine, '127\.0\.0\.1:(\d+)')
    if (-not $listenerMatch.Success) {
        throw ('Broker config has no loopback listener: ' + $configPath)
    }
    $configuredPort = [int]$listenerMatch.Groups[1].Value
    if ($configuredPort -ne $ExpectedPort) {
        throw ("Broker port contract mismatch: lane requested {0}, gmqttd.yml listens on {1}. Pass a matching BrokerPort or an isolated broker config." -f $ExpectedPort, $configuredPort)
    }

    $aetherlinkConfigPath = Join-Path $Directory 'aetherlink.yml'
    if (Test-Path -LiteralPath $aetherlinkConfigPath) {
        $brokerLine = Get-Content -LiteralPath $aetherlinkConfigPath |
            Where-Object { $_ -match 'broker:\s+tcp://127\.0\.0\.1:(\d+)' } |
            Select-Object -First 1
        if ($brokerLine) {
            $brokerMatch = [regex]::Match([string]$brokerLine, 'tcp://127\.0\.0\.1:(\d+)')
            if ($brokerMatch.Success -and [int]$brokerMatch.Groups[1].Value -ne $ExpectedPort) {
                throw ("Broker plugin port contract mismatch: lane requested {0}, aetherlink.yml points to {1}. Pass a matching BrokerPort or an isolated broker config." -f $ExpectedPort, $brokerMatch.Groups[1].Value)
            }
        }
    }

    return [pscustomobject]@{
        expected_port = $ExpectedPort
        gmqtt_listener_port = $configuredPort
        aetherlink_broker_port = $ExpectedPort
        config = $configPath
    }
}

$databaseProvision = Ensure-IsolatedDatabase -Name $dbName
$databaseProvision | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $rawDir 'database-provision.json') -Encoding UTF8

$contractTestOut = Join-Path $rawDir 'synthetic-rdi-contract.stdout.log'
$contractTestErr = Join-Path $rawDir 'synthetic-rdi-contract.stderr.log'
& node (Join-Path $ProjectRoot 'automation_tests\scripts\test_synthetic_rdi_protocol_validation.js') 1> $contractTestOut 2> $contractTestErr
if ($LASTEXITCODE -ne 0) { throw ('synthetic-rdi contract checks failed with exit code ' + $LASTEXITCODE) }

foreach ($port in @($BrokerPort, $BackendPort)) {
    if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) {
        throw ('Refusing to start isolated lane: port ' + $port + ' is already in use')
    }
}

function Wait-TcpPort {
    param([string]$TargetHost, [int]$Port, [int]$Attempts = 40)
    for ($i = 0; $i -lt $Attempts; $i++) {
        $client = $null
        try {
            $client = [Net.Sockets.TcpClient]::new()
            $task = $client.ConnectAsync($TargetHost, $Port)
            if ($task.Wait(1000) -and $client.Connected) {
                return
            }
        } catch {
        } finally {
            if ($client) { $client.Dispose() }
        }
        Start-Sleep -Milliseconds 250
    }
    throw ('TCP port ' + $TargetHost + ':' + $Port + ' did not become ready')
}

function Wait-Http {
    param([string]$Url, [int]$Attempts = 60)
    for ($i = 0; $i -lt $Attempts; $i++) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2
            if ($response.StatusCode -eq 200) { return }
        } catch {
        }
        Start-Sleep -Milliseconds 500
    }
    throw ('HTTP endpoint did not become ready: ' + $Url)
}

$brokerJob = $null
$backendJob = $null
$laneExitCode = 0
try {
    $brokerDir = if ([System.IO.Path]::IsPathRooted($BrokerDirectory)) {
        [System.IO.Path]::GetFullPath($BrokerDirectory)
    } else {
        Join-Path $ProjectRoot $BrokerDirectory
    }
    $brokerPortContract = Assert-BrokerPortContract -Directory $brokerDir -ExpectedPort $BrokerPort
    $brokerPortContract | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $rawDir 'broker-port-contract.json') -Encoding UTF8
    $brokerExe = if ([System.IO.Path]::IsPathRooted($BrokerBinary)) {
        [System.IO.Path]::GetFullPath($BrokerBinary)
    } else {
        Join-Path $ProjectRoot $BrokerBinary
    }
    $brokerOut = Join-Path $rawDir 'broker.stdout.log'
    $brokerErr = Join-Path $rawDir 'broker.stderr.log'
    $brokerJob = Start-Job -ScriptBlock {
        param($dir, $exe, $out, $err, $runtimeEnvironment)
        Set-Location -LiteralPath $dir
        foreach ($item in $runtimeEnvironment.GetEnumerator()) {
            Set-Item -Path ('Env:' + $item.Key) -Value $item.Value
        }
        & $exe start -c 'gmqttd.yml' 1> $out 2> $err
    } -ArgumentList $brokerDir, $brokerExe, $brokerOut, $brokerErr, @{
        GMQTT_DB_PSQL_PSQLADDR = $dbHost
        GMQTT_DB_PSQL_PSQLPORT = $dbPort
        GMQTT_DB_PSQL_PSQLUSER = $dbUser
        GMQTT_DB_PSQL_PSQLPASS = $DbPassword
        GMQTT_DB_PSQL_PSQLDB = $dbName
        GMQTT_DB_PSQL_SSLMODE = 'disable'
        GMQTT_DB_REDIS_CONN = '127.0.0.1:6379'
        GMQTT_DB_REDIS_DB_NUM = $redisDb
        GMQTT_DB_REDIS_PASSWORD = ''
        GMQTT_MQTT_BROKER = 'tcp://' + $brokerAddress
        GMQTT_MQTT_PASSWORD = $mqttRuntimeSecret
        GMQTT_MQTT_PLUGIN_PASSWORD = $mqttRuntimeSecret
        GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID = $BrokerId
    }
    Wait-TcpPort '127.0.0.1' $BrokerPort

    $backendDir = Join-Path $ProjectRoot 'backend'
    $backendExe = if ([System.IO.Path]::IsPathRooted($BackendBinary)) {
        [System.IO.Path]::GetFullPath($BackendBinary)
    } else {
        Join-Path $ProjectRoot $BackendBinary
    }
    $backendOut = Join-Path $rawDir 'backend.stdout.log'
    $backendErr = Join-Path $rawDir 'backend.stderr.log'
    $backendJob = Start-Job -ScriptBlock {
        param($dir, $exe, $out, $err, $runtimeEnvironment)
        Set-Location -LiteralPath $dir
        foreach ($item in $runtimeEnvironment.GetEnumerator()) {
            Set-Item -Path ('Env:' + $item.Key) -Value $item.Value
        }
        & $exe -config '.\configs\conf-localdev.yml' 1> $out 2> $err
    } -ArgumentList $backendDir, $backendExe, $backendOut, $backendErr, @{
        GOTP_DB_PSQL_HOST = $dbHost
        GOTP_DB_PSQL_PORT = $dbPort
        GOTP_DB_PSQL_DBNAME = $dbName
        GOTP_DB_PSQL_USERNAME = $dbUser
        GOTP_DB_PSQL_PASSWORD = $DbPassword
        GOTP_DB_REDIS_ADDR = '127.0.0.1:6379'
        GOTP_DB_REDIS_DB = $redisDb
        GOTP_DB_REDIS_PASSWORD = ''
        GOTP_SERVICE_HTTP_HOST = '127.0.0.1'
        GOTP_SERVICE_HTTP_PORT = [string]$BackendPort
        GOTP_DEPLOYMENT_PUBLIC_URL = 'http://127.0.0.1:' + $BackendPort
        GOTP_JWT_KEY = $jwtRuntimeSecret
        GOTP_MARKET_ENABLED = 'false'
        GOTP_GRPC_TPTODB_TYPE = 'NONE'
        GOTP_MQTT_ENABLED = 'true'
        GOTP_MQTT_ACCESS_ADDRESS = $brokerAddress
        GOTP_MQTT_BROKER = $brokerAddress
        GOTP_MQTT_USER = 'root'
        GOTP_MQTT_PASS = $mqttRuntimeSecret
        GOTP_MQTT_CLIENT_ID = 'aetherlink-synthetic-rdi-backend'
        GOTP_UPLINK_ENABLE = 'true'
        GOTP_MQTT_SESSION_REVOCATIONS_BROKER_ID = $BrokerId
    }
    Wait-Http $healthUrl

    # Normalize only the explicitly marked synthetic fixture. This is a
    # provenance-protected compatibility step for fixtures created before the
    # hardware_identity field was added; it never touches an ordinary device.
    $fixtureNormalizeOut = Join-Path $rawDir 'fixture-normalization.json'
    $fixtureNormalizeErr = Join-Path $rawDir 'fixture-normalization.stderr.log'
    & node (Join-Path $ProjectRoot 'automation_tests\scripts\seed_synthetic_rdi_fixture.js') --seed --confirm 1> $fixtureNormalizeOut 2> $fixtureNormalizeErr
    if ($LASTEXITCODE -ne 0) { throw ('synthetic fixture normalization failed with exit code ' + $LASTEXITCODE) }
    $fixtureNormalization = Get-Content -LiteralPath $fixtureNormalizeOut -Raw | ConvertFrom-Json
    if ($fixtureNormalization.mode -ne 'synthetic-rdi' -or $fixtureNormalization.pid -ne $fixturePid) {
        throw 'synthetic fixture normalization returned an unexpected provenance or PID'
    }
    if ([string]::IsNullOrWhiteSpace([string]$fixtureNormalization.id) -or
        $fixtureNormalization.hardware_identity.kind -ne 'synthetic' -or
        $fixtureNormalization.hardware_identity.serial -ne ('SYNTH-HW-' + $fixturePid)) {
        throw 'synthetic fixture seed did not return an explicitly synthetic identity'
    }
    $deviceId = [string]$fixtureNormalization.id
    $env:SYNTHETIC_RDI_DEVICE_ID = $deviceId

    if ($fixtureNormalization.action -eq 'created' -and
        ($fixtureNormalization.activate_flag -ne 'inactive' -or $fixtureNormalization.is_enabled -ne 'disabled')) {
        throw 'fresh synthetic fixture must begin inactive/disabled before API activation'
    }

    New-Item -ItemType Directory -Force -Path $accountRuntimeDir | Out-Null
    Copy-Item -LiteralPath $existingAccountEnvPath -Destination (Join-Path $accountRuntimeDir '.env.local')
    foreach ($line in (Get-Content -LiteralPath $existingAccountEnvPath)) {
        $trimmed = $line.Trim()
        $envLineMatch = [regex]::Match($trimmed, '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$')
        if (-not $envLineMatch.Success) {
            continue
        }
        $key = $envLineMatch.Groups[1].Value
        if ($key -match '^(SUPER_ADMIN|TENANT_ADMIN|TENANT_ADMIN_B|TENANT_USER|READONLY_USER|EMAIL_CHANGE_TENANT)_(EMAIL|PASSWORD)$') {
            $value = $envLineMatch.Groups[2].Value.Trim()
            if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            Set-Item -Path ('Env:' + $key) -Value $value
        }
    }
    $accountOut = Join-Path $rawDir 'prepare-accounts.stdout.log'
    $accountErr = Join-Path $rawDir 'prepare-accounts.stderr.log'
    & node (Join-Path $ProjectRoot 'automation_tests\scripts\prepare_local_accounts.js') 1> $accountOut 2> $accountErr
    if ($LASTEXITCODE -ne 0) { throw ('isolated automation account preparation failed with exit code ' + $LASTEXITCODE) }

    # The SQL seed intentionally leaves a fresh fixture inactive/disabled.
    # Exercise the public activation endpoint before any RDI API or protocol
    # assertion, and preserve whether this run actually activated it.
    $activationOut = Join-Path $rawDir 'synthetic-activation.json'
    $activationErr = Join-Path $rawDir 'synthetic-activation.stderr.log'
    & node (Join-Path $ProjectRoot 'automation_tests\scripts\activate_synthetic_rdi_fixture.js') 1> $activationOut 2> $activationErr
    if ($LASTEXITCODE -ne 0) { throw ('synthetic API activation failed with exit code ' + $LASTEXITCODE) }
    $activationEvidence = Get-Content -LiteralPath $activationOut -Raw | ConvertFrom-Json
    if ($activationEvidence.mode -ne 'synthetic-rdi' -or
        $activationEvidence.real_rdi_status -ne 'not-tested' -or
        $activationEvidence.claim_scope -ne 'isolated-software-path-only' -or
        $activationEvidence.activate_flag -ne 'active' -or
        $activationEvidence.is_enabled -ne 'enabled' -or
        $activationEvidence.device_id -ne $deviceId -or
        $activationEvidence.pid -ne $fixturePid -or
        @('activated-this-run', 'reused-existing') -notcontains $activationEvidence.action) {
        throw 'synthetic API activation evidence failed provenance/state assertions'
    }

    # Run the current cross-tenant RDI share/link API module against the same
    # explicitly synthetic fixture before exercising the MQTT protocol lane.
    # Keep its reports separate so a focused result cannot be mistaken for a
    # full API aggregate.
    New-Item -ItemType Directory -Force -Path $shareApiRoot, $shareApiReportDir, $shareApiVerificationDir | Out-Null
    $shareApiOut = Join-Path $rawDir 'share-link-api.stdout.log'
    $shareApiErr = Join-Path $rawDir 'share-link-api.stderr.log'
    & node (Join-Path $ProjectRoot 'automation_tests\run_tests.js') --module device 1> $shareApiOut 2> $shareApiErr
    $shareApiExit = $LASTEXITCODE
    if ($shareApiExit -ne 0) { throw ('synthetic share/link API validation failed with exit code ' + $shareApiExit) }
    $shareApiSummaryPath = Join-Path $shareApiReportDir 'summary.json'
    if (-not (Test-Path -LiteralPath $shareApiSummaryPath)) {
        throw ('synthetic share/link API validation did not produce summary.json: ' + $shareApiSummaryPath)
    }

    $emulatorExe = $env:SYNTHETIC_RDI_EMULATOR_BIN
    & $emulatorExe -mode manifest -seed ('synthetic-rdi-' + $runStamp) -pid $fixturePid -device-id $deviceId 1> (Join-Path $protocolDir 'manifest.json') 2> (Join-Path $rawDir 'manifest.stderr.log')
    if ($LASTEXITCODE -ne 0) { throw 'offline manifest command failed' }
    & $emulatorExe -mode session -seed ('synthetic-rdi-' + $runStamp) -pid $fixturePid -device-id $deviceId -ack-mode success 1> (Join-Path $protocolDir 'session.json') 2> (Join-Path $rawDir 'session.stderr.log')
    if ($LASTEXITCODE -ne 0) { throw 'offline session command failed' }
    $exampleReplay = Join-Path $ProjectRoot 'backend\cmd\synthetic-rdi-protocol-emulator\examples\synthetic-session.json'
    & $emulatorExe -mode replay -replay-file $exampleReplay 1> (Join-Path $protocolDir 'replay-validation.json') 2> (Join-Path $rawDir 'replay.stderr.log')
    if ($LASTEXITCODE -ne 0) { throw 'offline replay validation failed' }

    $nodeOut = Join-Path $rawDir 'protocol-validation.stdout.log'
    $nodeErr = Join-Path $rawDir 'protocol-validation.stderr.log'
    & node (Join-Path $ProjectRoot 'automation_tests\scripts\run_synthetic_rdi_protocol_validation.js') 1> $nodeOut 2> $nodeErr
    $nodeExit = $LASTEXITCODE
    if ($nodeExit -ne 0) { throw ('synthetic API/MQTT validation failed with exit code ' + $nodeExit) }
    $protocolSummaryPath = Join-Path $env:SYNTHETIC_RDI_REPORT_DIR 'summary.json'
    $protocolSummary = Get-Content -LiteralPath $protocolSummaryPath -Raw | ConvertFrom-Json
    if ($protocolSummary.claim_scope -ne 'isolated-software-path-only') {
        throw 'protocol summary claim_scope is not isolated-software-path-only'
    }
    foreach ($ackCase in @('success', 'failure')) {
        $case = $protocolSummary.cases.$ackCase
        if (-not $case -or
            $case.state_transition.before_offline -ne $true -or
            $case.state_transition.online_transition -ne $true -or
            $case.state_transition.offline_transition -ne $true) {
            throw ('protocol state transition is incomplete for ' + $ackCase + ': expected offline -> online -> offline')
        }
    }

    $psql = 'psql'
    $psqlCommand = Get-Command psql.exe -ErrorAction SilentlyContinue
    if ($psqlCommand) {
        $psql = $psqlCommand.Source
    } elseif (Test-Path 'C:\Program Files\PostgreSQL\17\bin\psql.exe') {
        $psql = 'C:\Program Files\PostgreSQL\17\bin\psql.exe'
    }

    $deviceLiteral = $deviceId.Replace("'", "''")
    # The application schema stores device IDs as varchar(36), even though the
    # fixture value is UUID-shaped. Keep this readback schema-aware: casting the
    # literal to uuid makes PostgreSQL compare varchar = uuid and fails before
    # any evidence can be written.
    $deviceSql = "SELECT json_build_object('device_id',d.id::text,'device_number',d.device_number,'activate_flag',d.activate_flag,'is_enabled',d.is_enabled,'is_online',d.is_online,'fixture_provenance',COALESCE(d.additional_info->>'fixture_provenance',''),'hardware_identity',COALESCE(d.additional_info->'hardware_identity','{}'::json),'temperature_1',t.number_v,'telemetry_ts',t.ts) FROM public.devices d LEFT JOIN public.telemetry_current_datas t ON t.device_id=d.id::text AND t.key='temperature_1' WHERE d.id='$deviceLiteral';"
    & $psql -X -v ON_ERROR_STOP=1 -h $dbHost -p $dbPort -U $dbUser -d $dbName -At -q -c $deviceSql 2> (Join-Path $rawDir 'db-readback.stderr.log') | Set-Content -LiteralPath (Join-Path $rawDir 'db-readback.json') -Encoding UTF8
    if ($LASTEXITCODE -ne 0) { throw 'database fixture readback failed' }
    $dbReadback = Get-Content -LiteralPath (Join-Path $rawDir 'db-readback.json') -Raw | ConvertFrom-Json
    if ($dbReadback.fixture_provenance -ne 'synthetic-rdi' -or
        $dbReadback.device_number -ne $fixturePid -or
        $dbReadback.activate_flag -ne 'active' -or
        $dbReadback.is_enabled -ne 'enabled' -or
        $dbReadback.hardware_identity.kind -ne 'synthetic' -or
        $dbReadback.hardware_identity.serial -ne ('SYNTH-HW-' + $fixturePid)) {
        throw 'database fixture readback failed synthetic provenance/activation/hardware assertions'
    }
    $logSql = "SELECT COALESCE(json_agg(row_to_json(x)),'[]'::json) FROM (SELECT message_id,status,identify,rsp_data,error_message FROM public.command_set_logs WHERE device_id='$deviceLiteral' ORDER BY created_at DESC LIMIT 6) x;"
    & $psql -X -v ON_ERROR_STOP=1 -h $dbHost -p $dbPort -U $dbUser -d $dbName -At -q -c $logSql 2> (Join-Path $rawDir 'command-log-readback.stderr.log') | Set-Content -LiteralPath (Join-Path $rawDir 'command-log-readback.json') -Encoding UTF8
    if ($LASTEXITCODE -ne 0) { throw 'database command log readback failed' }

    $secretHits = 0
    $databaseSecretHits = 0
    $mqttSecretHits = 0
    $jwtSecretHits = 0
    $secretHitFiles = @()
    $jwtOrBearerPatternHits = 0
    foreach ($file in (Get-ChildItem -LiteralPath $EvidenceRoot -Recurse -File)) {
        $secretMatches = @()
        $dbMatches = @(Select-String -LiteralPath $file.FullName -SimpleMatch $DbPassword -ErrorAction SilentlyContinue)
        $mqttMatches = @(Select-String -LiteralPath $file.FullName -SimpleMatch $mqttRuntimeSecret -ErrorAction SilentlyContinue)
        $jwtSecretMatches = @(Select-String -LiteralPath $file.FullName -SimpleMatch $jwtRuntimeSecret -ErrorAction SilentlyContinue)
        $databaseSecretHits += $dbMatches.Count
        $mqttSecretHits += $mqttMatches.Count
        $jwtSecretHits += $jwtSecretMatches.Count
        $secretHits += $dbMatches.Count + $mqttMatches.Count + $jwtSecretMatches.Count
        $secretMatches += $dbMatches + $mqttMatches + $jwtSecretMatches
        if ($secretMatches.Count -gt 0) {
            $secretHitFiles += $file.FullName.Substring($EvidenceRoot.Length).TrimStart('\')
        }
        $jwtMatches = @(Select-String -LiteralPath $file.FullName -Pattern 'eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}' -ErrorAction SilentlyContinue)
        $bearerMatches = @(Select-String -LiteralPath $file.FullName -Pattern 'Bearer\s+(?!\[REDACTED\])[^\s,}]+' -ErrorAction SilentlyContinue)
        $jwtOrBearerPatternHits += $jwtMatches.Count + $bearerMatches.Count
    }
    $secretHitFiles = @($secretHitFiles | Sort-Object -Unique)
    if ($secretHits -gt 0 -or $jwtOrBearerPatternHits -gt 0) {
        $sensitiveReasons = @()
        if ($databaseSecretHits -gt 0) { $sensitiveReasons += ('database=' + $databaseSecretHits) }
        if ($mqttSecretHits -gt 0) { $sensitiveReasons += ('mqtt=' + $mqttSecretHits) }
        if ($jwtSecretHits -gt 0) { $sensitiveReasons += ('jwt-secret=' + $jwtSecretHits) }
        if ($jwtOrBearerPatternHits -gt 0) { $sensitiveReasons += ('jwt-or-bearer-pattern=' + $jwtOrBearerPatternHits) }
        throw ('sensitive runtime secret or token material was found in evidence; ' + ($sensitiveReasons -join ', ') + '; files=' + ($secretHitFiles -join ', '))
    }

    $scan = @{
        evidence_class = 'protocol-emulator'
        fixture_provenance = 'synthetic-rdi'
        database_password_included = ($databaseSecretHits -gt 0)
        mqtt_password_included = ($mqttSecretHits -gt 0)
        jwt_secret_included = ($jwtSecretHits -gt 0)
        exact_runtime_secret_hits = $secretHits
        database_secret_hits = $databaseSecretHits
        mqtt_secret_hits = $mqttSecretHits
        jwt_secret_hits = $jwtSecretHits
        jwt_or_bearer_pattern_hits = $jwtOrBearerPatternHits
    }
    $scan | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $rawDir 'sensitive-scan.json') -Encoding UTF8

    $manifest = @{
        schema = 'aetherlink.synthetic-rdi.evidence.v1'
        kind = 'synthetic-rdi'
        evidence_class = 'protocol-emulator'
        fixture_provenance = 'synthetic-rdi'
        device_execution = 'not-proven'
        real_rdi_status = 'not-tested'
        claim_scope = 'isolated-software-path-only'
        verdict = 'partial-current'
        production_signoff = 'not-ready'
        generated_at = (Get-Date).ToString('o')
        backend = $apiBase
        broker = $brokerAddress
        database = $dbName
        redis_db = 11
        fixture = @{
            pid = $fixturePid
            device_id = $deviceId
            voucher_secret_redacted = $true
            hardware_identity = $fixtureNormalization.hardware_identity
        }
        activation_precondition = @{
            seed_action = $fixtureNormalization.action
            seed_activate_flag = $fixtureNormalization.activate_flag
            seed_is_enabled = $fixtureNormalization.is_enabled
            activation = $activationEvidence
        }
        state_transition = @{
            success = 'offline -> online -> offline'
            failure = 'offline -> online -> offline'
        }
        lanes = @{ share_link_api = 'focused-current-device-module'; protocol = 'offline-manifest-session-replay'; api_mqtt = 'success-and-failure-ack-with-online-telemetry-offline-readback'; real_rdi = 'not-tested' }
        share_link_api = @{ report = 'share-link-api/reports/summary.json'; exit_code = $shareApiExit }
        cleanup = @{ status = 'retained-for-follow-up'; fixture_absent = $false }
        redaction = @{ database_password_included = ($databaseSecretHits -gt 0); mqtt_password_included = ($mqttSecretHits -gt 0); jwt_or_bearer_included = ($jwtOrBearerPatternHits -gt 0); exact_runtime_secret_hits = $secretHits }
        blocking_gaps = @('real RDI PID and activation', 'real voucher and hardware identity', 'real firmware MQTT session', 'real physical telemetry and online state', 'real physical ACK', 'production ThingsVis/negative-menu integration')
    }
    $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $EvidenceRoot 'manifest.json') -Encoding UTF8

    @"
# Synthetic RDI protocol validation

- classification: synthetic-rdi / protocol-emulator / simulation
- claim_scope: isolated-software-path-only; not real-rdi and not production sign-off
- backend: $apiBase
- broker: $brokerAddress
- fixture PID: $fixturePid
- fixture device_id: $deviceId
- fixture hardware identity: synthetic / $($fixtureNormalization.hardware_identity.serial)
- activation evidence: $($activationEvidence.action) via POST /api/v1/rdi/devices/activate; final activate_flag=$($activationEvidence.activate_flag), is_enabled=$($activationEvidence.is_enabled)
- API/MQTT result: success and failure ACK cases were run through the isolated broker/backend path
- required state transition for both ACK cases: offline -> online -> offline
- protocol result: offline manifest, session and replay validation were run
- cleanup: fixture retained for follow-up; this package does not claim cleanup
- real RDI status: not tested

This package proves only the software path protocol-emulator -> isolated GMQTT -> backend -> API/SQL. It does not prove a real RDI PID, activation, voucher, hardware identity, firmware MQTT session, physical telemetry, physical online state, or physical device ACK.
"@ | Set-Content -LiteralPath (Join-Path $EvidenceRoot 'README.md') -Encoding UTF8

    # Stop-Job/Remove-Job is deferred until finally, but the redirected log
    # handles must be closed before hashing. The jobs are no longer needed once
    # the API/SQL evidence has been collected, so release them here and let the
    # finally block remain idempotent.
    if ($backendJob) {
        Stop-Job -Job $backendJob -ErrorAction SilentlyContinue | Out-Null
        Remove-Job -Job $backendJob -Force -ErrorAction SilentlyContinue
        $backendJob = $null
    }
    if ($brokerJob) {
        Stop-Job -Job $brokerJob -ErrorAction SilentlyContinue | Out-Null
        Remove-Job -Job $brokerJob -Force -ErrorAction SilentlyContinue
        $brokerJob = $null
    }
    Start-Sleep -Milliseconds 250

    $hashLines = @()
    # The caller may redirect this script's stdout/stderr to files inside the
    # evidence root.  Those handles belong to the parent PowerShell process
    # and can still be open while this script is finalizing.  They are wrapper
    # logs, not lane evidence, so exclude them from the in-process hash pass.
    $wrapperLogNames = @('lane.stdout.log', 'lane.stderr.log')
    foreach ($file in (Get-ChildItem -LiteralPath $EvidenceRoot -Recurse -File | Where-Object {
        $_.FullName -notmatch '\\hashes\\' -and $_.Name -notin $wrapperLogNames
    })) {
        $relative = $file.FullName.Substring($EvidenceRoot.Length).TrimStart('\').Replace('\', '/')
        $fileHash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        $hashLines += ($fileHash + '  ' + $relative)
    }
    New-Item -ItemType Directory -Force -Path (Join-Path $EvidenceRoot 'hashes') | Out-Null
    $hashLines | Sort-Object | Set-Content -LiteralPath (Join-Path $EvidenceRoot 'hashes\SHA256SUMS.txt') -Encoding UTF8

    [PSCustomObject]@{
        evidence_root = $EvidenceRoot
        node_exit = $nodeExit
        share_link_api_exit = $shareApiExit
        share_link_api_summary = (Get-Content -LiteralPath $shareApiSummaryPath -Raw).Trim()
        secret_hits = $secretHits
        database_secret_hits = $databaseSecretHits
        mqtt_secret_hits = $mqttSecretHits
        jwt_secret_hits = $jwtSecretHits
        jwt_or_bearer_pattern_hits = $jwtOrBearerPatternHits
        db_readback = (Get-Content -LiteralPath (Join-Path $rawDir 'db-readback.json') -Raw).Trim()
        protocol_files = @(Get-ChildItem -LiteralPath $protocolDir -File | Select-Object -ExpandProperty Name)
    } | ConvertTo-Json -Depth 6 -Compress
} catch {
    $laneExitCode = 1
    $detail = if ($_.Exception) { $_.Exception.Message } else { [string]$_ }
    Write-Error ('synthetic-rdi lane failed: ' + $detail)
} finally {
    if ($backendJob) {
        Stop-Job -Job $backendJob -ErrorAction SilentlyContinue | Out-Null
        Remove-Job -Job $backendJob -Force -ErrorAction SilentlyContinue
    }
    if ($brokerJob) {
        Stop-Job -Job $brokerJob -ErrorAction SilentlyContinue | Out-Null
        Remove-Job -Job $brokerJob -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $accountRuntimeDir) {
        Remove-Item -LiteralPath $accountRuntimeDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    foreach ($name in @(
        'PGPASSWORD',
        'GOTP_DB_PSQL_PASSWORD',
        'AETHERLINK_DB_PASSWORD',
        'GMQTT_DB_PSQL_PSQLPASS',
        'GOTP_MQTT_PASS',
        'GMQTT_MQTT_PASSWORD',
        'GMQTT_MQTT_PLUGIN_PASSWORD',
        'GOTP_JWT_KEY',
        'AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES'
    )) {
        Remove-Item -Path ('Env:' + $name) -ErrorAction SilentlyContinue
    }
}

if ($laneExitCode -ne 0) {
    exit $laneExitCode
}
