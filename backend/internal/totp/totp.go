// 文件用途：2FA（TOTP）核心引擎（ROADMAP C7 剩余项，RFC 6238 / RFC 4226）。
// 核心逻辑：纯标准库实现——base32 密钥编解码、HMAC-SHA1 动态截断、30s 时间步进计数、
//   带 ±window 容差的校验（防时钟漂移）、otpauth:// 供应 URI。
// 关键注意事项：
//   - 校验失败不区分"密钥错/窗口错"，避免时序与存在性侧信道（常量时间比较）；
//   - 生产接入时校验后必须做"一次性消费"（use-once / replay 防复用）与速率限制，本包不含；
//   - 密钥以 base32 无填充大写表示（兼容 Google Authenticator / Authy）。
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// periodSeconds TOTP 时间步长（RFC 6238 默认 30s）。
const periodSeconds = 30

// digits 输出位数。
const digits = 6

var b32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret 生成 nBytes 字节随机密钥并返回 base32（无填充）形式。
func GenerateSecret(nBytes int) (string, error) {
	if nBytes < 10 {
		nBytes = 20 // 默认 160bit，兼容主流验证器
	}
	raw := make([]byte, nBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("totp: 生成密钥失败: %w", err)
	}
	return b32NoPad.EncodeToString(raw), nil
}

// hmacSHA1 计算 HTOP 使用的 HMAC-SHA1。
func hmacSHA1(key, msg []byte) []byte {
	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	return mac.Sum(nil)
}

// dynamicTruncate RFC 4226 动态截断 → 31bit 整数。
func dynamicTruncate(hs []byte) uint32 {
	offset := hs[len(hs)-1] & 0x0f
	return binary.BigEndian.Uint32(hs[offset:offset+4]) & 0x7fffffff
}

// totpAtCounter 按计数计算验证码。
func totpAtCounter(secret []byte, counter uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	code := dynamicTruncate(hmacSHA1(secret, msg[:])) % 1000000
	return fmt.Sprintf("%06d", code)
}

func counterAt(t time.Time) uint64 {
	return uint64(t.Unix()) / periodSeconds
}

// Validate 校验用户输入的 6 位验证码；window 为允许的 ±时间步进容差（一般 1）。
// 常量时间比较，失败不暴露具体原因。
func Validate(code, secretB32 string, window int, now time.Time) bool {
	secret, err := b32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(secretB32)))
	if err != nil {
		return false
	}
	code = strings.TrimSpace(code)
	counter := counterAt(now)
	// 允许负窗口？时钟超前场景少见，双向往返即可。
	for delta := -window; delta <= window; delta++ {
		expected := totpAtCounter(secret, uint64(int64(counter)+int64(delta)))
		if subtleEqual(expected, code) {
			return true
		}
	}
	return false
}

// ProvisioningURI 生成 otpauth:// 供应 URI（供二维码/手动录入）。
func ProvisioningURI(issuer, account, secretB32 string) string {
	issuer = strings.TrimSpace(issuer)
	q := url.Values{}
	q.Set("secret", strings.ToUpper(strings.TrimSpace(secretB32)))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(digits))
	q.Set("period", fmt.Sprint(periodSeconds))
	label := url.PathEscape(issuer + ":" + account)
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// CounterAt 供上层做"当前计数"埋点（如记录 last_counter 防重放）。
func CounterAt(t time.Time) uint64 { return counterAt(t) }

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
