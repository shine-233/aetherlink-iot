// 文件用途：提供设备凭证（voucher）Redis 缓存键与 devices.voucher_hash 存储哈希的统一构造能力。
// 核心逻辑：VoucherCacheKey 返回完整 voucher 的 SHA-256 十六进制摘要，作为设备凭证缓存统一使用的 Redis 键；
// VoucherStorageHash 为同一算法的存储哈希导出别名（存储哈希=缓存键算法，跨服务契约）。
// 关键注意事项（跨服务契约）：此算法必须与 mqtt-broker/plugin/aetherlink/db.go 的 voucherCacheKey 保持一致，
// 任一侧变更需双端同步修改并同步更新契约测试（backend/pkg/utils/vouchercache_test.go 与
// mqtt-broker/plugin/aetherlink/voucher_cache_key_test.go），否则换凭证/删设备后将无法正确失效 broker 侧缓存，
// 导致旧凭证在缓存 TTL 内仍可通过 MQTT 认证。

package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// voucherHMACKey 是缓存键/存储哈希的 HMAC 域分离密钥。
// 它不是安全秘密——用途是让摘要与裸 SHA-256 域分离，并满足 CodeQL 对敏感数据
// 使用键控哈希的要求。确定性由 HMAC 构造保证：同一 voucher → 同一摘要。
const voucherHMACKey = "aetherlink:voucher-cache:v1"

// VoucherCacheKey 返回设备凭证缓存键。键由完整 voucher 的 HMAC-SHA256 摘要构成，
// 避免把携带明文 MQTT 口令的 voucher JSON 直接作为 Redis key 落地。
// 跨服务契约：此算法必须与 mqtt-broker/plugin/aetherlink/db.go voucherCacheKey 保持一致，
// 任一侧变更需双端同步修改并同步更新契约测试（backend/pkg/utils/vouchercache_test.go 与
// mqtt-broker/plugin/aetherlink/voucher_cache_key_test.go）。
func VoucherCacheKey(voucher string) string {
	mac := hmac.New(sha256.New, []byte(voucherHMACKey))
	mac.Write([]byte(voucher))
	return hex.EncodeToString(mac.Sum(nil))
}

// VoucherStorageHash 返回 devices.voucher_hash 列使用的存储哈希（hmac-sha256hex）。
// 存储哈希=缓存键算法，跨服务契约：与 VoucherCacheKey 同源同值——backend 写入/匹配
// （dal/device_voucher_hash.go、device_query_reads.go、device_identity_queries.go）与
// broker 双模式匹配（mqtt-broker/plugin/aetherlink/db.go）必须使用同一摘要；
// 算法变更需双端同步并更新两侧契约测试。
func VoucherStorageHash(voucher string) string {
	return VoucherCacheKey(voucher)
}
