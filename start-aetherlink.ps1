param(
  [string]$PublicUrl = "",
  [string]$MqttAddress = "",
  [string]$BindAddress = "",
  [string]$PerformanceTier = "",
  [switch]$Server,
  [switch]$Doctor,
  [switch]$NoBuild,
  [switch]$SkipVerify,
  [switch]$NoPause,
  [switch]$Open,
  [switch]$Help
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

if ($Help) {
  Write-Host "AetherLink IoT one-click starter"
  Write-Host "Usage:"
  Write-Host "  .\start-aetherlink.ps1"
  Write-Host "  .\start-aetherlink.ps1 -Doctor"
  Write-Host "  .\start-aetherlink.ps1 -Open"
  Write-Host "  .\start-aetherlink.ps1 -PerformanceTier light"
  Write-Host "  .\start-aetherlink.ps1 -Server -PublicUrl http://1.2.3.4:8080 -MqttAddress 1.2.3.4:1883 -BindAddress 0.0.0.0"
  Write-Host ""
  Write-Host "Double-click start-aetherlink.cmd for the guided Windows startup."
  exit 0
}

$starter = Join-Path $Root "deploy\start-windows.ps1"
if (-not (Test-Path -LiteralPath $starter)) {
  throw "Missing deploy\start-windows.ps1. Run this script from a complete AetherLink IoT package."
}

$starterArgs = @()
$starterParams = @{}
if ($Server) { $starterArgs += "-Server"; $starterParams["Server"] = $true }
if ($Doctor) { $starterArgs += "-Doctor"; $starterParams["Doctor"] = $true }
if ($NoBuild) { $starterArgs += "-NoBuild"; $starterParams["NoBuild"] = $true }
if ($SkipVerify) { $starterArgs += "-SkipVerify"; $starterParams["SkipVerify"] = $true }
if ($NoPause) { $starterArgs += "-NoPause"; $starterParams["NoPause"] = $true }
if ($Open) { $starterArgs += "-Open"; $starterParams["Open"] = $true }
if ($PublicUrl) {
  $starterArgs += "-PublicUrl"
  $starterArgs += $PublicUrl
  $starterParams["PublicUrl"] = $PublicUrl
}
if ($MqttAddress) {
  $starterArgs += "-MqttAddress"
  $starterArgs += $MqttAddress
  $starterParams["MqttAddress"] = $MqttAddress
}
if ($BindAddress) {
  $starterArgs += "-BindAddress"
  $starterArgs += $BindAddress
  $starterParams["BindAddress"] = $BindAddress
}
if ($PerformanceTier) {
  $starterArgs += "-PerformanceTier"
  $starterArgs += $PerformanceTier
  $starterParams["PerformanceTier"] = $PerformanceTier
}

Write-Host "AetherLink IoT one-click starter"
Write-Host "Project root: $Root"
Write-Host "Running: .\deploy\start-windows.ps1 $($starterArgs -join ' ')"
Write-Host ""

& $starter @starterParams
exit $LASTEXITCODE
