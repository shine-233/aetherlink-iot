$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$DoctorPath = Join-Path $Root "deploy\doctor.ps1"
$FixtureDir = Join-Path $PSScriptRoot "fixtures"
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
  $DoctorPath,
  [ref]$tokens,
  [ref]$parseErrors
)

if ($parseErrors.Count -gt 0) {
  throw "doctor.ps1 does not parse: $($parseErrors.Message -join '; ')"
}

$functionDefinitions = foreach ($functionName in @(
  "Resolve-AetherLinkPerformanceTier",
  "ConvertTo-AetherLinkTcpPort",
  "Get-AetherLinkMqttEndpoint",
  "Get-AetherLinkAddressHost",
  "Test-AetherLinkLocalHost",
  "Test-AetherLinkPlaceholderHost",
  "Test-AetherLinkServerAddress"
)) {
  $definition = $ast.Find({
    param($node)
    $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
      $node.Name -eq $functionName
  }, $true)
  if (-not $definition) {
    throw "Missing production function: $functionName"
  }
  $definition
}

. ([scriptblock]::Create(($functionDefinitions.Extent.Text -join [Environment]::NewLine)))

function Get-FixtureRows {
  param([string]$Path)

  return @(Get-Content -LiteralPath $Path | Where-Object {
    $_ -and -not $_.StartsWith("#")
  } | ForEach-Object {
    ,($_ -split "`t", -1)
  })
}

function Assert-Equal {
  param([object]$Actual, [object]$Expected, [string]$Message)
  if ($Actual -ne $Expected) {
    throw "$Message. Expected '$Expected', got '$Actual'."
  }
}

$mqttCount = 0
foreach ($columns in Get-FixtureRows (Join-Path $FixtureDir "doctor-mqtt-endpoints.tsv")) {
  $name, $endpointText, $validText, $expectedHost, $expectedPort, $localText = $columns
  $mqttCount++
  $endpoint = Get-AetherLinkMqttEndpoint $endpointText
  $actualValid = $null -ne $endpoint
  Assert-Equal ([int]$actualValid) ([int]$validText) "$name validity"
  if ($actualValid) {
    Assert-Equal $endpoint.Host $expectedHost "$name host"
    Assert-Equal ([string]$endpoint.Port) $expectedPort "$name port"
    Assert-Equal ([int](Test-AetherLinkLocalHost $endpoint.Host)) ([int]$localText) "$name local host"
  }
}

$performanceCount = 0
foreach ($columns in Get-FixtureRows (Join-Path $FixtureDir "doctor-performance-tiers.tsv")) {
  $name, $inputText, $expectedNormalized, $validText = $columns
  $performanceCount++
  if ($inputText -eq "<empty>") { $inputText = "" }
  $actualNormalized = Resolve-AetherLinkPerformanceTier $inputText
  if ($expectedNormalized -eq "<empty>") { $expectedNormalized = "" }
  Assert-Equal $actualNormalized $expectedNormalized "$name normalization"
  $actualValid = @("light", "standard", "production") -contains $actualNormalized
  Assert-Equal ([int]$actualValid) ([int]$validText) "$name validity"
}

$portCount = 0
foreach ($columns in Get-FixtureRows (Join-Path $FixtureDir "doctor-tcp-ports.tsv")) {
  $name, $inputText, $validText, $expectedPort = $columns
  $portCount++
  if ($inputText -eq "<empty>") { $inputText = "" }
  $actualPort = ConvertTo-AetherLinkTcpPort $inputText
  $actualValid = $null -ne $actualPort
  Assert-Equal ([int]$actualValid) ([int]$validText) "$name validity"
  if ($actualValid) {
    Assert-Equal ([string]$actualPort) $expectedPort "$name normalized port"
  }
}

$serverAddressCases = @(
  @("server-local-url", "http://127.0.0.1:8080", $false),
  @("server-unspecified-ip", "http://0.0.0.0:8080", $false),
  @("server-placeholder-url", "http://YOUR-IP:8080", $false),
  @("server-local-mqtt", "localhost:1883", $false),
  @("server-placeholder-mqtt", "example.com:1883", $false),
  @("server-real-url", "https://console.example-customer.invalid:8443", $true),
  @("server-real-mqtt", "broker.example-customer.invalid:1883", $true)
)
foreach ($case in $serverAddressCases) {
  Assert-Equal ([bool](Test-AetherLinkServerAddress $case[1])) ([bool]$case[2]) "$($case[0]) server address validation"
}

$total = $mqttCount + $performanceCount + $portCount + $serverAddressCases.Count
Write-Host "PowerShell doctor pure rules contract: $total passed (MQTT $mqttCount, performance $performanceCount, port $portCount, server-address $($serverAddressCases.Count))"
