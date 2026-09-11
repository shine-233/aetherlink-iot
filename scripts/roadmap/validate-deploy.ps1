# Purpose: target deployment validation (ROADMAP P0.1).
# Contract:
#   - Policy violations that are provable without a live target are FAIL (exit 1).
#   - Anything requiring an unreachable/absent target is PENDING (exit 2), never a pass.
#   - -CheckOnly runs local policy checks only and performs no network access.
# Exit codes: 0=pass, 1=policy violation, 2=live evidence incomplete (PENDING).
[CmdletBinding()]
param(
  [string]$BaseUrl = $env:AETHERLINK_BASE_URL,
  [string]$MqttAddress = $env:AETHERLINK_MQTT_ACCESS_ADDRESS,
  [switch]$CheckOnly
)

$ErrorActionPreference = 'Stop'

$script:Failures = 0
$script:Pending = 0

function Write-Check([string]$Level, [string]$Name, [string]$Detail) {
  Write-Output ("[{0}] {1} :: {2}" -f $Level, $Name, $Detail)
}
function Fail([string]$Name, [string]$Detail) { $script:Failures++; Write-Check 'FAIL' $Name $Detail }
function Pend([string]$Name, [string]$Detail) { $script:Pending++; Write-Check 'PENDING' $Name $Detail }
function Pass([string]$Name, [string]$Detail) { Write-Check 'PASS' $Name $Detail }
function Warn([string]$Name, [string]$Detail) { Write-Check 'WARN' $Name $Detail }

$localHosts = @('localhost', '127.0.0.1', '::1')

# 1. Target must be supplied. Absent target is PENDING, not success and not a crash.
function Test-TargetPresent([string]$Url) {
  # Note: any Write-Output inside a function lands in its return value, so use a script-scope flag instead of returning a bool.
  if ([string]::IsNullOrWhiteSpace($Url) -or $Url -like '*CHANGE_ME*') {
    Pend 'target-present' 'AETHERLINK_BASE_URL not supplied; nothing to validate'
    $script:HasTarget = $false
    return
  }
  Pass 'target-present' $Url
  $script:HasTarget = $true
}

# 2. Server targets must use HTTPS. Plain http leaks credentials and MQTT cookies.
function Test-HttpsScheme([string]$Url) {
  $uri = [Uri]$Url
  if ($localHosts -contains $uri.Host) {
    Warn 'https-scheme' "target host is $($uri.Host); local target, scheme not enforced"
    return
  }
  if ($uri.Scheme -ne 'https') {
    Fail 'https-scheme' "server target must use https, got $($uri.Scheme)"
    return
  }
  Pass 'https-scheme' 'server target uses https'
}

# 3. Loopback address on a real deployment means devices cannot reach the platform.
function Test-LoopbackGuard([string]$Url) {
  $uri = [Uri]$Url
  if ($localHosts -contains $uri.Host) {
    Warn 'loopback-guard' "public url host is $($uri.Host); devices on other hosts cannot connect"
    return
  }
  Pass 'loopback-guard' "public url host is $($uri.Host)"
}

# 4. TLS certificate: expiry and hostname binding. Errors are PENDING (target may be down).
function Test-TlsCertificate([string]$Url) {
  if ($CheckOnly) { Pend 'tls-certificate' 'CheckOnly=true; no network access'; return }
  $uri = [Uri]$Url
  if ($uri.Scheme -ne 'https') { Pend 'tls-certificate' 'non-https target; TLS not applicable'; return }
  $port = if ($uri.Port -gt 0) { $uri.Port } else { 443 }
  try {
    $client = New-Object Net.Sockets.TcpClient
    $task = $client.ConnectAsync($uri.Host, $port)
    if (-not $task.Wait(5000)) { throw 'connect timeout' }
    $stream = New-Object Net.Security.SslStream($client.GetStream(), $false, { $true })
    $stream.AuthenticateAsClient($uri.Host)
    $cert = $stream.RemoteCertificate
    if ($null -eq $cert) { throw 'no certificate presented' }
    $x509 = New-Object Security.Cryptography.X509Certificates.X509Certificate2($cert)
    $now = Get-Date
    if ($x509.NotAfter -lt $now) { Fail 'tls-certificate' "certificate expired at $($x509.NotAfter)"; return }
    if ($x509.NotBefore -gt $now) { Fail 'tls-certificate' "certificate not yet valid until $($x509.NotBefore)"; return }
    $days = [math]::Round(($x509.NotAfter - $now).TotalDays)
    if ($days -lt 7) { Warn 'tls-certificate' "certificate expires in $days day(s)"; return }
    Pass 'tls-certificate' "valid for host $($uri.Host), expires in $days day(s)"
    $client.Close()
  } catch {
    Pend 'tls-certificate' "cannot verify TLS ($($_.Exception.Message)); target may be unreachable"
  }
}

# 5. Health response and Secure cookie flag.
function Test-HealthAndCookie([string]$Url) {
  if ($CheckOnly) { Pend 'http-health' 'CheckOnly=true; no network access'; return }
  try {
    $response = Invoke-WebRequest -Uri $Url -Method Get -TimeoutSec 8 -UseBasicParsing -MaximumRedirection 3
    Pass 'http-health' "status $($response.StatusCode)"
    $cookie = $response.Headers['Set-Cookie']
    if ([string]::IsNullOrWhiteSpace($cookie)) {
      Pend 'cookie-secure' 'no Set-Cookie observed on this response; cannot prove Secure flag'
      return
    }
    if ($cookie -notmatch '(?i)Secure') {
      Fail 'cookie-secure' 'session cookie missing Secure attribute'
      return
    }
    Pass 'cookie-secure' 'Set-Cookie carries Secure'
  } catch {
    Pend 'http-health' "request failed ($($_.Exception.Message)); target may be unreachable"
  }
}

# 6. MQTT/MQTTS reachability from the device network.
function Test-MqttReachable([string]$Address) {
  if ([string]::IsNullOrWhiteSpace($Address) -or $Address -like '*CHANGE_ME*') {
    Pend 'mqtt-reachable' 'AETHERLINK_MQTT_ACCESS_ADDRESS not supplied'
    return
  }
  if ($CheckOnly) { Pend 'mqtt-reachable' "CheckOnly=true; target=$Address not probed"; return }
  $parts = $Address -split ':'
  $host = $parts[0]
  $port = 1883
  if ($parts.Count -gt 1) { $port = [int]$parts[1] }
  if ($localHosts -contains $host) {
    Warn 'mqtt-reachable' "mqtt address is $host; devices on other hosts cannot connect"
    return
  }
  try {
    $client = New-Object Net.Sockets.TcpClient
    $task = $client.ConnectAsync($host, $port)
    if (-not $task.Wait(5000)) { throw 'connect timeout' }
    if (-not $client.Connected) { throw 'not connected' }
    $client.Close()
    Pass 'mqtt-reachable' "tcp reachable $host`:$port"
  } catch {
    Pend 'mqtt-reachable' "$Address unreachable ($($_.Exception.Message)); cannot prove device access"
  }
}

Write-Output "AetherLink deployment validation :: CheckOnly=$CheckOnly"
$script:HasTarget = $false
Test-TargetPresent $BaseUrl
if ($script:HasTarget) {
  Test-HttpsScheme $BaseUrl
  Test-LoopbackGuard $BaseUrl
  Test-TlsCertificate $BaseUrl
  Test-HealthAndCookie $BaseUrl
}
Test-MqttReachable $MqttAddress

Write-Output ''
if ($script:Failures -gt 0) {
  Write-Output "VERDICT=BLOCKED :: $($script:Failures) policy violation(s)"
  exit 1
}
if ($script:Pending -gt 0) {
  Write-Output "VERDICT=PENDING :: $($script:Pending) check(s) not executed; deployment unproven"
  exit 2
}
Write-Output 'VERDICT=PASS :: all deployment checks pass'
exit 0
