// 文件用途：OnMsgArrived 等每条 PUBLISH 都会走到的 Redis 读取的进程内短 TTL 缓存。
// 核心逻辑：
//   - 设备调试配置（tp:devdebug:cfg:<device>）：每条上行/下行诊断事件原先都要 GET 一次，
//     绝大多数设备未开启调试，因此以 devDebugCfgCacheTTL 缓存"是否存在/配置内容"，
//     负结果同样缓存；到期判断（expire_at）仍在每次使用时按当前时间重新计算。
//   - 主题映射（tp:topicmap:{up,down}:<config>）：原先每条上行都 GET + JSON 反序列化，
//     并对每条映射重新拼接正则串再查 sync.Map；现缓存"已编译"的映射切片（LRU + TTL）。
//
// 关键注意事项：没有跨进程失效通道（backend 仅删除 Redis 键），因此新鲜度上界即 TTL；
// 本进程 InvalidateMappingCache 会同步清掉本地条目。条目绑定写入时的 redisCache 客户端，
// 客户端被替换（重连、测试换 miniredis）时自动视为未命中。
package aetherlink

import (
	"container/list"
	"regexp"
	"sync"
	"time"

	"gopkg.in/redis.v5"
)

const (
	devDebugCfgCacheTTL        = 3 * time.Second
	devDebugCfgCacheMaxEntries = 16384

	topicMapLocalCacheTTL        = 5 * time.Second
	topicMapLocalCacheMaxEntries = 4096
)

var hotpathCacheNow = time.Now

// ---- device debug config ----

type devDebugCfgCacheEntry struct {
	client    *redis.Client
	cfg       DeviceDebugConfig
	present   bool
	expiresAt time.Time
}

type devDebugCfgCache struct {
	mu      sync.RWMutex
	entries map[string]devDebugCfgCacheEntry
}

var devDebugCfgLocal = &devDebugCfgCache{entries: map[string]devDebugCfgCacheEntry{}}

func (c *devDebugCfgCache) get(client *redis.Client, deviceID string) (DeviceDebugConfig, bool, bool) {
	c.mu.RLock()
	entry, ok := c.entries[deviceID]
	c.mu.RUnlock()
	if !ok || entry.client != client || !hotpathCacheNow().Before(entry.expiresAt) {
		return DeviceDebugConfig{}, false, false
	}
	return entry.cfg, entry.present, true
}

func (c *devDebugCfgCache) put(client *redis.Client, deviceID string, cfg DeviceDebugConfig, present bool) {
	c.mu.Lock()
	if len(c.entries) >= devDebugCfgCacheMaxEntries {
		// 容量兜底：整体清空而非逐条淘汰；最坏只是多一轮 Redis 读取。
		c.entries = make(map[string]devDebugCfgCacheEntry, len(c.entries)/2)
	}
	c.entries[deviceID] = devDebugCfgCacheEntry{
		client:    client,
		cfg:       cfg,
		present:   present,
		expiresAt: hotpathCacheNow().Add(devDebugCfgCacheTTL),
	}
	c.mu.Unlock()
}

func (c *devDebugCfgCache) invalidate(deviceID string) {
	c.mu.Lock()
	delete(c.entries, deviceID)
	c.mu.Unlock()
}

// InvalidateDeviceDebugConfigCache 丢弃指定设备的本地调试配置缓存。
func InvalidateDeviceDebugConfigCache(deviceID string) {
	devDebugCfgLocal.invalidate(deviceID)
}

// ---- compiled topic mappings ----

// compiledTopicMapping 是预编译好源/目标正则的映射行；正则为 nil 表示该方向模式非法
// （例如含 '#'），与逐条 compile 失败时 continue 的旧语义一致。
type compiledTopicMapping struct {
	DeviceTopicMapping
	sourceRx *regexp.Regexp
	targetRx *regexp.Regexp
}

func compileTopicMappings(rows []DeviceTopicMapping) []compiledTopicMapping {
	out := make([]compiledTopicMapping, len(rows))
	for i, row := range rows {
		out[i].DeviceTopicMapping = row
		if rx, ok := compileSourcePattern(row.SourceTopic); ok {
			out[i].sourceRx = rx
		}
		if rx, ok := compileTargetPattern(row.TargetTopic); ok {
			out[i].targetRx = rx
		}
	}
	return out
}

type topicMapLocalKey struct {
	deviceConfigID string
	direction      Direction
}

type topicMapLocalEntry struct {
	key       topicMapLocalKey
	client    *redis.Client
	mappings  []compiledTopicMapping
	expiresAt time.Time
}

type topicMapLocalCache struct {
	mu    sync.Mutex
	max   int
	ll    *list.List
	items map[topicMapLocalKey]*list.Element
}

func newTopicMapLocalCache(max int) *topicMapLocalCache {
	return &topicMapLocalCache{max: max, ll: list.New(), items: map[topicMapLocalKey]*list.Element{}}
}

var topicMapLocal = newTopicMapLocalCache(topicMapLocalCacheMaxEntries)

func (c *topicMapLocalCache) get(client *redis.Client, key topicMapLocalKey) ([]compiledTopicMapping, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*topicMapLocalEntry)
	if entry.client != client || !hotpathCacheNow().Before(entry.expiresAt) {
		c.ll.Remove(el)
		delete(c.items, key)
		return nil, false
	}
	c.ll.MoveToFront(el)
	return entry.mappings, true
}

func (c *topicMapLocalCache) put(client *redis.Client, key topicMapLocalKey, mappings []compiledTopicMapping) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := &topicMapLocalEntry{key: key, client: client, mappings: mappings, expiresAt: hotpathCacheNow().Add(topicMapLocalCacheTTL)}
	if el, ok := c.items[key]; ok {
		el.Value = entry
		c.ll.MoveToFront(el)
		return
	}
	c.items[key] = c.ll.PushFront(entry)
	for c.ll.Len() > c.max {
		back := c.ll.Back()
		c.ll.Remove(back)
		delete(c.items, back.Value.(*topicMapLocalEntry).key)
	}
}

func (c *topicMapLocalCache) invalidate(deviceConfigID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, dir := range []Direction{DirectionUp, DirectionDown} {
		key := topicMapLocalKey{deviceConfigID: deviceConfigID, direction: dir}
		if el, ok := c.items[key]; ok {
			c.ll.Remove(el)
			delete(c.items, key)
		}
	}
}

func (c *topicMapLocalCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
