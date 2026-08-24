// 文件用途：设备凭证网页测试缓存（凭证哈希存储 Phase 2b，见 references/backend-hardening-plan.md 车道1）。
// 核心逻辑：devices.voucher 列停写明文后，网页模拟器/调试回显（telemetry_simulation.go 的
// ServeEchoData / GetSimulationInit / SimulationSend 三处）无法再从 DB 读到明文凭证；
// 本缓存在六处凭证写路径落 voucher_hash 的同一收口点，以 SETEX 暂存"创建/轮换时的明文
// voucher JSON"，TTL 24 小时，保住"创建后 24h 内可直接网页测试"的产品体验。
// 关键注意事项（边界）：本缓存是 UX 增强，不是一致性依赖——
//  1. 写失败仅 Warn 不阻断主流程（设备认证匹配走 devices.voucher_hash 列，与缓存无关）；
//  2. 读 miss / Redis 故障一律 fail-closed 归一为 ErrCredentialCacheMiss，由调用方返回明确业务错误，
//     不区分"过期"与"故障"以避免向调用面泄漏基础设施状态；
//  3. 缓存过期不代表凭证失效，只代表网页测试入口需要轮换凭证后重新获取。
package dal

import (
	"context"
	"errors"
	"strings"
	"time"

	global "aetherlink-iot/backend/pkg/global"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// DeviceCredentialTestCacheTTL 网页测试缓存存活时间。产品语义为"创建后 24h 内可直接网页测试，
// 过期需轮换凭证重新获取"；调整该值等于调整产品承诺窗口，需同步 references/backend-hardening-plan.md。
const DeviceCredentialTestCacheTTL = 24 * time.Hour

// deviceCredentialTestCacheKeyPrefix 网页测试缓存键前缀；键形如 <prefix><deviceID>。
const deviceCredentialTestCacheKeyPrefix = "aetherlink:device_cred_test_cache:"

// ErrCredentialCacheMiss 测试缓存未命中哨兵（TTL 过期、键不存在与 Redis 故障 fail-closed 均归一到此）。
var ErrCredentialCacheMiss = errors.New("device credential test cache miss")

// deviceCredentialCacheStore 抽象缓存存取面。接口本身不导出，但方法集均为导出形态，
// 外部测试可用自有假实现隐式满足并替换 DeviceCredentialCacheStore。
type deviceCredentialCacheStore interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
}

type redisDeviceCredentialCacheStore struct{}

func (redisDeviceCredentialCacheStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if global.REDIS == nil {
		return errors.New("redis client is not initialized")
	}
	return global.REDIS.Set(ctx, key, value, ttl).Err()
}

func (redisDeviceCredentialCacheStore) Get(ctx context.Context, key string) (string, error) {
	if global.REDIS == nil {
		return "", errors.New("redis client is not initialized")
	}
	return global.REDIS.Get(ctx, key).Result()
}

// DeviceCredentialCacheStore 缓存存取的包级替换点（测试注入 seam，项目无 miniredis 依赖；
// 单测以假实现整体替换，风格参照 pkg/common/lock_test.go）。生产代码不得改写。
var DeviceCredentialCacheStore deviceCredentialCacheStore = redisDeviceCredentialCacheStore{}

func deviceCredentialTestCacheKey(deviceID string) string {
	return deviceCredentialTestCacheKeyPrefix + deviceID
}

// StoreDeviceCredentialTestCache 以 24h TTL 暂存设备当前明文 voucher JSON，供网页模拟器读侧使用。
// 仅在六处凭证写路径（createDevicesWithDefaultRootGroup 覆盖的四类创建 +
// persistAndReloadVoucher 凭证轮换）写 voucher_hash 的同一收口点调用；
// 失败仅 Warn 不阻断主流程——缓存是 UX 增强，不是一致性依赖。
func StoreDeviceCredentialTestCache(deviceID, voucherJSON string) {
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(voucherJSON) == "" {
		return
	}
	if err := DeviceCredentialCacheStore.Set(
		context.Background(), deviceCredentialTestCacheKey(deviceID), voucherJSON, DeviceCredentialTestCacheTTL,
	); err != nil {
		logrus.WithError(err).WithField("device_id", deviceID).
			Warn("store device credential test cache failed; web simulator test window unavailable for this device")
	}
}

// LoadDeviceCredentialTestCache 读取设备明文 voucher JSON 测试缓存。
// 未命中（含 TTL 过期、Redis 故障 fail-closed）统一返回 ErrCredentialCacheMiss。
func LoadDeviceCredentialTestCache(deviceID string) (string, error) {
	if strings.TrimSpace(deviceID) == "" {
		return "", ErrCredentialCacheMiss
	}
	value, err := DeviceCredentialCacheStore.Get(context.Background(), deviceCredentialTestCacheKey(deviceID))
	if err != nil {
		if !errors.Is(err, redis.Nil) && !errors.Is(err, ErrCredentialCacheMiss) {
			// Redis 故障按过期处理（fail-closed），日志保留定位线索但不向调用面区分。
			logrus.WithError(err).WithField("device_id", deviceID).Warn("load device credential test cache failed")
		}
		return "", ErrCredentialCacheMiss
	}
	if strings.TrimSpace(value) == "" {
		return "", ErrCredentialCacheMiss
	}
	return value, nil
}
