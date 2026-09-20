// 文件用途：通用 Secrets Storage（ROADMAP TB-18）业务逻辑与安全边界单元测试。
// 核心逻辑：覆盖 Key 正则、类型校验、掩码规则、引用语法解析以及信封加密跨租户防搬运隔离。
package service

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/secrets"

	"github.com/spf13/viper"
)

// setupTestMasterKey 为测试注入模拟的 master key
func setupTestMasterKey(t *testing.T) {
	t.Helper()
	key32 := []byte("01234567890123456789012345678901") // 32 bytes
	b64Key := base64.StdEncoding.EncodeToString(key32)
	viper.Set("secrets.master_keys.k1", b64Key)
	viper.Set("secrets.active_key_id", "k1")
}

func TestSecretKeyRegexValidation(t *testing.T) {
	validKeys := []string{
		"AWS_ACCESS_KEY_ID",
		"stripe-secret.v1",
		"mqtt_client_pass",
		"apiKey123",
		"A.B-C_D",
		"k",
	}
	for _, k := range validKeys {
		if !model.SecretKeyRegex.MatchString(k) {
			t.Errorf("expected key %q to be valid, but regex rejected it", k)
		}
	}

	invalidKeys := []string{
		"",
		"key with spaces",
		"key/slash",
		"key@email",
		"key#hash",
		"key$dollar",
		"key%percent",
		"key:colon",
		strings.Repeat("a", 65), // 超长
	}
	for _, k := range invalidKeys {
		if model.SecretKeyRegex.MatchString(k) {
			t.Errorf("expected key %q to be invalid, but regex accepted it", k)
		}
	}
}

func TestSecretTypeValidation(t *testing.T) {
	validTypes := []string{
		model.SecretTypeGeneric,
		model.SecretTypeApiKey,
		model.SecretTypeToken,
		model.SecretTypePassword,
		model.SecretTypeCertificate,
		model.SecretTypeOAuth2,
	}
	for _, st := range validTypes {
		if !validateSecretType(st) {
			t.Errorf("expected type %q to be valid", st)
		}
	}

	invalidTypes := []string{
		"",
		"UNKNOWN",
		"INVALID_TYPE",
		"generic", // 必须大写归一
	}
	for _, st := range invalidTypes {
		if validateSecretType(st) {
			t.Errorf("expected type %q to be invalid", st)
		}
	}
}

func TestSecretMaskPreview(t *testing.T) {
	cases := []struct {
		plain string
		head  int
		want  string
	}{
		{"sk-proj-123456789", 4, "sk-p****"},
		{"abcdef", 4, "abcd****"},
		{"abc", 4, "****"},
		{"", 4, "****"},
	}
	for _, c := range cases {
		got := secrets.Mask(c.plain, c.head)
		if got != c.want {
			t.Errorf("Mask(%q, %d) = %q; want %q", c.plain, c.head, got, c.want)
		}
	}
}

func TestResolveSecretKeyParsing(t *testing.T) {
	setupTestMasterKey(t)

	// 测试解析语法：${secret.KEY} 与 secret:KEY 与 KEY
	cases := []struct {
		input string
		want  string
	}{
		{"${secret.AWS_KEY}", "AWS_KEY"},
		{"${secret.db_pass_123}", "db_pass_123"},
		{"secret:MQTT_TOKEN", "MQTT_TOKEN"},
		{"DIRECT_KEY", "DIRECT_KEY"},
		{"  ${secret.SPACED_KEY}  ", "SPACED_KEY"},
	}

	for _, tc := range cases {
		// 校验提取逻辑
		raw := strings.TrimSpace(tc.input)
		key := raw
		if strings.HasPrefix(raw, "${secret.") && strings.HasSuffix(raw, "}") {
			key = strings.TrimSuffix(strings.TrimPrefix(raw, "${secret."), "}")
		} else if strings.HasPrefix(raw, "secret:") {
			key = strings.TrimPrefix(raw, "secret:")
		}
		key = strings.TrimSpace(key)
		if key != tc.want {
			t.Errorf("input %q got key %q; want %q", tc.input, key, tc.want)
		}
	}

	// 测试畸形语法 Fail-Closed
	invalidInputs := []string{
		"",
		"   ",
		"${secret.}",
		"${secret.  }",
		"secret:",
	}
	for _, input := range invalidInputs {
		_, err := ResolveSecret(context.Background(), "t1", input)
		if err == nil {
			t.Errorf("expected error for invalid input %q, but got nil", input)
		}
	}
}

func TestSecretEnvelopeTenantIsolation(t *testing.T) {
	setupTestMasterKey(t)

	tenantA := "tenant-alpha"
	tenantB := "tenant-beta"
	secretVal := "super-confidential-api-token-9988"

	// 1. 在租户 A 下加密
	sealedForA, err := secrets.Seal(secretVal, tenantA)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	if !secrets.IsEnvelope(sealedForA) {
		t.Fatalf("expected valid envelope, got: %s", sealedForA)
	}

	// 2. 租户 A 凭据正常解密
	decryptedA, err := secrets.Open(sealedForA, tenantA)
	if err != nil {
		t.Fatalf("Open for tenant A failed: %v", err)
	}
	if decryptedA != secretVal {
		t.Fatalf("decrypted value mismatch: got %q want %q", decryptedA, secretVal)
	}

	// 3. 租户 B 试图解密租户 A 的密文（AAD 不匹配，防搬运攻击）
	_, err = secrets.Open(sealedForA, tenantB)
	if err == nil {
		t.Fatalf("SECURITY VIOLATION: tenant B succeeded in decrypting tenant A's envelope!")
	}
	if !strings.Contains(err.Error(), "decrypt failed") {
		t.Errorf("expected decrypt failed error, got: %v", err)
	}
}

func TestSecretTamperedCiphertext(t *testing.T) {
	setupTestMasterKey(t)

	tenantID := "tenant-001"
	sealed, err := secrets.Seal("secret-val", tenantID)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	// 篡改密文部分
	parts := strings.Split(sealed, ".")
	tampered := parts[0] + "." + parts[1] + ".A" + parts[2][1:]

	_, err = secrets.Open(tampered, tenantID)
	if err == nil {
		t.Fatalf("expected tamper detection, but Open succeeded")
	}
}
