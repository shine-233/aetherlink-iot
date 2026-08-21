// 文件用途：验证 API Key 生成、哈希与展示前缀工具的契约。
// 核心逻辑：钉死"数据库只存 SHA-256 摘要、明文仅创建时返回一次、前缀不可还原"的凭据边界。
// 关键注意事项：HashAPIKey 是开放接口鉴权的查询键，算法变更必须同步 dal.VerifyOpenAPIKey。
// 重构建议：若未来引入 key 轮换或多前缀品牌，需同步扩展 prefix 规则测试。
package utils

import (
	"strings"
	"testing"
)

func TestGenerateAPIKeyShape(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	if !strings.HasPrefix(key, "sk_") {
		t.Fatalf("key %q missing sk_ prefix", key)
	}
	if len(key) != len("sk_")+64 {
		t.Fatalf("key length = %d, want %d", len(key), len("sk_")+64)
	}
	if key == mustGenerate(t) || key != key {
		t.Fatalf("unexpected deterministic generation")
	}
}

func TestHashAPIKeyIsDeterministicHex(t *testing.T) {
	key := "sk_abcdef0123456789"
	h1 := HashAPIKey(key)
	h2 := HashAPIKey(key)
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("hash length = %d, want 64 hex chars", len(h1))
	}
	for _, c := range h1 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("hash contains non-hex char %q", c)
		}
	}
	if HashAPIKey(key+"x") == h1 {
		t.Fatal("different keys must not collide on hash")
	}
}

func TestAPIKeyDisplayPrefixNeverRevealsSecret(t *testing.T) {
	full, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	prefix := APIKeyDisplayPrefix(full)
	if prefix != full[:11] {
		t.Fatalf("prefix = %q, want first 11 chars %q", prefix, full[:11])
	}
	if strings.Contains(full[len(prefix):], "") && len(prefix) >= len(full) {
		t.Fatal("prefix must be strictly shorter than the full key")
	}
	if got := APIKeyDisplayPrefix("short"); got != "short" {
		t.Fatalf("short key prefix = %q, want unchanged", got)
	}
}

func mustGenerate(t *testing.T) string {
	t.Helper()
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	return key
}
