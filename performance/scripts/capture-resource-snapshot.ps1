param(
  [Parameter(Mandatory = $true)]
  [string]$OutputPath,

  [string]$TargetUrl = "http://localhost:8080",
  [string]$BackendUrl = "http://localhost:9999"
)

$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force -Path $OutputPath | Out-Null

$snapshot = [ordered]@{
  captured_at = (Get-Date).ToString("o")
  machine = [ordered]@{
    computer_name = $env:COMPUTERNAME
    processor_count = [Environment]::ProcessorCount
    os_version = [System.Environment]::OSVersion.VersionString
  }
  targets = [ordered]@{
    target_url = $TargetUrl
    backend_url = $BackendUrl
  }
  processes = @(Get-Process | Sort-Object -Property CPU -Descending | Select-Object -First 20 ProcessName, Id, CPU, WorkingSet64)
}

if (Get-Command docker -ErrorAction SilentlyContinue) {
  $snapshot["docker"] = [ordered]@{
    version = (& docker --version 2>$null)
    compose_version = (& docker compose version 2>$null)
    stats = (& docker stats --no-stream 2>$null)
  }
} else {
  $snapshot["docker"] = [ordered]@{
    available = $false
  }
}

$snapshot | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -Path (Join-Path $OutputPath "resource-snapshot.json")
