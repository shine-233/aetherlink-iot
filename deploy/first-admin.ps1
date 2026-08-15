param(
  [string]$BackendUrl = $env:AETHERLINK_FIRST_ADMIN_BACKEND_URL,
  [string]$Email = $env:AETHERLINK_FIRST_ADMIN_EMAIL,
  [string]$Password = $env:AETHERLINK_FIRST_ADMIN_PASSWORD
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

function Read-AetherLinkEnvFile {
  param([string]$Path)

  $values = @{}
  if (-not (Test-Path -LiteralPath $Path)) {
    return $values
  }

  Get-Content -LiteralPath $Path | ForEach-Object {
    $line = $_.Trim()
    if (-not $line -or $line.StartsWith("#") -or -not $line.Contains("=")) {
      return
    }

    $name, $value = $line.Split("=", 2)
    $values[$name.Trim()] = $value.Trim().Trim('"').Trim("'")
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

function Join-AetherLinkUrl {
  param(
    [string]$BaseUrl,
    [string]$Path
  )

  return "$($BaseUrl.TrimEnd('/'))/$($Path.TrimStart('/'))"
}

function ConvertFrom-AetherLinkSecureString {
  param([securestring]$SecureValue)

  $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureValue)
  try {
    return [System.Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
  } finally {
    [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
  }
}

function Read-AetherLinkPassword {
  if ($Password) {
    return $Password
  }

  Write-Host "Password rules: 8-20 chars, uppercase, lowercase, number, and special char."
  $first = Read-Host "Super admin password" -AsSecureString
  $second = Read-Host "Confirm password" -AsSecureString
  $firstPlain = ConvertFrom-AetherLinkSecureString $first
  $secondPlain = ConvertFrom-AetherLinkSecureString $second
  if ($firstPlain -ne $secondPlain) {
    throw "Passwords do not match. No account was created."
  }
  return $firstPlain
}

function Test-AetherLinkEmail {
  param([string]$Value)

  return $Value -match '^[^@\s]+@[^@\s]+\.[^@\s]+$'
}

function Test-AetherLinkPassword {
  param([string]$Value)

  if (-not $Value -or $Value.Length -lt 8 -or $Value.Length -gt 20) {
    return $false
  }
  if ($Value -notmatch '^[\x21-\x7E]+$') {
    return $false
  }
  return $Value -match '[A-Z]' -and $Value -match '[a-z]' -and $Value -match '[0-9]' -and $Value -match '[^A-Za-z0-9]'
}

function Write-AetherLinkInitFailureHint {
  param([object]$Result)

  Write-Host "Super admin init failed with business code $($Result.code): $($Result.message)" -ForegroundColor Red
  $codeText = [string]$Result.code
  if (@("200055", "200056", "200057") -contains $codeText) {
    Write-Host "Market registration check failed. Open the frontend first-run page to complete the market return flow, or check market configuration before retrying this script."
  }
}

$envValues = Read-AetherLinkEnvFile -Path ".env"

if (-not $BackendUrl) {
  $BackendUrl = "http://localhost:$(Get-AetherLinkEnvOrDefault $envValues 'BACKEND_PORT' '9999')"
}

$PublicUrl = Get-AetherLinkEnvOrDefault $envValues "AETHERLINK_PUBLIC_URL" "http://localhost:8080"
$SetupUrl = Join-AetherLinkUrl $BackendUrl "/api/v1/tenant/setup-state"
$InitUrl = Join-AetherLinkUrl $BackendUrl "/api/v1/tenant/super-admin/init"
$headers = @{ "Accept-Language" = "en_US" }

Write-Host "Checking first-start state: $SetupUrl"
$state = Invoke-RestMethod -Method Get -Uri $SetupUrl -Headers $headers -TimeoutSec 15
if ($state.code -ne 200) {
  throw "Setup-state API returned business code $($state.code): $($state.message)"
}

$hasAdmin = if ($state.data -and $state.data.PSObject.Properties.Name -contains "has_admin") {
  $state.data.has_admin
} else {
  $null
}
$nextStep = if ($state.data) { [string]$state.data.next_step } else { "" }
if ($hasAdmin -ne $false -or $nextStep -ne "create_super_admin") {
  Write-Host "No super admin was created."
  Write-Host "Current has_admin: $hasAdmin"
  Write-Host "Current next step: $nextStep"
  Write-Host "Open: $PublicUrl"
  exit 0
}

while (-not $Email) {
  $Email = Read-Host "Super admin email"
  $Email = $Email.Trim()
}
if (-not (Test-AetherLinkEmail $Email.Trim())) {
  throw "Email format is invalid. No account was created."
}

$adminPassword = Read-AetherLinkPassword
if (-not $adminPassword) {
  throw "Password is required. No account was created."
}
if (-not (Test-AetherLinkPassword $adminPassword)) {
  throw "Password does not meet the local rule: 8-20 visible ASCII chars with uppercase, lowercase, number, and special char. No account was created."
}

$body = @{
  email = $Email.Trim()
  password = $adminPassword
} | ConvertTo-Json -Compress

Write-Host "Creating first super admin..."
$result = Invoke-RestMethod -Method Post -Uri $InitUrl -Headers $headers -ContentType "application/json" -Body $body -TimeoutSec 30
if ($result.code -ne 200) {
  Write-AetherLinkInitFailureHint $result
  exit 1
}

Write-Host "First super admin created."
Write-Host "Open: $PublicUrl"
Write-Host "Next: sign in as the super admin, create the tenant admin at /management/user?setup=tenant-admin, then sign in as the tenant admin."
Write-Host "After that, follow 接入第一台设备: check deployment health, generate the first device, send one test telemetry message, confirm latest telemetry plus the first chart, then download the success proof."
