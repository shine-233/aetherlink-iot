// 文件用途：JWT 会话 Redis 键摘要的契约测试，锁定 TokenDigest 的算法与确定性。
// 核心逻辑：对已知输入断言 HMAC-SHA256 十六进制摘要值，防止键空间算法漂移导致会话读写错位。
// 关键注意事项：middleware/jwt_auth.go、api/telemetry_ws_auth.go、service/sys_user_auth.go
// 三端共用该函数；修改算法必须同步检查三处调用点与既有 Redis 键的迁移策略。

package utils

import (
	"strings"
	"testing"
)

func TestTokenDigestContract(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.contract-token"

	// 已知输入的 HMAC-SHA256 十六进制摘要；修改算法时此断言会失败以提示跨链路同步。
	const expected = "94e622e4458d5a7dfb4c67ded6f88c2b7db39dad10619c3a10b5a0f413dac531"

	got := TokenDigest(token)
	if got != expected {
		t.Fatalf("TokenDigest = %q, want contract hmac-sha256 hex %q", got, expected)
	}
	if strings.Contains(got, "contract") {
		t.Fatal("digest must not embed plaintext token material")
	}
	if TokenDigest(token) != got {
		t.Fatal("TokenDigest must be deterministic for identical input")
	}
	if TokenDigest(token+" ") == got {
		t.Fatal("different tokens must map to different digests")
	}
	// 域分离：同一输入在 voucher 缓存键与 JWT 会话键下不得产生相同摘要。
	if VoucherCacheKey(token) == got {
		t.Fatal("TokenDigest must be domain-separated from VoucherCacheKey")
	}
}
