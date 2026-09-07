[CmdletBinding()]
param(
  [string]$BaseUrl = $env:AETHERLINK_BASE_URL,
  [switch]$CheckOnly
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($BaseUrl)) { throw 'AETHERLINK_BASE_URL is required' }

Write-Output "Deployment validation target: $BaseUrl"
if ($CheckOnly) {
  Write-Output 'CheckOnly=true; TLS, health, migration, and restore assertions remain TODO.'
  exit 0
}

throw 'Scaffold only: add authenticated health, migration, TLS, MQTT, and backup assertions.'
