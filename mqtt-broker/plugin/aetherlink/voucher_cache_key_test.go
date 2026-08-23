package aetherlink

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// TestVoucherCacheKeyIsStableHash 锁定 voucher 缓存键契约：
// 键必须是完整 voucher 的 SHA-256 十六进制摘要，且不得包含明文 voucher 片段，
// 防止携带 MQTT 明文口令的凭证 JSON 作为 Redis key 落地。
func TestVoucherCacheKeyIsStableHash(t *testing.T) {
	voucher := `{"username":"dev-1","password":"s3cret-pass"}`

	want := sha256.Sum256([]byte(voucher))
	expected := hex.EncodeToString(want[:])
	got := voucherCacheKey(voucher)

	if got != expected {
		t.Fatalf("voucherCacheKey = %q, want stable sha256 hex %q", got, expected)
	}
	if strings.Contains(got, "s3cret") || strings.Contains(got, "dev-1") {
		t.Fatal("cache key must not embed plaintext voucher material")
	}

	if voucherCacheKey(voucher) != got {
		t.Fatal("voucherCacheKey must be deterministic for identical input")
	}
	if voucherCacheKey(voucher+" ") == got {
		t.Fatal("different vouchers must map to different cache keys")
	}
}
