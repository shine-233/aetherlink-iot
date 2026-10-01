// 文件用途：维护 plugin\aetherlink\topicmap_cache.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

// topicmap_cache.go caches device topic mappings for the AetherLink plugin.
//
// Cache lifetime and invalidation affect high-frequency MQTT routing behavior,
// so changes should be tested with topic-map service cases.
package aetherlink

import (
	"context"
	"fmt"
	"time"
)

const (
	topicMapCacheTTL      = 24 * time.Hour
	topicMapEmptyCacheTTL = 5 * time.Minute
)

type cachedTopicMappings struct {
	Loaded bool                 `json:"loaded"`
	Rows   []DeviceTopicMapping `json:"rows"`
}

func cacheKeyUp(deviceConfigID string) string {
	return fmt.Sprintf("tp:topicmap:up:%s", deviceConfigID)
}

func cacheKeyDown(deviceConfigID string) string {
	return fmt.Sprintf("tp:topicmap:down:%s", deviceConfigID)
}

// getCompiledMappings 是热路径入口：先查进程内 LRU（已编译正则），未命中再走
// Redis→DB 的 GetMappingsWithCache 并编译一次。错误不缓存。
func getCompiledMappings(ctx context.Context, deviceConfigID string, direction Direction) ([]compiledTopicMapping, error) {
	client := redisCache
	key := topicMapLocalKey{deviceConfigID: deviceConfigID, direction: direction}
	if mappings, ok := topicMapLocal.get(client, key); ok {
		return mappings, nil
	}
	rows, err := GetMappingsWithCache(ctx, deviceConfigID, direction)
	if err != nil {
		return nil, err
	}
	compiled := compileTopicMappings(rows)
	topicMapLocal.put(client, key, compiled)
	return compiled, nil
}

func GetMappingsWithCache(ctx context.Context, deviceConfigID string, direction Direction) ([]DeviceTopicMapping, error) {
	var key string
	if direction == DirectionUp {
		key = cacheKeyUp(deviceConfigID)
	} else {
		key = cacheKeyDown(deviceConfigID)
	}

	var cached cachedTopicMappings
	if err := GetRedisForJsondata(key, &cached); err == nil && cached.Loaded {
		return cached.Rows, nil
	}

	// Compatibility with cache entries written before empty-result caching.
	var flatCached []DeviceTopicMapping
	if err := GetRedisForJsondata(key, &flatCached); err == nil && len(flatCached) > 0 {
		return flatCached, nil
	}

	rows, err := LoadEnabledMappings(ctx, deviceConfigID, direction)
	if err != nil {
		return nil, err
	}
	ttl := topicMapCacheTTL
	if len(rows) == 0 {
		ttl = topicMapEmptyCacheTTL
	}
	_ = SetRedisForJsondata(key, cachedTopicMappings{Loaded: true, Rows: rows}, ttl)
	return rows, nil
}
