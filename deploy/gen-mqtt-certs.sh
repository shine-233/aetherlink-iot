#!/usr/bin/env sh
# 文件用途：生成本地开发/内网自签名 MQTT TLS 证书到 deploy/certs/（server.crt/server.key）。
# 核心逻辑：使用 openssl 生成带 SAN 的 10 年期自签名证书，私钥落盘并收紧权限。
# 关键注意事项：仅开发/内网联调用——自签名证书无法做公网身份校验，生产环境必须改用正规 CA 签发。
# 用法示例：
#   sh deploy/gen-mqtt-certs.sh
#   sh deploy/gen-mqtt-certs.sh aetherlink-mqtt-dev 192.168.1.10 mqtt.example.lan
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
CERTS_DIR="$SCRIPT_DIR/certs"

CN="${1:-aetherlink-mqtt-dev}"
if [ "$#" -gt 0 ]; then
  shift
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "ERROR: openssl 未安装或不在 PATH 中，请先安装 openssl。" >&2
  exit 1
fi

mkdir -p "$CERTS_DIR"

# 组装 SAN 列表：默认覆盖 localhost 与回环地址；传入条目按内容识别为 IP: 或 DNS:。
SAN="DNS:localhost,IP:127.0.0.1"
for entry in "$@"; do
  case "$entry" in
    # 含冒号按 IPv6 处理；纯数字与点号按 IPv4 处理；其余视为域名。
    *:*) SAN="$SAN,IP:$entry" ;;
    *[!0-9.]*) SAN="$SAN,DNS:$entry" ;;
    *) SAN="$SAN,IP:$entry" ;;
  esac
done

SERVER_CRT="$CERTS_DIR/server.crt"
SERVER_KEY="$CERTS_DIR/server.key"

# CN 通过临时 config 文件传入而不是 "-subj /CN=..."：Git Bash/MSYS 会把
# 以斜杠开头的 -subj 参数误当作路径做自动转换，写法因环境而异；config 文件
# 在原生 Linux/macOS、Git Bash、Windows 原生 openssl 下行为一致。
TMP_CNF="$(mktemp)"
trap 'rm -f "$TMP_CNF"' EXIT
{
  printf '[req]\n'
  printf 'distinguished_name = req_dn\n'
  printf 'prompt = no\n'
  printf '\n'
  printf '[req_dn]\n'
  printf 'CN = %s\n' "$CN"
} > "$TMP_CNF"

# -addext 需要 OpenSSL 1.1.1+。
openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
  -keyout "$SERVER_KEY" \
  -out "$SERVER_CRT" \
  -config "$TMP_CNF" \
  -addext "subjectAltName=$SAN"

chmod 600 "$SERVER_KEY" 2>/dev/null || true

echo "已生成开发自签名证书（仅开发/内网用，生产请使用正规 CA）："
echo "  证书: $SERVER_CRT"
echo "  私钥: $SERVER_KEY"
echo "  CN:   $CN"
echo "  SAN:  $SAN"
