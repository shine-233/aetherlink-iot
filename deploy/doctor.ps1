param(
  [string]$PublicUrl = $env:AETHERLINK_PUBLIC_URL,
  [string]$MqttAddress = $env:AETHERLINK_MQTT_ACCESS_ADDRESS,
  [string]$PerformanceTier = $env:AETHERLINK_PERFORMANCE_TIER,
  [switch]$Server,
  [switch]$LiveDb,
  [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
  Write-Host "Usage: .\deploy\doctor.ps1 [-Server] [-LiveDb] [-PublicUrl <url>] [-MqttAddress <host:port>] [-PerformanceTier light|standard|production]"
  Write-Host "Checks Docker, Compose, .env, secrets, ports, required files, disk, memory, and compose config without starting containers."
  Write-Host "  -Server  Treat localhost public browser/MQTT addresses as blocking errors."
  Write-Host "  -LiveDb  Also probe the configured PostgreSQL endpoint with pg_isready/psql or a TCP fallback."
  Write-Host "  -PerformanceTier  Validate Compose resource preset: light, standard, or production."
  exit 0
}

$LiveDb = $LiveDb -or $env:AETHERLINK_DOCTOR_LIVE_DB -eq "1"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

$script:DoctorResults = @()
$script:AetherLinkEnvIssues = @()

function Read-AetherLinkEnvFile {
  param([string]$Path)

  $values = @{}
  if (-not (Test-Path -LiteralPath $Path)) {
    return $values
  }

  $lineNumber = 0
  Get-Content -LiteralPath $Path | ForEach-Object {
    $lineNumber += 1
    $rawLine = $_
    $line = $rawLine.Trim()
    if (-not $line -or $line.StartsWith("#") -or -not $line.Contains("=")) {
      if ($line -and -not $line.StartsWith("#")) {
        $script:AetherLinkEnvIssues += "Line $lineNumber is ignored because it does not contain '='."
      }
      return
    }

    $name, $value = $line.Split("=", 2)
    $trimmedName = $name.Trim()
    $trimmedValue = $value.Trim()
    if (-not $trimmedName) {
      $script:AetherLinkEnvIssues += "Line $lineNumber has an empty key."
      return
    }
    if ($trimmedName.StartsWith("export ")) {
      $script:AetherLinkEnvIssues += "Line $lineNumber uses 'export'; .env entries should be plain KEY=value."
      $trimmedName = $trimmedName.Substring(7).Trim()
    }
    if ($trimmedName -notmatch "^[A-Za-z_][A-Za-z0-9_]*$") {
      $script:AetherLinkEnvIssues += "Line $lineNumber key '$trimmedName' is not a valid variable name."
    }
    if ($name -ne $trimmedName) {
      $script:AetherLinkEnvIssues += "Line $lineNumber key '$trimmedName' has surrounding whitespace."
    }
    if (($trimmedValue.StartsWith('"') -and -not $trimmedValue.EndsWith('"')) -or ($trimmedValue.StartsWith("'") -and -not $trimmedValue.EndsWith("'"))) {
      $script:AetherLinkEnvIssues += "Line $lineNumber value for $trimmedName has an unmatched quote."
    }
    if ($values.ContainsKey($trimmedName)) {
      $script:AetherLinkEnvIssues += "Line $lineNumber duplicates key $trimmedName; the last value wins."
    }
    $values[$trimmedName] = $trimmedValue.Trim('"').Trim("'")
  }
  return $values
}

function Get-AetherLinkEnvOrDefault {
  param(
    [hashtable]$Values,
    [string]$Name,
    [string]$Default
  )

  if ($Values.ContainsKey($Name) -and $Values[$Name]) {
    return $Values[$Name]
  }
  return $Default
}

function Get-AetherLinkEnvKeys {
  param([string]$Path)

  $keys = @()
  if (-not (Test-Path -LiteralPath $Path)) {
    return $keys
  }

  Get-Content -LiteralPath $Path | ForEach-Object {
    $line = $_.Trim()
    if (-not $line -or $line.StartsWith("#") -or -not $line.Contains("=")) {
      return
    }
    $name = $line.Split("=", 2)[0].Trim()
    if ($name.StartsWith("export ")) {
      $name = $name.Substring(7).Trim()
    }
    if ($name) {
      $keys += $name
    }
  }
  return $keys
}

function Resolve-AetherLinkPerformanceTier {
  param([string]$Tier)

  $resolved = "light"
  if ($Tier) {
    $resolved = $Tier.Trim().ToLowerInvariant()
  }
  return $resolved
}

function Add-AetherLinkDoctorResult {
  param(
    [string]$Name,
    [string]$Level,
    [bool]$Ok,
    [string]$Message,
    [string]$Fix = ""
  )

  $script:DoctorResults += [ordered]@{
    name = $Name
    level = $Level
    ok = $Ok
    message = $Message
    fix = $Fix
  }
}

function Test-AetherLinkPortAvailable {
  param([int]$Port)

  $listener = $null
  try {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
    $listener.Start()
    return $true
  } catch {
    return $false
  } finally {
    if ($listener) {
      $listener.Stop()
    }
  }
}

function ConvertTo-AetherLinkTcpPort {
  param([string]$Value)

  $parsedPort = 0
  if ($Value -match '^[0-9]+$' -and [int]::TryParse($Value, [ref]$parsedPort) -and $parsedPort -ge 1 -and $parsedPort -le 65535) {
    return $parsedPort
  }
  return $null
}

function Test-AetherLinkPort {
  param(
    [string]$Name,
    [string]$Port
  )

  $parsed = ConvertTo-AetherLinkTcpPort $Port
  if ($null -eq $parsed) {
    Add-AetherLinkDoctorResult $Name "error" $false "$Name must be a TCP port between 1 and 65535; current value: $Port." "Edit .env and set a valid $Name."
    return
  }

  if (Test-AetherLinkPortAvailable $parsed) {
    Add-AetherLinkDoctorResult $Name "error" $true "$Name=$parsed is available on localhost."
  } else {
    Add-AetherLinkDoctorResult $Name "warning" $false "$Name=$parsed is already in use on localhost." "If this is not an existing AetherLink container, stop the conflicting service or change $Name in .env."
  }
}

function Test-AetherLinkPortDuplicates {
  param([hashtable]$Ports)

  $seen = @{}
  foreach ($name in $Ports.Keys) {
    $port = [string]$Ports[$name]
    if (-not $seen.ContainsKey($port)) {
      $seen[$port] = @()
    }
    $seen[$port] += $name
  }

  $duplicates = @(
    $seen.GetEnumerator() |
      Where-Object { $_.Value.Count -gt 1 } |
      ForEach-Object { "$($_.Key): $($_.Value -join ', ')" }
  )
  $ok = $duplicates.Count -eq 0
  Add-AetherLinkDoctorResult "env-port-duplicates" "error" $ok "Internal port duplicates: $($(if ($ok) { 'none' } else { $duplicates -join '; ' }))." "Give each exposed service a unique port in .env."
}

function Get-AetherLinkUrlPort {
  param([string]$Value)

  try {
    $uri = [System.Uri]::new($Value)
    if ($uri.IsDefaultPort) {
      if ($uri.Scheme -eq "https") { return 443 }
      if ($uri.Scheme -eq "http") { return 80 }
    }
    return $uri.Port
  } catch {
    return $null
  }
}

function Get-AetherLinkMqttEndpoint {
  param([string]$Value)

  if ([string]::IsNullOrWhiteSpace($Value)) { return $null }

  $trimmed = $Value.Trim()
  if ($trimmed -ne $Value) { return $null }
  $addressHost = ""
  $portText = ""
  $bracketedIpv6 = $false

  if ($trimmed -match '^\[(?<addressHost>[^\[\]]+)\]:(?<port>[0-9]+)$') {
    $addressHost = $Matches.addressHost
    $portText = $Matches.port
    $bracketedIpv6 = $true
  } elseif ($trimmed -match '^(?<addressHost>[^:\[\]\s]+):(?<port>[0-9]+)$') {
    $addressHost = $Matches.addressHost
    $portText = $Matches.port
  } else {
    return $null
  }

  $parsedPort = ConvertTo-AetherLinkTcpPort $portText
  if ($null -eq $parsedPort) { return $null }

  if ($bracketedIpv6) {
    $ipAddress = $null
    $validIpv6 = [System.Net.IPAddress]::TryParse($addressHost, [ref]$ipAddress) -and
      $ipAddress.AddressFamily -eq [System.Net.Sockets.AddressFamily]::InterNetworkV6
    if (-not $validIpv6) { return $null }
    $addressHost = $ipAddress.ToString().ToLowerInvariant()
  } elseif ($addressHost -match '^\d+(?:\.\d+)+$') {
    $segments = @($addressHost.Split([char]'.'))
    if ($segments.Count -ne 4) { return $null }

    $normalizedSegments = @()
    foreach ($segment in $segments) {
      $octet = 0
      if ($segment -notmatch '^\d{1,3}$' -or ($segment.Length -gt 1 -and $segment.StartsWith("0")) -or -not [int]::TryParse($segment, [ref]$octet) -or $octet -lt 0 -or $octet -gt 255) {
        return $null
      }
      $normalizedSegments += [string]$octet
    }
    $addressHost = $normalizedSegments -join "."
  } else {
    $hostname = if ($addressHost.EndsWith(".")) { $addressHost.Substring(0, $addressHost.Length - 1) } else { $addressHost }
    $hostnamePattern = '^(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)(?:\.(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?))*$'
    if (-not $hostname -or $hostname.Length -gt 253 -or $hostname -notmatch $hostnamePattern) {
      return $null
    }
    $addressHost = $hostname.ToLowerInvariant()
  }

  return [pscustomobject]@{
    Host = $addressHost
    Port = $parsedPort
  }
}

function Get-AetherLinkAddressHost {
  param([string]$Value)

  if (-not $Value) { return "" }
  $trimmed = $Value.Trim()
  try {
    if ($trimmed -match "^[a-zA-Z][a-zA-Z0-9+.-]*://") {
      return ([System.Uri]::new($trimmed)).Host.ToLowerInvariant()
    }
  } catch {
    return ""
  }

  $mqttEndpoint = Get-AetherLinkMqttEndpoint $trimmed
  if ($null -ne $mqttEndpoint) {
    return $mqttEndpoint.Host
  }
  return ""
}

function Test-AetherLinkLocalHost {
  param([string]$HostName)

  if ([string]::IsNullOrWhiteSpace($HostName)) { return $false }
  $normalizedHost = $HostName.Trim().TrimEnd(".").Trim([char[]]"[]").ToLowerInvariant()
  return @("localhost", "127.0.0.1", "0.0.0.0", "::", "::1") -contains $normalizedHost
}

function Test-AetherLinkLocalAddress {
  param([string]$Value)

  $addressHost = Get-AetherLinkAddressHost $Value
  return Test-AetherLinkLocalHost $addressHost
}

function Test-AetherLinkPlaceholderHost {
  param([string]$HostName)

  if ([string]::IsNullOrWhiteSpace($HostName)) { return $true }
  $normalizedHost = $HostName.Trim().TrimEnd(".").Trim([char[]]"[]").ToLowerInvariant()
  if (@("example.com", "example.net", "example.org") -contains $normalizedHost) { return $true }
  return $normalizedHost -match "^(?:your[-_]?ip|your[-_]?domain|change[-_]?me|placeholder|todo)$"
}

function Test-AetherLinkServerAddress {
  param([string]$Value)

  $addressHost = Get-AetherLinkAddressHost $Value
  if ([string]::IsNullOrWhiteSpace($addressHost)) { return $false }
  return -not (Test-AetherLinkLocalHost $addressHost) -and -not (Test-AetherLinkPlaceholderHost $addressHost)
}

function Test-AetherLinkWeakSecret {
  param(
    [string]$Name,
    [string]$Value
  )

  $safeValue = if ($null -eq $Value) { "" } else { $Value }
  $lower = $safeValue.ToLowerInvariant()
  $weakValues = @("password", "postgres", "redis", "admin", "root", "123456", "aetherlink")
  $weak = -not $Value -or $weakValues -contains $lower -or $lower -match "^(.)\1{5,}$" -or $lower -match "^change_me"
  Add-AetherLinkDoctorResult "env-$Name-weak-value" "error" (-not $weak) "$Name weak/default value check passed: $(-not $weak)." "Use a unique random value, not password/postgres/redis/admin/root/123456, a repeated character, or a change_me placeholder."
}

function Test-AetherLinkPath {
  param(
    [string]$Name,
    [string]$Path,
    [switch]$Directory
  )

  $exists = if ($Directory) { Test-Path -LiteralPath $Path -PathType Container } else { Test-Path -LiteralPath $Path -PathType Leaf }
  Add-AetherLinkDoctorResult "path-$Name" "error" $exists "$Path exists: $exists." "Restore $Path before building the private deployment package."
}

function Test-AetherLinkTcpConnect {
  param(
    [string]$HostName,
    [int]$Port,
    [int]$TimeoutMs = 3000
  )

  $client = [System.Net.Sockets.TcpClient]::new()
  try {
    $async = $client.BeginConnect($HostName, $Port, $null, $null)
    if (-not $async.AsyncWaitHandle.WaitOne($TimeoutMs, $false)) {
      return $false
    }
    $client.EndConnect($async)
    return $true
  } catch {
    return $false
  } finally {
    $client.Close()
  }
}

function Test-AetherLinkLivePostgres {
  param([hashtable]$Values)

  $hostName = Get-AetherLinkEnvOrDefault $Values "GOTP_DB_PSQL_HOST" "postgres"
  $portText = Get-AetherLinkEnvOrDefault $Values "GOTP_DB_PSQL_PORT" "5432"
  $dbName = Get-AetherLinkEnvOrDefault $Values "GOTP_DB_PSQL_DBNAME" "aetherlink_iot"
  $userName = Get-AetherLinkEnvOrDefault $Values "GOTP_DB_PSQL_USERNAME" "postgres"
  $password = Get-AetherLinkEnvOrDefault $Values "GOTP_DB_PSQL_PASSWORD" ""
  $port = 0

  if (-not [int]::TryParse($portText, [ref]$port) -or $port -lt 1 -or $port -gt 65535) {
    Add-AetherLinkDoctorResult "postgres-live-port" "error" $false "GOTP_DB_PSQL_PORT=$portText is not a valid TCP port." "Set GOTP_DB_PSQL_PORT to a value between 1 and 65535."
    return
  }

  $pgIsReady = Get-Command pg_isready -ErrorAction SilentlyContinue
  if ($pgIsReady) {
    & pg_isready -h $hostName -p $port -U $userName -d $dbName 1>$null 2>$null
    Add-AetherLinkDoctorResult "postgres-live-pg-isready" "error" ($LASTEXITCODE -eq 0) "pg_isready $hostName`:$port/$dbName exit code: $LASTEXITCODE." "Start PostgreSQL or fix GOTP_DB_PSQL_HOST/PORT/DBNAME/USERNAME."
  } else {
    Add-AetherLinkDoctorResult "postgres-live-pg-isready" "warning" $false "pg_isready is not installed; falling back to TCP reachability only." "Install PostgreSQL client tools for a stronger live DB preflight."
  }

  $psql = Get-Command psql -ErrorAction SilentlyContinue
  if ($psql -and $password) {
    $oldPassword = $env:PGPASSWORD
    try {
      $env:PGPASSWORD = $password
      & psql "host=$hostName port=$port user=$userName dbname=$dbName connect_timeout=3 sslmode=prefer" -Atqc "SELECT 1" 1>$null 2>$null
      Add-AetherLinkDoctorResult "postgres-live-select" "error" ($LASTEXITCODE -eq 0) "psql SELECT 1 exit code: $LASTEXITCODE." "Fix PostgreSQL credentials, database name, network route, or pg_hba.conf."
    } finally {
      $env:PGPASSWORD = $oldPassword
    }
    return
  }

  $tcpOk = Test-AetherLinkTcpConnect $hostName $port
  Add-AetherLinkDoctorResult "postgres-live-tcp" "error" $tcpOk "TCP connection to $hostName`:$port succeeded: $tcpOk." "Start PostgreSQL, expose the port, or set GOTP_DB_PSQL_HOST to the address reachable from this machine."
  if (-not $psql) {
    Add-AetherLinkDoctorResult "postgres-live-auth" "warning" $false "psql is not installed, so credentials were not verified." "Install PostgreSQL client tools to let doctor run SELECT 1."
  }
}

$envValues = Read-AetherLinkEnvFile -Path ".env"
$envExists = Test-Path -LiteralPath ".env"

Add-AetherLinkDoctorResult "env-file" "error" $envExists ".env exists: $envExists." "Run .\deploy\init.ps1 once to create .env with generated secrets."

if ($envExists) {
  Add-AetherLinkDoctorResult "env-syntax" "error" ($script:AetherLinkEnvIssues.Count -eq 0) "Parsed .env with $($script:AetherLinkEnvIssues.Count) syntax issue(s)." "Fix malformed, duplicate, or whitespace-padded keys in .env."
  foreach ($issue in $script:AetherLinkEnvIssues) {
    Add-AetherLinkDoctorResult "env-syntax-detail" "warning" $false $issue "Keep one KEY=value entry per line."
  }

  $exampleKeys = @(Get-AetherLinkEnvKeys ".env.example")
  $envKeys = @(Get-AetherLinkEnvKeys ".env")
  $missingKeys = @($exampleKeys | Where-Object { $envKeys -notcontains $_ })
  $extraKeys = @($envKeys | Where-Object { $exampleKeys -notcontains $_ })
  Add-AetherLinkDoctorResult "env-example-required-keys" "error" ($missingKeys.Count -eq 0) ".env is missing $($missingKeys.Count) key(s) from .env.example." "Add missing keys: $($missingKeys -join ', ')."
  Add-AetherLinkDoctorResult "env-extra-keys" "warning" ($extraKeys.Count -eq 0) ".env has $($extraKeys.Count) key(s) not present in .env.example." "Check for typos or document intentional custom keys: $($extraKeys -join ', ')."
}

$docker = Get-Command docker -ErrorAction SilentlyContinue
Add-AetherLinkDoctorResult "docker-cli" "error" ([bool]$docker) "Docker CLI found: $([bool]$docker)." "Install Docker Desktop, then run this script again."

if ($docker) {
  docker compose version 1>$null 2>$null
  Add-AetherLinkDoctorResult "docker-compose" "error" ($LASTEXITCODE -eq 0) "Docker Compose plugin exit code: $LASTEXITCODE." "Install or enable the Docker Compose v2 plugin."
  docker info 1>$null 2>$null
  Add-AetherLinkDoctorResult "docker-daemon" "error" ($LASTEXITCODE -eq 0) "Docker daemon exit code: $LASTEXITCODE." "Start Docker Desktop or the Docker Engine service."
} else {
  Add-AetherLinkDoctorResult "docker-compose" "error" $false "Docker Compose plugin could not be checked because Docker is missing." "Install Docker Desktop."
  Add-AetherLinkDoctorResult "docker-daemon" "error" $false "Docker daemon could not be checked because Docker is missing." "Install and start Docker Desktop."
}

Test-AetherLinkPath "env-example" ".env.example"
Test-AetherLinkPath "compose-file" "docker-compose.yml"
Test-AetherLinkPath "backend" "backend" -Directory
Test-AetherLinkPath "frontend" "frontend" -Directory
Test-AetherLinkPath "mqtt-broker" "mqtt-broker" -Directory
Test-AetherLinkPath "backend-dockerfile" "backend/Dockerfile"
Test-AetherLinkPath "frontend-dockerfile" "frontend/Dockerfile"
Test-AetherLinkPath "mqtt-broker-dockerfile" "mqtt-broker/Dockerfile"
Test-AetherLinkPath "backend-sql" "backend/sql" -Directory
Test-AetherLinkPath "postgres-migrations" "deploy/postgres/00-run-migrations.sh"
Test-AetherLinkPath "gmqtt-config" "mqtt-broker/cmd/gmqttd/default_config.yml"
Test-AetherLinkPath "gmqtt-aetherlink-example" "mqtt-broker/cmd/gmqttd/aetherlink.example.yml"

$offlineImageArchives = @()
foreach ($imageDir in @("deploy/images", "images")) {
  if (Test-Path -LiteralPath $imageDir -PathType Container) {
    $offlineImageArchives += @(Get-ChildItem -LiteralPath $imageDir -File -Recurse -Include *.tar, *.tar.gz, *.tgz -ErrorAction SilentlyContinue)
  }
}
Add-AetherLinkDoctorResult "package-boundary-source-build" "warning" ($offlineImageArchives.Count -gt 0) "Offline image archive count: $($offlineImageArchives.Count). This package otherwise builds or pulls Docker images on the target machine." "For air-gapped installs, prepare image tarballs under deploy/images or use a private registry before running init."

if ($envExists) {
  $requiredSecrets = @(
    "POSTGRES_PASSWORD",
    "GOTP_DB_PSQL_PASSWORD",
    "REDIS_PASSWORD",
    "GOTP_DB_REDIS_PASSWORD",
    "MQTT_ROOT_PASSWORD",
    "MQTT_PLUGIN_PASSWORD",
    "GOTP_MQTT_PASS",
    "GOTP_JWT_KEY"
  )
  foreach ($name in $requiredSecrets) {
    $value = Get-AetherLinkEnvOrDefault $envValues $name ""
    $ok = $value -and $value -notmatch "^change_me"
    Add-AetherLinkDoctorResult "env-$name" "error" ([bool]$ok) "$name is $($(if ($ok) { 'set' } else { 'missing or still a placeholder' }))." "Regenerate .env with .\deploy\init.ps1 or edit $name manually."
  }

  $jwtKey = Get-AetherLinkEnvOrDefault $envValues "GOTP_JWT_KEY" ""
  Add-AetherLinkDoctorResult "env-GOTP_JWT_KEY-length" "warning" ($jwtKey.Length -ge 32) "GOTP_JWT_KEY length is $($jwtKey.Length) character(s)." "Use at least 32 random characters."

  foreach ($name in @("POSTGRES_PASSWORD", "REDIS_PASSWORD", "MQTT_ROOT_PASSWORD", "MQTT_PLUGIN_PASSWORD")) {
    $value = Get-AetherLinkEnvOrDefault $envValues $name ""
    Add-AetherLinkDoctorResult "env-$name-length" "warning" ($value.Length -ge 16) "$name length is $($value.Length) character(s)." "Use at least 16 random characters."
    Test-AetherLinkWeakSecret $name $value
  }

  $psqlMatch = (Get-AetherLinkEnvOrDefault $envValues "POSTGRES_PASSWORD" "") -eq (Get-AetherLinkEnvOrDefault $envValues "GOTP_DB_PSQL_PASSWORD" "")
  Add-AetherLinkDoctorResult "postgres-password-match" "error" $psqlMatch "POSTGRES_PASSWORD and GOTP_DB_PSQL_PASSWORD match: $psqlMatch." "Set both values to the same generated password."

  $postgresDb = Get-AetherLinkEnvOrDefault $envValues "POSTGRES_DB" ""
  $gotpPostgresDb = Get-AetherLinkEnvOrDefault $envValues "GOTP_DB_PSQL_DBNAME" ""
  $postgresDbMatch = [bool]($postgresDb -and $gotpPostgresDb -and $postgresDb -eq $gotpPostgresDb)
  Add-AetherLinkDoctorResult "postgres-database-match" "error" $postgresDbMatch "POSTGRES_DB and GOTP_DB_PSQL_DBNAME match: $postgresDbMatch." "Set POSTGRES_DB and GOTP_DB_PSQL_DBNAME to the same non-empty database name."

  $postgresUser = Get-AetherLinkEnvOrDefault $envValues "POSTGRES_USER" ""
  $gotpPostgresUser = Get-AetherLinkEnvOrDefault $envValues "GOTP_DB_PSQL_USERNAME" ""
  $postgresUserMatch = [bool]($postgresUser -and $gotpPostgresUser -and $postgresUser -eq $gotpPostgresUser)
  Add-AetherLinkDoctorResult "postgres-username-match" "error" $postgresUserMatch "POSTGRES_USER and GOTP_DB_PSQL_USERNAME match: $postgresUserMatch." "Set POSTGRES_USER and GOTP_DB_PSQL_USERNAME to the same non-empty username."

  $redisMatch = (Get-AetherLinkEnvOrDefault $envValues "REDIS_PASSWORD" "") -eq (Get-AetherLinkEnvOrDefault $envValues "GOTP_DB_REDIS_PASSWORD" "")
  Add-AetherLinkDoctorResult "redis-password-match" "error" $redisMatch "REDIS_PASSWORD and GOTP_DB_REDIS_PASSWORD match: $redisMatch." "Set both values to the same generated password."

  $mqttRootPassword = Get-AetherLinkEnvOrDefault $envValues "MQTT_ROOT_PASSWORD" ""
  $mqttPluginPassword = Get-AetherLinkEnvOrDefault $envValues "MQTT_PLUGIN_PASSWORD" ""
  $gotpMqttPassword = Get-AetherLinkEnvOrDefault $envValues "GOTP_MQTT_PASS" ""
  $mqttPasswordMatch = [bool]($mqttRootPassword -and $gotpMqttPassword -and $mqttRootPassword -eq $gotpMqttPassword)
  Add-AetherLinkDoctorResult "mqtt-password-match" "error" $mqttPasswordMatch "MQTT_ROOT_PASSWORD and GOTP_MQTT_PASS match: $mqttPasswordMatch." "Set both values to the same generated password."

  $mqttPluginPasswordDistinct = [bool]($mqttRootPassword -and $mqttPluginPassword -and $mqttRootPassword -ne $mqttPluginPassword)
  Add-AetherLinkDoctorResult "mqtt-plugin-password-distinct" "error" $mqttPluginPasswordDistinct "MQTT_ROOT_PASSWORD and MQTT_PLUGIN_PASSWORD are distinct: $mqttPluginPasswordDistinct." "Use separate generated passwords for the root MQTT account and broker plugin."

  $mqttUser = Get-AetherLinkEnvOrDefault $envValues "GOTP_MQTT_USER" ""
  Add-AetherLinkDoctorResult "mqtt-backend-user" "error" ($mqttUser -eq "root") "GOTP_MQTT_USER=$mqttUser." "Set GOTP_MQTT_USER=root for the current broker integration."

  $mqttBrokerId = Get-AetherLinkEnvOrDefault $envValues "MQTT_BROKER_ID" ""
  $validMqttBrokerId = $mqttBrokerId -match '^[A-Za-z0-9._:-]{1,128}$'
  Add-AetherLinkDoctorResult "mqtt-broker-id" "error" $validMqttBrokerId "MQTT_BROKER_ID=$mqttBrokerId." "Use 1-128 characters from letters, digits, dot, underscore, colon, and hyphen; keep it stable across restarts."

  $ports = @{
    FRONTEND_PORT = Get-AetherLinkEnvOrDefault $envValues "FRONTEND_PORT" "8080"
    BACKEND_PORT = Get-AetherLinkEnvOrDefault $envValues "BACKEND_PORT" "9999"
    MQTT_PORT = Get-AetherLinkEnvOrDefault $envValues "MQTT_PORT" "1883"
    BROKER_METRICS_PORT = Get-AetherLinkEnvOrDefault $envValues "BROKER_METRICS_PORT" "8082"
  }
  # MQTTS is opt-in through a user-provided Compose override and is not part of the default exposed-port contract.
  Test-AetherLinkPortDuplicates $ports
  foreach ($entry in $ports.GetEnumerator()) {
    Test-AetherLinkPort $entry.Key $entry.Value
  }

  if (-not $PublicUrl) {
    $PublicUrl = Get-AetherLinkEnvOrDefault $envValues "AETHERLINK_PUBLIC_URL" "http://localhost:8080"
  }
  if (-not $MqttAddress) {
    $MqttAddress = Get-AetherLinkEnvOrDefault $envValues "AETHERLINK_MQTT_ACCESS_ADDRESS" "localhost:1883"
  }
  if (-not $PerformanceTier) {
    $PerformanceTier = Get-AetherLinkEnvOrDefault $envValues "AETHERLINK_PERFORMANCE_TIER" "light"
  }

  $resolvedPerformanceTier = Resolve-AetherLinkPerformanceTier $PerformanceTier
  $validPerformanceTier = @("light", "standard", "production") -contains $resolvedPerformanceTier
  Add-AetherLinkDoctorResult "performance-tier" "error" $validPerformanceTier "AETHERLINK_PERFORMANCE_TIER=$resolvedPerformanceTier." "Use light, standard, or production."

  $gotpOtaAddress = Get-AetherLinkEnvOrDefault $envValues "GOTP_OTA_DOWNLOAD_ADDRESS" ""
  $gotpMqttAddress = Get-AetherLinkEnvOrDefault $envValues "GOTP_MQTT_ACCESS_ADDRESS" ""
  Add-AetherLinkDoctorResult "public-url-ota-match" "error" ($PublicUrl -eq $gotpOtaAddress) "AETHERLINK_PUBLIC_URL matches GOTP_OTA_DOWNLOAD_ADDRESS: $($PublicUrl -eq $gotpOtaAddress)." "Set GOTP_OTA_DOWNLOAD_ADDRESS to the same public URL used by the frontend."
  Add-AetherLinkDoctorResult "mqtt-address-backend-match" "error" ($MqttAddress -eq $gotpMqttAddress) "AETHERLINK_MQTT_ACCESS_ADDRESS matches GOTP_MQTT_ACCESS_ADDRESS: $($MqttAddress -eq $gotpMqttAddress)." "Set GOTP_MQTT_ACCESS_ADDRESS to the same host:port shown to devices."

  $servicePort = Get-AetherLinkEnvOrDefault $envValues "GOTP_SERVICE_HTTP_PORT" "9999"
  Add-AetherLinkDoctorResult "backend-container-port" "error" ($servicePort -eq "9999") "GOTP_SERVICE_HTTP_PORT=$servicePort." "Keep GOTP_SERVICE_HTTP_PORT=9999 because docker-compose.yml maps host BACKEND_PORT to container 9999."

  $frontendPublicPort = Get-AetherLinkUrlPort $PublicUrl
  $frontendPortText = Get-AetherLinkEnvOrDefault $envValues "FRONTEND_PORT" "8080"
  $frontendPort = ConvertTo-AetherLinkTcpPort $frontendPortText
  $frontendPortMatch = $null -ne $frontendPublicPort -and $null -ne $frontendPort -and $frontendPublicPort -eq $frontendPort
  Add-AetherLinkDoctorResult "public-url-port-match" "warning" $frontendPortMatch "AETHERLINK_PUBLIC_URL port $frontendPublicPort vs FRONTEND_PORT $frontendPortText." "Use a valid exposed FRONTEND_PORT in AETHERLINK_PUBLIC_URL unless a reverse proxy maps it differently."

  $mqttEndpoint = Get-AetherLinkMqttEndpoint $MqttAddress
  $mqttPublicPort = if ($null -ne $mqttEndpoint) { $mqttEndpoint.Port } else { $null }
  $mqttPortText = Get-AetherLinkEnvOrDefault $envValues "MQTT_PORT" "1883"
  $mqttPort = ConvertTo-AetherLinkTcpPort $mqttPortText
  $mqttPortMatch = $null -ne $mqttPublicPort -and $null -ne $mqttPort -and $mqttPublicPort -eq $mqttPort
  Add-AetherLinkDoctorResult "mqtt-address-port-match" "warning" $mqttPortMatch "AETHERLINK_MQTT_ACCESS_ADDRESS port $mqttPublicPort vs MQTT_PORT $mqttPortText." "Use a valid exposed MQTT_PORT in AETHERLINK_MQTT_ACCESS_ADDRESS unless a load balancer maps it differently."

  if ($LiveDb) {
    Test-AetherLinkLivePostgres $envValues
  }
}

$publicUrlOk = $PublicUrl -match "^https?://"
Add-AetherLinkDoctorResult "public-url" "error" $publicUrlOk "AETHERLINK_PUBLIC_URL=$PublicUrl." "Use a full URL such as http://192.168.1.10:8080."

$mqttEndpoint = Get-AetherLinkMqttEndpoint $MqttAddress
$mqttAddressOk = $null -ne $mqttEndpoint
Add-AetherLinkDoctorResult "mqtt-address" "error" $mqttAddressOk "AETHERLINK_MQTT_ACCESS_ADDRESS=$MqttAddress." "Use hostname:port, IPv4:port, or bracketed IPv6:port, for example broker.example.com:1883, 192.168.1.10:1883, or [2001:db8::1]:1883; the port must be 1-65535."

if ($Server) {
  $serverPublicOk = Test-AetherLinkServerAddress $PublicUrl
  $serverPublicMessage = if ($serverPublicOk) { "Server mode public URL is a non-local, non-placeholder address: $PublicUrl." } else { "Server mode public URL is missing, local-only, or a placeholder: $PublicUrl." }
  Add-AetherLinkDoctorResult "server-public-url-not-local" "error" $serverPublicOk $serverPublicMessage "Set -PublicUrl or AETHERLINK_PUBLIC_URL to an IP/domain users can open, for example http://192.168.1.10:8080."
  $serverMqttOk = $mqttAddressOk -and (Test-AetherLinkServerAddress $MqttAddress)
  $serverMqttMessage = if (-not $mqttAddressOk) { "Server mode MQTT address is invalid: $MqttAddress." } elseif ($serverMqttOk) { "Server mode MQTT address is a non-local, non-placeholder endpoint: $MqttAddress." } else { "Server mode MQTT address is missing, local-only, or a placeholder: $MqttAddress." }
  Add-AetherLinkDoctorResult "server-mqtt-address-not-local" "error" $serverMqttOk $serverMqttMessage "Set -MqttAddress or AETHERLINK_MQTT_ACCESS_ADDRESS to an IP/domain devices can reach, for example 192.168.1.10:1883."
}

if ($mqttAddressOk) {
  $mqttHost = $mqttEndpoint.Host
  $mqttLocalOnly = Test-AetherLinkLocalHost $mqttHost
  Add-AetherLinkDoctorResult "mqtt-public-exposure" "warning" $mqttLocalOnly "MQTT access host is $mqttHost." "If MQTT is reachable outside this machine, confirm broker authentication/ACL and network firewall rules before production use."
} else {
  Add-AetherLinkDoctorResult "mqtt-public-exposure" "warning" $false "MQTT access host could not be evaluated because the address is invalid." "Fix AETHERLINK_MQTT_ACCESS_ADDRESS before evaluating public exposure."
}

if ($docker -and $envExists) {
  docker compose config 1>$null 2>$null
  Add-AetherLinkDoctorResult "compose-config" "error" ($LASTEXITCODE -eq 0) "docker compose config exit code: $LASTEXITCODE." "Fix docker-compose.yml or .env, then rerun doctor."
}

try {
  $driveName = ([System.IO.Path]::GetPathRoot($Root.Path)).TrimEnd(":\")
  $drive = Get-PSDrive -Name $driveName -ErrorAction Stop
  $freeGb = [math]::Round($drive.Free / 1GB, 1)
  Add-AetherLinkDoctorResult "disk-free" "warning" ($freeGb -ge 8) "Free disk on $driveName`: ${freeGb}GB." "Free at least 8GB before building images."
} catch {
  Add-AetherLinkDoctorResult "disk-free" "warning" $true "Disk free space could not be checked." ""
}

try {
  $memoryGb = [math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB, 1)
  Add-AetherLinkDoctorResult "memory" "warning" ($memoryGb -ge 2) "Physical memory: ${memoryGb}GB." "Use at least 2GB RAM for the lightweight stack."
} catch {
  Add-AetherLinkDoctorResult "memory" "warning" $true "Memory could not be checked." ""
}

$errors = @($script:DoctorResults | Where-Object { $_.level -eq "error" -and -not $_.ok })
$warnings = @($script:DoctorResults | Where-Object { $_.level -eq "warning" -and -not $_.ok })

Write-Host "AetherLink IoT deployment doctor"
foreach ($result in $script:DoctorResults) {
  $prefix = if ($result.ok) { "[OK]" } elseif ($result.level -eq "warning") { "[WARN]" } else { "[ERROR]" }
  Write-Host "$prefix $($result.name): $($result.message)"
  if (-not $result.ok -and $result.fix) {
    Write-Host "      Fix: $($result.fix)"
  }
}

Write-Host ""
Write-Host "Doctor summary: $($errors.Count) error(s), $($warnings.Count) warning(s)."
Write-Host ""
Write-Host "What to do next:"
if ($errors.Count -gt 0) {
  Write-Host "- Must fix the [ERROR] items above before startup can be trusted."
  if ($Server) {
    Write-Host "- After fixing them, rerun: .\deploy\init.ps1 -Doctor -Server -PublicUrl http://YOUR-IP:8080 -MqttAddress YOUR-IP:1883"
  } else {
    Write-Host "- After fixing them, rerun: .\deploy\init.ps1 -Doctor"
  }
  Write-Host "- If the problem is a port conflict, either stop the other service or edit FRONTEND_PORT, BACKEND_PORT, MQTT_PORT, or BROKER_METRICS_PORT in .env."
  Write-Host "- If the problem is an address mismatch, keep AETHERLINK_PUBLIC_URL and GOTP_OTA_DOWNLOAD_ADDRESS the same, and keep AETHERLINK_MQTT_ACCESS_ADDRESS and GOTP_MQTT_ACCESS_ADDRESS the same."
} elseif ($warnings.Count -gt 0) {
  Write-Host "- Startup is not blocked by doctor errors, but review the [WARN] items before production use."
  Write-Host "- To start anyway, run: .\deploy\init.ps1"
} else {
  Write-Host "- Preflight is clean. To start, run: .\deploy\init.ps1"
}

if ($errors.Count -gt 0) {
  exit 1
}

exit 0
