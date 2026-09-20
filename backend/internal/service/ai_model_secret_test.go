// 文件用途：AI 模型凭证在服务层的静态加密证据（ROADMAP P0.7 门禁）。
// 覆盖：明文不落库、主密钥缺失 fail closed、跨租户密文搬运被拒、
//
//	遗留明文可读且被标记重封装、出参掩码不泄漏。
package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/secrets"
	"github.com/spf13/viper"
)

// configureSecrets 装载随机主密钥，避免写入任何真实密钥材料。
func configureSecrets(t *testing.T, active string, ids ...string) {
	t.Helper()
	viper.Reset()
	for _, id := range ids {
		buf := make([]byte, 32)
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

func TestSealModelAPIKeyNeverPersistsPlaintext(t *testing.T) {
	configureSecrets(t, "k1", "k1")
	const plain = "sk-live-tenant-a-abcdefghijklmnop"
	m := &model.AiModel{ID: "m1", TenantID: "tenant-a"}

	if err := sealModelAPIKey(m, plain); err != nil {
		t.Fatalf("sealModelAPIKey: %v", err)
	}
	// 门禁：数据库与日志不出现明文密钥。
	if strings.Contains(m.APIKey, plain) {
		t.Fatalf("persisted value contains plaintext: %q", m.APIKey)
	}
	if !secrets.IsEnvelope(m.APIKey) {
		t.Fatalf("expected envelope, got %q", m.APIKey)
	}

	got, needsReseal, err := openModelAPIKey(m)
	if err != nil {
		t.Fatalf("openModelAPIKey: %v", err)
	}
	if got != plain {
		t.Fatalf("round trip mismatch: %q", got)
	}
	if needsReseal {
		t.Fatal("fresh ciphertext must not need reseal")
	}
}

func TestSealModelAPIKeyFailsClosedWithoutMasterKey(t *testing.T) {
	// active_key_id 未配置：必须失败，且绝不把明文写入档案。
	configureSecrets(t, "", "k1")
	m := &model.AiModel{ID: "m1", TenantID: "tenant-a"}
	err := sealModelAPIKey(m, "sk-plain")
	if err == nil {
		t.Fatal("expected failure when master key is unconfigured")
	}
	if !errors.Is(err, secrets.ErrMasterKeyUnavailable) {
		t.Fatalf("expected ErrMasterKeyUnavailable, got %v", err)
	}
	// 关键：失败路径不得留下明文。
	if m.APIKey != "" {
		t.Fatalf("fail-open leak, model holds: %q", m.APIKey)
	}
}

func TestOpenModelAPIKeyRejectsCrossTenantCiphertextMove(t *testing.T) {
	configureSecrets(t, "k1", "k1")
	m := &model.AiModel{ID: "m1", TenantID: "tenant-a"}
	if err := sealModelAPIKey(m, "sk-tenant-a-secret"); err != nil {
		t.Fatalf("seal: %v", err)
	}

	// 把整行搬到另一个租户（模拟越权读取/数据搬运）。
	m.TenantID = "tenant-b"
	if _, _, err := openModelAPIKey(m); !errors.Is(err, secrets.ErrDecryptFailed) {
		t.Fatalf("expected ErrDecryptFailed for cross-tenant ciphertext, got %v", err)
	}
}

func TestOpenModelAPIKeyLegacyPlaintextIsReadableAndResealable(t *testing.T) {
	configureSecrets(t, "k1", "k1")
	// 迁移前遗留明文行。
	m := &model.AiModel{ID: "m1", TenantID: "tenant-a", APIKey: "sk-legacy-plain"}

	got, needsReseal, err := openModelAPIKey(m)
	if err != nil {
		t.Fatalf("legacy plaintext must stay readable during migration: %v", err)
	}
	if got != "sk-legacy-plain" {
		t.Fatalf("unexpected value: %q", got)
	}
	if !needsReseal {
		t.Fatal("legacy plaintext must be flagged for reseal")
	}

	// 重新封装后不再需要重加密，且明文仍一致。
	if err := sealModelAPIKey(m, got); err != nil {
		t.Fatalf("reseal: %v", err)
	}
	again, needsReseal, err := openModelAPIKey(m)
	if err != nil || again != "sk-legacy-plain" || needsReseal {
		t.Fatalf("after reseal: %q reseal=%v err=%v", again, needsReseal, err)
	}
}

func TestAiModelMaskedNeverLeaksPlaintext(t *testing.T) {
	m := &model.AiModel{ID: "m1", TenantID: "tenant-a"}
	resp := aiModelMasked(m, "sk-live-abcdefghijklmnop")
	if resp.APIKeyMasked != "sk-l****" {
		t.Fatalf("unexpected mask: %q", resp.APIKeyMasked)
	}
	if strings.Contains(resp.APIKeyMasked, "abcdefghijklmnop") {
		t.Fatal("mask leaks secret tail")
	}
	// 解密失败（空明文）时只出全掩码，不回显密文。
	if got := aiModelMasked(m, "").APIKeyMasked; got != "****" {
		t.Fatalf("expected full mask on decrypt failure, got %q", got)
	}
	// 短密钥不得泄漏长度或内容。
	if got := aiModelMasked(m, "sk").APIKeyMasked; got != "****" {
		t.Fatalf("short key must fully mask, got %q", got)
	}
}
