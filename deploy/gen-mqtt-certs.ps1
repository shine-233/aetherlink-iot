# 文件用途：生成本地开发/内网联调用的自签名 MQTT TLS 证书到 deploy/certs/（server.crt/server.key）。
# 核心逻辑：优先使用 PATH 中的 openssl，其次回退 Git for Windows 自带 openssl；生成带 SAN 的十年期证书。
# 关键注意事项：仅限开发/内网环境。自签名证书无法通过公网信任校验，生产部署必须改用正规 CA 签发。
# 用法示例：
#   powershell -ExecutionPolicy Bypass -File deploy\gen-mqtt-certs.ps1
#   powershell -ExecutionPolicy Bypass -File deploy\gen-mqtt-certs.ps1 -CommonName aetherlink-mqtt-dev -SubjectAltName 192.168.1.10,mqtt.example.lan
[CmdletBinding()]
param(
    # 证书 CN，默认 aetherlink-mqtt-dev。
    [string]$CommonName = "aetherlink-mqtt-dev",
    # 追加的 SAN 条目：IPv4/IPv6 自动识别为 IP:，其余按 DNS: 处理。
    # localhost 与 127.0.0.1 始终包含，无需重复传入。
    [string[]]$SubjectAltName = @()
)

$ErrorActionPreference = 'Stop'

$certsDir = Join-Path $PSScriptRoot 'certs'
New-Item -ItemType Directory -Force -Path $certsDir | Out-Null

# 定位 openssl（先查 PATH，再查 Git for Windows 常见安装路径）。
$openssl = (Get-Command openssl.exe -ErrorAction SilentlyContinue).Source
if (-not $openssl) {
    $candidates = @(
        (Join-Path ${env:ProgramFiles} 'Git\usr\bin\openssl.exe'),
        (Join-Path ${env:ProgramFiles(x86)} 'Git\usr\bin\openssl.exe')
    )
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) {
            $openssl = $candidate
            break
        }
    }
}
if (-not $openssl) {
    throw "未找到 openssl.exe。请安装 Git for Windows（自带 openssl）或单独安装 OpenSSL 后重试。"
}

# 组装 SAN 列表：默认覆盖 localhost 回环地址，其余条目自动识别为 IP: 或 DNS:。
$sanParts = @('DNS:localhost', 'IP:127.0.0.1')
foreach ($entry in $SubjectAltName) {
    $value = $entry.Trim()
    if (-not $value) { continue }
    if ($value -match '^[0-9.]+$' -or $value.Contains(':')) {
        $sanParts += "IP:$value"
    }
    else {
        $sanParts += "DNS:$value"
    }
}
$san = $sanParts -join ','

$serverCrt = Join-Path $certsDir 'server.crt'
$serverKey = Join-Path $certsDir 'server.key'

# -addext 需要 OpenSSL 1.1.1+；更老版本会直接报错退出。
& $openssl req -x509 -newkey rsa:2048 -nodes -days 3650 `
    -keyout $serverKey `
    -out $serverCrt `
    -subj "/CN=$CommonName" `
    -addext "subjectAltName=$san"
if ($LASTEXITCODE -ne 0) {
    throw "openssl 生成证书失败（exit $LASTEXITCODE）。请确认 openssl 版本 >= 1.1.1。"
}

Write-Host "已生成本地自签名证书（仅供开发/内网使用），生产部署请使用正规 CA。"
Write-Host "  证书: $serverCrt"
Write-Host "  私钥: $serverKey"
Write-Host "  CN:   $CommonName"
Write-Host "  SAN:  $san"
