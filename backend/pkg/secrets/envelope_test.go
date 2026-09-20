// 文件用途：信封加密的定向证据（ROADMAP P0.7 门禁）。
// 覆盖：明文不落库、主密钥缺失/非法 fail closed、轮换窗口读旧密文并可重加密、
//
//	跨租户 AAD 绑定、掩码不可还原。
package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// configureKeys 用随机 32 字节主密钥装载 viper（每个用例独立，避免污染全局配置）。
func configureKeys(t *testing.T, active string, ids ...string) {
	t.Helper()
	viper.Reset()
	for _, id := range ids {
		buf := make([]byte, keyBytes)
		if _, err := rand.Read(buf); err != nil {
			t.Fatalf("generate key: %v", err)
		}
		viper.Set("secrets.master_keys."+id, base64.StdEncoding.EncodeToString(buf))
	}
	if active != "" {
		viper.Set("secrets.active_key_id", active)
	}
	t.Cleanup(viper.Reset)
}

func TestSealProducesEnvelopeWithoutPlaintext(t *testing.T) {
	configureKeys(t, "k1", "k1")
	const plain = "sk-tenant-secret-value-123456"

	sealed, err := Seal(plain, "tenant-a")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if !IsEnvelope(sealed) {
		t.Fatalf("expected envelope, got %q", sealed)
	}
	// 门禁：数据库与日志不出现明文密钥。
	if strings.Contains(sealed, plain) || strings.Contains(sealed, "sk-tenant-secret") {
		t.Fatalf("ciphertext leaks plaintext: %q", sealed)
	}

	got, err := Open(sealed, "tenant-a")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != plain {
		t.Fatalf("round trip mismatch: %q", got)
	}
}

func TestSealFailsClosedWhenMasterKeyMissing(t *testing.T) {
	// active_key_id 未配置。
	configureKeys(t, "", "k1")
	if _, err := Seal("plain", "tenant-a"); !errors.Is(err, ErrMasterKeyUnavailable) {
		t.Fatalf("expected ErrMasterKeyUnavailable, got %v", err)
	}

	// active_key_id 指向未配置的 key id。
	configureKeys(t, "missing", "k1")
	if _, err := Seal("plain", "tenant-a"); !errors.Is(err, ErrMasterKeyUnavailable) {
		t.Fatalf("expected ErrMasterKeyUnavailable, got %v", err)
	}

	// 关键：失败时绝不返回明文兜底。
	got, err := Seal("plain", "tenant-a")
	if err == nil {
		t.Fatalf("expected failure, got %q", got)
	}
	if strings.Contains(got, "plain") {
		t.Fatalf("fail-open leak: %q", got)
	}
}

func TestSealFailsClosedWhenKeyWrongLengthOrNotBase64(t *testing.T) {
	viper.Reset()
	viper.Set("secrets.active_key_id", "k1")
	// 16 字节（AES-128）也不接受：门禁要求 AES-256。
	viper.Set("secrets.master_keys.k1", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	t.Cleanup(viper.Reset)
	if _, err := Seal("plain"); !errors.Is(err, ErrMasterKeyUnavailable) {
		t.Fatalf("expected ErrMasterKeyUnavailable for short key, got %v", err)
	}

	viper.Reset()
	viper.Set("secrets.active_key_id", "k1")
	viper.Set("secrets.master_keys.k1", "not-valid-base64!!")
	if _, err := Seal("plain"); !errors.Is(err, ErrMasterKeyUnavailable) {
		t.Fatalf("expected ErrMasterKeyUnavailable for bad base64, got %v", err)
	}
}

func TestRotationWindowReadsOldCiphertextAndReseals(t *testing.T) {
	configureKeys(t, "k1", "k1", "k0")

	// 历史密文由旧主密钥 k0 产生。
	legacy, err := SealWithKey("k0", "sk-old", "tenant-a")
	if err != nil {
		t.Fatalf("SealWithKey(k0): %v", err)
	}

	// 轮换后（active=k1）旧密文仍可读。
	got, err := Open(legacy, "tenant-a")
	if err != nil {
		t.Fatalf("old ciphertext must stay readable in rotation window: %v", err)
	}
	if got != "sk-old" {
		t.Fatalf("unexpected plaintext: %q", got)
	}

	// 且被识别为需要重加密。
	if !NeedsReseal(legacy) {
		t.Fatal("expected legacy ciphertext to need reseal")
	}

	// 重加密后落到当前主密钥，不再需要重加密。
	resealed, err := Seal(got, "tenant-a")
	if err != nil {
		t.Fatalf("resal: %v", err)
	}
	id, err := KeyIDOf(resealed)
	if err != nil {
		t.Fatalf("KeyIDOf: %v", err)
	}
	if id != "k1" {
		t.Fatalf("expected reseal onto k1, got %s", id)
	}
	if NeedsReseal(resealed) {
		t.Fatal("fresh ciphertext must not need reseal")
	}
	if again, err := Open(resealed, "tenant-a"); err != nil || again != "sk-old" {
		t.Fatalf("reseal round trip failed: %q %v", again, err)
	}
}

func TestAADBindsTenantAndRejectsCiphertextMove(t *testing.T) {
	configureKeys(t, "k1", "k1")
	sealed, err := Seal("sk-a", "tenant-a")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// 同租户 AAD 正常。
	if _, err := Open(sealed, "tenant-a"); err != nil {
		t.Fatalf("same-tenant open must succeed: %v", err)
	}
	// 跨租户搬运：AAD 不匹配必须认证失败。
	if _, err := Open(sealed, "tenant-b"); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("expected ErrDecryptFailed for cross-tenant AAD, got %v", err)
	}
	// 无 AAD 打开同样失败。
	if _, err := Open(sealed); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("expected ErrDecryptFailed without AAD, got %v", err)
	}
}

func TestOpenRejectsMalformedCiphertext(t *testing.T) {
	configureKeys(t, "k1", "k1")
	for _, bad := range []string{"", "plaintext-key", "aenv1.k1", "aenv2.k1.!!!!", "aenv1..AAAA"} {
		if _, err := Open(bad, "tenant-a"); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestNeedsResealTreatsLegacyPlaintextAsResealable(t *testing.T) {
	configureKeys(t, "k1", "k1")
	// 迁移前遗留明文行：必须被识别为需要重新封装。
	if !NeedsReseal("sk-legacy-plaintext") {
		t.Fatal("legacy plaintext must need reseal")
	}
	if IsEnvelope("sk-legacy-plaintext") {
		t.Fatal("legacy plaintext must not look like an envelope")
	}
}

func TestMaskIsIrreversible(t *testing.T) {
	if got := Mask("sk-abcdefgh", 4); got != "sk-a****" {
		t.Fatalf("unexpected mask: %q", got)
	}
	// 短于 head 时全掩码，不泄漏长度或内容。
	if got := Mask("sk", 4); got != "****" {
		t.Fatalf("short value must fully mask, got %q", got)
	}
	if got := Mask("sk-abcdefgh", 0); got != "****" {
		t.Fatalf("zero head must fully mask, got %q", got)
	}
	// 掩码不可逆：不含原始尾部。
	if strings.Contains(Mask("sk-abcdefgh", 4), "abcdefgh") {
		t.Fatal("mask leaks original tail")
	}
}
