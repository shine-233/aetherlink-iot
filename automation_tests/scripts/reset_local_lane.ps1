# 文件用途：本地验证 lane 的清态复位脚本（P1 处置配套，见 VALIDATION.md 2026-08-23）。
# 核心逻辑：终止残留运行时进程 → FLUSHALL Redis → 清除自动化凭据与认证目录 → 强制重建隔离库。
# 关键注意事项：仅针对本机隔离环境（aetherlink_iot_sec_20260822 与已知进程名）；不会触碰生产配置。
# 使用示例：
#   powershell -ExecutionPolicy Bypass -File scripts\reset_local_lane.ps1 -PgPassword '<pwd>' [-SkipDb] [-StartStack]
# 参数 -StartStack 会在复位后拉起 broker(1883)+backend(19999)（需 runtime 目录已构建二进制）。
param(
  [string]$PgPassword = $env:PGPASSWORD,
  [string]$DbName = 'aetherlink_iot_sec_20260822',
  [switch]$SkipDb,
  [switch]$StartStack,
  [string]$RuntimeDir = (Join-Path $env:TEMP 'opencode\aetherlink-runtime')
)

$ErrorActionPreference = 'Continue'
$psql = 'C:\Program Files\PostgreSQL\17\bin\psql.exe'
if (-not $PgPassword) { Write-Error 'PG password required: pass -PgPassword or set PGPASSWORD'; exit 2 }

Write-Host '[1/4] 终止残留运行时进程...'
foreach ($name in @('aetherlink-backend', 'gmqttd', 'synthetic-rdi-protocol-emulator')) {
  Get-Process -Name $name -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
}
foreach ($port in @(9725, 19999)) {
  Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue |
    ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
}
Start-Sleep -Seconds 2

Write-Host '[2/4] FLUSHALL Redis...'
try {
  $r = New-Object Net.Sockets.TcpClient('127.0.0.1', 6379)
  $s = $r.GetStream()
  $b = [Text.Encoding]::ASCII.GetBytes("FLUSHALL`r`n")
  $s.Write($b, 0, $b.Length)
  $r.Close()
} catch { Write-Warning "Redis flush skipped: $($_.Exception.Message)" }

Write-Host '[3/4] 清除自动化凭据与认证目录...'
$root = Split-Path -Parent $PSScriptRoot
Remove-Item -Recurse -Force (Join-Path $root '.local') -ErrorAction SilentlyContinue
Remove-Item -Force (Join-Path $root '.env.local') -ErrorAction SilentlyContinue

if (-not $SkipDb) {
  Write-Host "[4/4] 重建隔离库 $DbName..."
  $env:PGPASSWORD = $PgPassword
  & $psql -U postgres -h 127.0.0.1 -w -tAc "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$DbName' AND pid <> pg_backend_pid()" | Out-Null
  & $psql -U postgres -h 127.0.0.1 -w -tAc "DROP DATABASE IF EXISTS $DbName" | Out-Null
  & $psql -U postgres -h 127.0.0.1 -w -tAc "CREATE DATABASE $DbName" | Out-Null
  Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
} else {
  Write-Host '[4/4] 跳过数据库重建 (-SkipDb)'
}

if ($StartStack) {
  Write-Host '拉起 broker + backend...'
  $env:AETHERLINK_DB_NAME = $DbName
  Start-Process -FilePath (Join-Path $RuntimeDir 'gmqttd.exe') -ArgumentList 'start','-c','default_config.yml' `
    -WorkingDirectory $RuntimeDir -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $RuntimeDir 'broker-out.log') `
    -RedirectStandardError (Join-Path $RuntimeDir 'broker-err.log')
  Start-Sleep -Seconds 6
  $env:GOTP_SERVICE_HTTP_PORT = '19999'
  Start-Process -FilePath (Join-Path $RuntimeDir 'aetherlink-backend.exe') `
    -ArgumentList '-config', (Join-Path $RuntimeDir 'backend-conf.yml') `
    -WorkingDirectory (Resolve-Path (Join-Path $PSScriptRoot '..\..\backend')) `
    -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $RuntimeDir 'backend-out.log') `
    -RedirectStandardError (Join-Path $RuntimeDir 'backend-err.log')
  Start-Sleep -Seconds 20
}

Write-Host 'lane 复位完成。后续：prepare_local_accounts → seed/activate RDI → preflight:api-e2e → run_tests.js'
