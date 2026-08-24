// 文件用途：提供 JWT 会话 Redis 键的统一摘要构造能力。
// 核心逻辑：TokenDigest 返回 JWT token 的 HMAC-SHA256 十六进制摘要，作为登录会话在
// Redis 中的键；HTTP 中间件、WebSocket 认证与 service 层 login/logout/refresh 共用同一函数。
// 关键注意事项（跨链路契约）：jwt_auth.go、api/telemetry_ws_auth.go、service/sys_user_auth.go
// 必须全部经由本函数生成键，任一侧绕过将导致会话读写错位（表现为登录后立即 401）；
// 算法与 VoucherCacheKey 同构（HMAC-SHA256hex），但域分离密钥不同，二者不可互换。

package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// tokenDigestKey 是 JWT 会话键摘要的 HMAC 域分离密钥。它不是安全秘密——用途是
// 让摘要与裸 SHA-256 域分离并满足键控哈希要求，确定性由 HMAC 构造保证。
const tokenDigestKey = "aetherlink:jwt-session-key:v1"

// TokenDigest 返回 JWT token 的 Redis 键摘要（hmac-sha256hex），
// 避免把完整明文 JWT 直接作为 Redis key 落地（防键空间泄漏/MONITOR 暴露凭证）。
func TokenDigest(token string) string {
	mac := hmac.New(sha256.New, []byte(tokenDigestKey))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}
