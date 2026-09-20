// 文件用途：P2.3 分析查询缓存——轻量分析的取数缝 TTL 缓存（config 门控，默认关闭）。
// 核心逻辑：把 (设备, 键, 起点, 终点, 窗口, 聚合) 的聚合结果缓存一小段时间，
// 多设备对比重复读同一序列时不再反复打数据库。
//
// 关键注意事项：
//   - **进程内缓存，实例之间不共享**：多实例部署时各实例各自回源，
//     换来的是无外部依赖与失效语义简单；跨实例一致性需要 Redis 版本时再演进。
//   - TTL 期间未结束的窗口会读到稍旧的聚合值——分析场景可接受，
//     实时面板不走这条路径。
//   - 默认关闭：开启与否是部署方的时延/新鲜度取舍，不由代码替人决定。
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"

	"github.com/spf13/viper"
)

// telemetryColdWindowCutoffMs 冷窗口边界：早于该时间戳的窗口走冷层（telemetry_rollups）。
// 降采样未启用或遥测栈为外部 TSDB 时返回 0（无冷层）。
func telemetryColdWindowCutoffMs() int64 {
	if !viper.GetBool(telemetryDownsampleEnabledKey) {
		return 0
	}
	if !dal.TelemetryDownsamplingActive() {
		return 0
	}
	days := viper.GetInt64(telemetryDownsampleOlderThanDays)
	if days <= 0 {
		days = 90
	}
	return time.Now().UnixMilli() - days*24*3600*1000
}

// fetchTelemetryAnalysisSeries 分析取数的统一入口：
// 整个窗口都早于冷层边界时读 telemetry_rollups，否则走常规取数（可被缓存包裹）。
// 部分冷部分热的窗口整体走常规路径——跨层拼接会把两种误差源混进一张表。
func fetchTelemetryAnalysisSeries(
	fetch func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error),
	deviceID, key string, start, end, windowMs int64, aggregateFunc string, coldCutoff int64,
) ([]map[string]interface{}, error) {
	if coldCutoff > 0 && end <= coldCutoff {
		return dal.GetTelemetryRollupAggregate(deviceID, key, start, end, windowMs, aggregateFunc)
	}
	return fetch(deviceID, key, start, end, windowMs, aggregateFunc)
}

// 分析查询缓存配置键。
const (
	telemetryAnalysisCacheEnabledKey = "telemetry.analysis_cache.enabled"
	telemetryAnalysisCacheTTLKey     = "telemetry.analysis_cache.ttl_seconds"
)

// telemetryCacheEntry 一个缓存项。
type telemetryCacheEntry struct {
	values    []map[string]interface{}
	expiresAt time.Time
}

// telemetryFetchCache 取数结果的 TTL 缓存。
type telemetryFetchCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]telemetryCacheEntry
}

// newTelemetryFetchCache 从配置构造缓存；未启用返回 nil。
func newTelemetryFetchCache() *telemetryFetchCache {
	if !viper.GetBool(telemetryAnalysisCacheEnabledKey) {
		return nil
	}
	ttlSeconds := viper.GetInt64(telemetryAnalysisCacheTTLKey)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return &telemetryFetchCache{
		ttl:     time.Duration(ttlSeconds) * time.Second,
		entries: make(map[string]telemetryCacheEntry),
	}
}

// cacheKey 聚合取数的缓存键。参数拼接后哈希，避免超长键与分隔符歧义。
func telemetryFetchCacheKey(deviceID, key string, start, end, windowMs int64, aggregateFunc string) string {
	raw := deviceID + "\x00" + key + "\x00" +
		strconv.FormatInt(start, 10) + "\x00" +
		strconv.FormatInt(end, 10) + "\x00" +
		strconv.FormatInt(windowMs, 10) + "\x00" + aggregateFunc
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// wrap 给取数函数加缓存语义。命中返回副本切片，避免调用方修改污染缓存。
func (c *telemetryFetchCache) wrap(
	next func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error),
) func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
	return func(deviceID, key string, start, end, windowMs int64, aggregateFunc string) ([]map[string]interface{}, error) {
		cacheKey := telemetryFetchCacheKey(deviceID, key, start, end, windowMs, aggregateFunc)
		now := time.Now()
		c.mu.RLock()
		entry, ok := c.entries[cacheKey]
		c.mu.RUnlock()
		if ok && now.Before(entry.expiresAt) {
			values := make([]map[string]interface{}, len(entry.values))
			copy(values, entry.values)
			return values, nil
		}
		fresh, err := next(deviceID, key, start, end, windowMs, aggregateFunc)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		// 简单的容量护栏：条目数超限时整体清空。分析缓存是加速器，不是数据副本。
		if len(c.entries) >= 4096 {
			c.entries = make(map[string]telemetryCacheEntry)
		}
		c.entries[cacheKey] = telemetryCacheEntry{values: fresh, expiresAt: now.Add(c.ttl)}
		c.mu.Unlock()
		return fresh, nil
	}
}
