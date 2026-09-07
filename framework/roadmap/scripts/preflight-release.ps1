[CmdletBinding()]
param(
  [string]$ApiBaseUrl = $env:API_BASE_URL,
  [string]$FrontendUrl = $env:FRONTEND_URL,
  [switch]$CheckOnly
)

$ErrorActionPreference = 'Stop'

function Require-Value([string]$Name, [string]$Value) {
  if ([string]::IsNullOrWhiteSpace($Value) -or $Value -like '*CHANGE_ME*') {
    throw "Missing release preflight value: $Name"
  }
}

Require-Value 'API_BASE_URL' $ApiBaseUrl
Require-Value 'FRONTEND_URL' $FrontendUrl

Write-Output "Release preflight inputs are present. API=$ApiBaseUrl FRONTEND=$FrontendUrl"
if ($CheckOnly) {
  Write-Output 'CheckOnly=true; service, database, broker, TLS, and E2E checks are intentionally not executed by this scaffold.'
  exit 0
}

throw 'Scaffold only: wire repository-specific release checks before enabling execution.'
