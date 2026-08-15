param([string]$OutputDir = "")
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $Root
if (-not $OutputDir) { $OutputDir = Join-Path $Root ("verification/backups/postgres-" + (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")) }
elseif (-not [IO.Path]::IsPathRooted($OutputDir)) { $OutputDir = Join-Path $Root $OutputDir }
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
if ((Test-Path -LiteralPath $OutputDir) -and (Get-ChildItem -LiteralPath $OutputDir -Force | Select-Object -First 1)) {
  throw "Backup refused: output directory is not empty: $OutputDir"
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$dump = Join-Path $OutputDir "database.dump"; $hashFile = "$dump.sha256"; $manifest = Join-Path $OutputDir "manifest.json"

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw "external blocker: Docker is required to back up the Compose deployment." }
& docker compose ps --status running postgres | Out-Null
if ($LASTEXITCODE -ne 0) { throw "PostgreSQL service is not running." }

$psi = [Diagnostics.ProcessStartInfo]::new()
$psi.FileName = "docker"; $psi.UseShellExecute = $false; $psi.RedirectStandardOutput = $true
# Leave stderr attached to the terminal so a full error pipe cannot deadlock the
# binary stdout copy. ProcessStartInfo.ArgumentList is unavailable in Windows PowerShell 5.1.
# Keep the shell command quoted as one docker exec argument without expanding
# container environment variables on the host.
$psi.Arguments = 'compose exec -T postgres sh -c "pg_dump -Fc -U \"$POSTGRES_USER\" -d \"$POSTGRES_DB\""'
$process = [Diagnostics.Process]::Start($psi)
$file = [IO.File]::Create($dump)
try { $process.StandardOutput.BaseStream.CopyTo($file) } finally { $file.Dispose() }
$process.WaitForExit()
if ($process.ExitCode -ne 0) { Remove-Item $dump -Force -ErrorAction SilentlyContinue; throw "PostgreSQL backup failed; see the Docker error above." }
if ((Get-Item $dump).Length -eq 0) { Remove-Item $dump -Force; throw "PostgreSQL backup is empty." }
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $dump).Hash.ToLowerInvariant()
"$hash  database.dump" | Set-Content -LiteralPath $hashFile -Encoding ascii
[ordered]@{ schema_version=1; created_at_utc=(Get-Date).ToUniversalTime().ToString("o"); format="postgresql-custom"; database_service="postgres"; dump_file="database.dump"; sha256=$hash; bytes=(Get-Item $dump).Length; includes_cluster_globals=$false } | ConvertTo-Json | Set-Content -LiteralPath $manifest -Encoding utf8
Write-Host "Created PostgreSQL backup: $dump"
Write-Host "Restore testing is required before treating this backup as recoverable."
