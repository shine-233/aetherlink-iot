// 文件用途：设备路由信息的进程内短 TTL 微缓存，削减上行热路径的每消息 PostgreSQL 直查。
// 核心逻辑：上行分发对每条 PUBLISH 都要解析目标设备行；本缓存以
//
//	device_route_cache.ttl（默认 5s，env GMQTT_DEVICE_ROUTE_CACHE_TTL）为新鲜度窗口，
//	命中即免查库；容量上限 device_route_cache.max_entries（默认 4096）。
//
// 关键注意事项：缓存使 is_enabled/activate_flag 等鉴权相关状态最多延迟一个 TTL 生效——
//
//	与 backend jwt_auth 用户状态 30s 进程内缓存的既有权衡一致；查询失败会主动失效
//	旧条目（设备删除后立即回到权威判定）；凭证轮换/删除通道同时清除此缓存。
package aetherlink

import (
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultDeviceRouteCacheTTL        = 5 * time.Second
	defaultDeviceRouteCacheMaxEntries = 4096

	deviceRouteCacheTTLConfigKey        = "device_route_cache.ttl"
	deviceRouteCacheMaxEntriesConfigKey = "device_route_cache.max_entries"
)

type deviceRouteCacheEntry struct {
	device    Device
	expiresAt time.Time
}

type deviceRouteCache struct {
	mu      sync.RWMutex
	entries map[string]deviceRouteCacheEntry
	ttl     time.Duration
	max     int
}

func readDeviceRouteCacheTTL() time.Duration {
	raw := strings.TrimSpace(viper.GetString(deviceRouteCacheTTLConfigKey))
	if raw == "" {
		return defaultDeviceRouteCacheTTL
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	return defaultDeviceRouteCacheTTL
}

func readDeviceRouteCacheMaxEntries() int {
	if v := viper.GetInt(deviceRouteCacheMaxEntriesConfigKey); v > 0 {
		return v
	}
	return defaultDeviceRouteCacheMaxEntries
}

func newDeviceRouteCache(ttl time.Duration, maxEntries int) *deviceRouteCache {
	if ttl <= 0 {
		ttl = defaultDeviceRouteCacheTTL
	}
	if maxEntries <= 0 {
		maxEntries = defaultDeviceRouteCacheMaxEntries
	}
	return &deviceRouteCache{entries: make(map[string]deviceRouteCacheEntry), ttl: ttl, max: maxEntries}
}

// deviceRoute 是进程级单例；在 runtimeInit 完成 viper 装配后首次使用时读取配置。
var deviceRoute = newDeviceRouteCache(readDeviceRouteCacheTTL(), readDeviceRouteCacheMaxEntries())

// get 返回命中条目的浅拷贝指针；调用方不得据此写回存储。
func (c *deviceRouteCache) get(id string) (*Device, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, false
	}
	c.mu.RLock()
	entry, ok := c.entries[id]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	device := entry.device
	return &device, true
}

// set 写入条目。容量满时先做一轮惰性过期清理，仍满则放弃本次写入（回退直查），
// 不驱逐他人条目，避免缓存抖动。
func (c *deviceRouteCache) set(id string, device *Device) {
	id = strings.TrimSpace(id)
	if id == "" || device == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[id]; !exists && len(c.entries) >= c.max {
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.expiresAt) {
				delete(c.entries, key)
			}
		}
		if len(c.entries) >= c.max {
			return
		}
	}
	c.entries[id] = deviceRouteCacheEntry{device: *device, expiresAt: time.Now().Add(c.ttl)}
}

// invalidate 使单设备条目立即失效（负查询与跨服务失效通道共用）。
func (c *deviceRouteCache) invalidate(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, id)
	c.mu.Unlock()
}

// setDeviceRouteCacheForTest 替换进程级缓存实例（仅测试使用），返回恢复函数。
// 需要完全旁路缓存的测试可传入空容量实例：set 恒被拒、get 恒未命中，
// 使"每次查找都触达存储回调"的原有断言语义保持不变。
func setDeviceRouteCacheForTest(c *deviceRouteCache) (restore func()) {
	previous := deviceRoute
	deviceRoute = c
	return func() { deviceRoute = previous }
}
