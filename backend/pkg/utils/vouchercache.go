// 文件用途：提供设备凭证（voucher）Redis 缓存键的统一构造能力。
// 核心逻辑：VoucherCacheKey 返回完整 voucher 的 SHA-256 十六进制摘要，作为设备凭证缓存统一使用的 Redis 键。
// 关键注意事项（跨服务契约）：此算法必须与 mqtt-broker/plugin/aetherlink/db.go 的 voucherCacheKey 保持一致，
// 任一侧变更需双端同步修改并同步更新契约测试（backend/pkg/utils/vouchercache_test.go 与
// mqtt-broker/plugin/aetherlink/voucher_cache_key_test.go），否则换凭证/删设备后将无法正确失效 broker 侧缓存，
// 导致旧凭证在缓存 TTL 内仍可通过 MQTT 认证。

package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// VoucherCacheKey 返回设备凭证缓存键。键由完整 voucher 的 SHA-256 摘要构成，
// 避免把携带明文 MQTT 口令的 voucher JSON 直接作为 Redis key 落地。
// 跨服务契约：此算法必须与 mqtt-broker/plugin/aetherlink/db.go voucherCacheKey 保持一致，
// 任一侧变更需双端同步修改并同步更新契约测试（backend/pkg/utils/vouchercache_test.go 与
// mqtt-broker/plugin/aetherlink/voucher_cache_key_test.go）。
func VoucherCacheKey(voucher string) string {
	sum := sha256.Sum256([]byte(voucher))
	return hex.EncodeToString(sum[:])
}
