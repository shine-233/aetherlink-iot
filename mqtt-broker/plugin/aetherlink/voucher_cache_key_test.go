package aetherlink

import (
	"strings"
	"testing"
)

// TestVoucherCacheKeyIsStableHash 锁定 voucher 缓存键契约。
// 期望值：与 backend/pkg/utils/vouchercache_test.go 使用同一向量，保证双端字面一致。
// 算法：HMAC-SHA256（域分离密钥 aetherlink:voucher-cache:v1），满足 CodeQL 键控哈希要求。
func TestVoucherCacheKeyIsStableHash(t *testing.T) {
	voucher := `{"username":"dev-1","password":"s3cret-pass"}`

	const expected = "f5aafa8b1e2369a8281b4aa74a844433e04852bf3193fdf2efc6593fad8a4a7a"

	got := voucherCacheKey(voucher)

	if got != expected {
		t.Fatalf("voucherCacheKey = %q, want stable hmac-sha256 hex %q", got, expected)
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
