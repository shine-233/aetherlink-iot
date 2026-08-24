// 文件用途：锁定 backend 侧设备凭证缓存键的跨服务契约测试。
// 核心逻辑：用硬编码的已知 voucher 向量校验 VoucherCacheKey 输出，防止哈希算法被无意改动。
// 关键注意事项：向量与期望值必须与 broker 侧契约测试保持字面一致。

package utils

import (
	"strings"
	"testing"
)

// TestVoucherCacheKeyContract 锁定 backend 与 mqtt-broker 双端的 voucher 缓存键契约。
// 契约参考：mqtt-broker/plugin/aetherlink/voucher_cache_key_test.go（TestVoucherCacheKeyIsStableHash）。
// 若修改 pkg/utils/vouchercache.go 的算法，必须同步修改 mqtt-broker/plugin/aetherlink/db.go
// 的 voucherCacheKey 以及两侧契约测试中的向量与期望值。
func TestVoucherCacheKeyContract(t *testing.T) {
	// 已知向量与 broker 侧契约测试使用同一字符串，保证双端字面一致。
	voucher := `{"username":"dev-1","password":"s3cret-pass"}`

	// 该值为上述向量的 HMAC-SHA256 十六进制摘要，独立计算后写死，防止实现自证。
	const expected = "f5aafa8b1e2369a8281b4aa74a844433e04852bf3193fdf2efc6593fad8a4a7a"

	got := VoucherCacheKey(voucher)
	if got != expected {
		t.Fatalf("VoucherCacheKey = %q, want contract hmac-sha256 hex %q", got, expected)
	}
	if strings.ContainsAny(got, "{}\":, ") || strings.Contains(got, "s3cret") {
		t.Fatal("cache key must not embed plaintext voucher material")
	}
	if VoucherCacheKey(voucher) != got {
		t.Fatal("VoucherCacheKey must be deterministic for identical input")
	}
	if VoucherCacheKey(voucher+" ") == got {
		t.Fatal("different vouchers must map to different cache keys")
	}
}
