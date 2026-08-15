$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$VerifyPath = Join-Path $Root "deploy\verify.ps1"
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
  $VerifyPath,
  [ref]$tokens,
  [ref]$parseErrors
)

if ($parseErrors.Count -gt 0) {
  throw "verify.ps1 does not parse: $($parseErrors.Message -join '; ')"
}

$functionDefinitions = foreach ($functionName in @(
  "Get-AetherLinkDeploymentHealthFailures",
  "Invoke-AetherLinkHttpCheck",
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

function Assert-Equal {
  param(
    [object]$Actual,
    [object]$Expected,
    [string]$Message
  )

  if ($Actual -ne $Expected) {
    throw "$Message. Expected '$Expected', got '$Actual'."
  }
}

function Assert-Contains {
  param(
    [object[]]$Values,
    [string]$Expected,
    [string]$Message
  )

  if ($Values -notcontains $Expected) {
    throw "$Message. Missing '$Expected' in '$($Values -join ',')'."
  }
}

$script:FixtureBody = ""
$script:FixtureStatus = 200
$script:FixtureCalls = 0

function Invoke-WebRequest {
  param(
    [switch]$UseBasicParsing,
    [string]$Uri,
    [int]$TimeoutSec
  )

  $script:FixtureCalls++
  return [pscustomobject]@{
    Content = $script:FixtureBody
    StatusCode = $script:FixtureStatus
  }
}

$cases = @(
  [ordered]@{
    name = "checks healthy"
    body = '{"checks":{"database":{"ok":true},"mqtt":{"ok":true}}}'
    status = 200
    expectedOk = $true
    expectedFailure = ""
  },
  [ordered]@{
    name = "failed dependency"
    body = '{"checks":{"database":{"ok":true},"mqtt":{"ok":false}}}'
    status = 200
    expectedOk = $false
    expectedFailure = "mqtt"
  },
  [ordered]@{
    name = "invalid json"
    body = '{not-json'
    status = 200
    expectedOk = $false
    expectedFailure = "health-payload-invalid-json"
  },
  [ordered]@{
    name = "missing contract"
    body = '{}'
    status = 200
    expectedOk = $false
    expectedFailure = "health-payload-contract"
  },
  [ordered]@{
    name = "legacy healthy"
    body = '{"frontend_proxy":{"ok":true},"api":{"ok":true}}'
    status = 200
    expectedOk = $true
    expectedFailure = ""
  },
  [ordered]@{
    name = "legacy partial"
    body = '{"frontend_proxy":{"ok":true}}'
    status = 200
    expectedOk = $false
    expectedFailure = "api"
  },
  [ordered]@{
    name = "non-200 healthy body"
    body = '{"checks":{"database":{"ok":true}}}'
    status = 503
    expectedOk = $false
    expectedFailure = ""
  }
)

$TempRoot = Join-Path ([System.IO.Path]::GetTempPath()) (
  "aetherlink-verify-contract-" + [guid]::NewGuid().ToString("N")
)

try {
  New-Item -ItemType Directory -Path $TempRoot | Out-Null
  $script:RawRoot = $TempRoot

  foreach ($case in $cases) {
    $script:FixtureBody = $case.body
    $script:FixtureStatus = $case.status
    $result = Invoke-AetherLinkHttpCheck -Name "deployment-health" -Url "http://fixture.invalid"

  Assert-Equal -Actual ([bool]$result.ok) -Expected ([bool]$case.expectedOk) -Message $case.name
    if ($case.expectedFailure) {
      Assert-Contains -Values @($result.failed_checks) -Expected $case.expectedFailure -Message $case.name
  }
}

foreach ($case in @(
  @("verify-local-url", "http://127.0.0.1:8080", $false),
  @("verify-placeholder-mqtt", "example.com:1883", $false),
  @("verify-real-url", "https://console.example-customer.invalid:8443", $true),
  @("verify-real-mqtt", "broker.example-customer.invalid:1883", $true)
)) {
  Assert-Equal -Actual ([bool](Test-AetherLinkServerAddress $case[1])) -Expected ([bool]$case[2]) -Message $case[0]
}

Assert-Equal -Actual $script:FixtureCalls -Expected $cases.Count -Message "Fixture call count"
Write-Host "PowerShell verify health contract: $($cases.Count + 4) passed"
} finally {
  if (Test-Path -LiteralPath $TempRoot) {
    $resolved = [System.IO.Path]::GetFullPath($TempRoot)
    $tempPath = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
    if ($resolved.StartsWith($tempPath, [System.StringComparison]::OrdinalIgnoreCase)) {
      Remove-Item -LiteralPath $resolved -Recurse -Force
    }
  }
}
