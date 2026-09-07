[CmdletBinding()]
param(
  [ValidateSet('plan','backup','restore')]
  [string]$Action = 'plan',
  [string]$ArtifactPath = '.framework-backup'
)

$ErrorActionPreference = 'Stop'
if ([IO.Path]::IsPathRooted($ArtifactPath) -and $ArtifactPath -eq [IO.Path]::GetPathRoot($ArtifactPath)) {
  throw 'Refusing a filesystem root as an artifact path.'
}

switch ($Action) {
  'plan' { Write-Output "Backup/restore plan only. Artifact=$ArtifactPath" }
  'backup' { throw 'Scaffold only: connect to the approved PostgreSQL/Redis backup tooling.' }
  'restore' { throw 'Scaffold only: connect to the approved restore tooling and verification queries.' }
}
