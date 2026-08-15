param([Parameter(Mandatory=$true)][string]$DumpFile, [switch]$ConfirmRestore)
$ErrorActionPreference = "Stop"
if (-not $ConfirmRestore) { throw "Restore refused: pass -ConfirmRestore explicitly." }
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path; Set-Location $Root
$DumpFile = (Resolve-Path -LiteralPath $DumpFile).Path
$hashFile = "$DumpFile.sha256"
if (-not (Test-Path -LiteralPath $hashFile)) { $hashFile = Join-Path (Split-Path $DumpFile) "database.dump.sha256" }
if (-not (Test-Path -LiteralPath $hashFile)) { throw "Restore refused: SHA-256 sidecar is required." }
$expected = ((Get-Content -LiteralPath $hashFile -TotalCount 1) -split '\s+')[0].ToLowerInvariant()
$actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $DumpFile).Hash.ToLowerInvariant()
if ($actual -ne $expected) { throw "Restore refused: dump SHA-256 mismatch." }
& docker compose ps --status running postgres | Out-Null
if ($LASTEXITCODE -ne 0) { throw "PostgreSQL service is not running." }

$psi = [Diagnostics.ProcessStartInfo]::new()
$psi.FileName = "docker"; $psi.UseShellExecute = $false; $psi.RedirectStandardInput = $true
# Leave stderr attached to the terminal so a full error pipe cannot deadlock the
# binary stdin copy. Use the Windows PowerShell 5.1-compatible Arguments property. The quoted
# command remains one docker exec argument and expands variables in the container.
# A single transaction keeps cleanup and recreation atomic if any restore step fails.
$psi.Arguments = 'compose exec -T postgres sh -c "pg_restore --clean --if-exists --exit-on-error --single-transaction --no-owner -U \"$POSTGRES_USER\" -d \"$POSTGRES_DB\""'
$process = [Diagnostics.Process]::Start($psi); $file = [IO.File]::OpenRead($DumpFile)
try { $file.CopyTo($process.StandardInput.BaseStream); $process.StandardInput.Close() } finally { $file.Dispose() }
$process.WaitForExit()
if ($process.ExitCode -ne 0) { throw "PostgreSQL restore failed; see the Docker error above." }
& docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "ANALYZE;"'
if ($LASTEXITCODE -ne 0) { throw "Post-restore ANALYZE failed." }
Write-Host "PostgreSQL restore completed. Validate application health before routing traffic."
