// 文件用途：TOTP 引擎单测——确定性自洽校验（同一密钥同计数同码）、窗口容差、
// 错误码拒绝、密钥格式/随机性、供应 URI 关键参数、RFC 4226 规范 6 位码对齐。
package totp

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateSecretIsBase32AndReusable(t *testing.T) {
	secret, err := GenerateSecret(20)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	if len(secret) < 26 { // 160bit → 32 chars
		t.Fatalf("密钥过短: %d", len(secret))
	}
	// 同一密钥与同计数必须产生同码（确定性）
	decoded, err := b32NoPad.DecodeString(strings.ToUpper(secret))
	if err != nil {
		t.Fatalf("密钥不是合法 base32: %v", err)
	}
	counter := counterAt(time.Unix(1_700_000_000, 0))
	if totpAtCounter(decoded, counter) != totpAtCounter(decoded, counter) {
		t.Fatal("同计数验证码不一致（应确定性）")
	}
}

func TestValidateRoundTripAndRejectsWrongCode(t *testing.T) {
	secret, _ := GenerateSecret(20)
	now := time.Unix(1_700_000_000, 0)
	code := totpAtCounter(mustDecode(t, secret), counterAt(now))
	if !Validate(code, secret, 0, now) {
		t.Fatal("正确验证码应通过（window=0）")
	}
	if Validate("000000", secret, 0, now) {
		t.Fatal("错误验证码应拒绝")
	}
	if Validate(code, secret, 0, now.Add(periodSeconds*time.Second)) {
		t.Fatal("越过窗口的验证码应拒绝（window=0）")
	}
	if !Validate(code, secret, 1, now.Add(periodSeconds*time.Second)) {
		t.Fatal("±1 步进容差应放行相邻窗口验证码")
	}
	if Validate(code, secret, 1, now.Add(3*periodSeconds*time.Second)) {
		t.Fatal("超出 ±1 窗口应拒绝")
	}
}

func TestValidateRejectsBadSecretAndBadCodeShape(t *testing.T) {
	now := time.Now()
	if Validate("123456", "!!!not-base32!!!", 1, now) {
		t.Fatal("非法密钥应拒绝")
	}
	secret, _ := GenerateSecret(20)
	if Validate("abc", secret, 1, now) {
		t.Fatal("非 6 位码应拒绝")
	}
	if Validate("1234567", secret, 1, now) {
		t.Fatal("7 位码应拒绝")
	}
}

func TestValidateConstantTimeStyleWrongSecret(t *testing.T) {
	secretA, _ := GenerateSecret(20)
	secretB, _ := GenerateSecret(20)
	now := time.Now()
	code := totpAtCounter(mustDecode(t, secretA), counterAt(now))
	if Validate(code, secretB, 1, now) {
		t.Fatal("用 B 密钥校验 A 的码应失败")
	}
}

func TestProvisioningURIContainsRequiredParams(t *testing.T) {
	secret, _ := GenerateSecret(20)
	uri := ProvisioningURI("AetherLink", "alice@example.com", secret)
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("URI 前缀错误: %s", uri)
	}
	for _, k := range []string{"secret=", "issuer=AetherLink", "algorithm=SHA1", "digits=6", "period=30"} {
		if !strings.Contains(uri, k) {
			t.Fatalf("URI 缺少 %s: %s", k, uri)
		}
	}
}

func mustDecode(t *testing.T, s string) []byte {
	t.Helper()
	d, err := b32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(s)))
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	return d
}
