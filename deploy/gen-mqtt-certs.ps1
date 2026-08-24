# �ļ���;�����ɱ��ؿ���/������ǩ�� MQTT TLS ֤�鵽 deploy/certs/��server.crt/server.key����
# �����߼�������ʹ�� PATH �� Git for Windows �Դ��� openssl�����ɴ� SAN �� 10 ������ǩ��֤�顣
# �ؼ�ע�����������/���������á�����ǩ��֤���޷�����������У�飬������������������� CA ǩ����
# �÷�ʾ����
#   powershell -ExecutionPolicy Bypass -File deploy\gen-mqtt-certs.ps1
#   powershell -ExecutionPolicy Bypass -File deploy\gen-mqtt-certs.ps1 -CommonName aetherlink-mqtt-dev -SubjectAltName 192.168.1.10,mqtt.example.lan
[CmdletBinding()]
param(
    # ֤�� CN��Ĭ�� aetherlink-mqtt-dev��
    [string]$CommonName = "aetherlink-mqtt-dev",
    # ׷�ӵ� SAN ��Ŀ��IPv4/IPv6 �Զ�ʶ��Ϊ IP:�����ఴ DNS: ������
    # localhost �� 127.0.0.1 ʼ�հ����������ظ����롣
    [string[]]$SubjectAltName = @()
)

$ErrorActionPreference = 'Stop'

$certsDir = Join-Path $PSScriptRoot 'certs'
New-Item -ItemType Directory -Force -Path $certsDir | Out-Null

# ���� openssl������ PATH����� Git for Windows ������װ·����Windows ��Ĭ���Դ� openssl����
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
    throw "δ�ҵ� openssl.exe���밲װ Git for Windows���Դ� openssl������� OpenSSL �����ԡ�"
}

# ��װ SAN �б���Ĭ�ϸ��� localhost ��ػ���ַ��������Ŀ������ʶ��Ϊ IP: �� DNS:��
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

# -addext ��Ҫ OpenSSL 1.1.1+�����ɰ汾�������ﱨ���˳���
& $openssl req -x509 -newkey rsa:2048 -nodes -days 3650 `
    -keyout $serverKey `
    -out $serverCrt `
    -subj "/CN=$CommonName" `
    -addext "subjectAltName=$san"
if ($LASTEXITCODE -ne 0) {
    throw "openssl ����֤��ʧ�ܣ�exit $LASTEXITCODE������ȷ�� openssl �汾 >= 1.1.1��"
}

Write-Host "�����ɿ�����ǩ��֤�飨������/�����ã�������ʹ������ CA����"
Write-Host "  ֤��: $serverCrt"
Write-Host "  ˽Կ: $serverKey"
Write-Host "  CN:   $CommonName"
Write-Host "  SAN:  $san"
