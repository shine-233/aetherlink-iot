// 文件用途：自动化场景触发缓存的 Redis 读写与回收。
// 核心逻辑：按“单一设备”和“单类设备”两个维度组织 Redis 键，串联设备一级缓存、条件组缓存和动作缓存。
// 并发约定：Cache 是无状态的——维度作为参数逐调用传递，不再写回结构体字段。
// 旧实现把维度存在进程单例的可变字段里，每次读写先 SetDeviceType 再拼键，
// 两个 goroutine 交错时会用对方的维度拼键（one/multiple 串键），此前全靠
// Automate 的全局互斥锁掩盖；去掉那把锁后必须从结构上消除共享可变状态。
// 关键注意事项：键格式 automate:v3:{one|multiple}:{_|_group_|_action_}:{id} 与线上存量缓存一致，不可改动。

package automatecache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	global "aetherlink-iot/backend/pkg/global"

	"github.com/redis/go-redis/v9"
)

// DefaultTTL 缓存条目的过期时间。
const DefaultTTL = 5 * time.Minute

// Store 是缓存依赖的最小 Redis 能力面，便于在无 Redis 的单测里替换为内存实现。
type Store interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

// KeyDimension 描述设备维度，决定缓存前缀与条件类型分流规则。
type KeyDimension interface {
	GetAutomateCacheKeyPrefix() string // 设备id或者设备配置
	GetDeviceTriggerConditionType() string
}

var (
	dimensionOne      KeyDimension = NewOneDeviceCache()
	dimensionMultiple KeyDimension = NewMultipleDeviceCache()
)

// dimensionFor 单一设备走 one 维度，带设备配置 ID 走 multiple 维度。
func dimensionFor(deviceConfigID string) KeyDimension {
	if deviceConfigID == "" {
		return dimensionOne
	}
	return dimensionMultiple
}

// Cache 自动化触发缓存。零可变状态，可被任意多个 goroutine 并发使用。
type Cache struct {
	client    Store
	expiredIn time.Duration
}

// New 以给定存储构造缓存；ttl<=0 时使用 DefaultTTL。
func New(client Store, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{client: client, expiredIn: ttl}
}

var (
	defaultMu     sync.Mutex
	defaultCache  *Cache
	defaultClient *redis.Client
)

// Default 返回绑定 global.REDIS 的进程级缓存。
// global.REDIS 在启动期才赋值（测试里也可能被替换），客户端变化时重新绑定，
// 避免在 Redis 初始化前被调用就永久缓存一个 nil 客户端。
func Default() *Cache {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultCache == nil || defaultClient != global.REDIS {
		defaultClient = global.REDIS
		defaultCache = New(global.REDIS, DefaultTTL)
	}
	return defaultCache
}

func keyOf(dim KeyDimension, cType, id string) string {
	return fmt.Sprintf("automate:v3:%s:%s:%s", dim.GetAutomateCacheKeyPrefix(), cType, id)
}

// keyBase 设备一级缓存键，表示某设备（或设备配置）当前挂接的自动化场景集合。
func keyBase(dim KeyDimension, deviceID string) string { return keyOf(dim, "_", deviceID) }

// keyGroup 条件组缓存键。
func keyGroup(dim KeyDimension, groupID string) string { return keyOf(dim, "_group_", groupID) }

// keyAction 场景动作缓存键。
func keyAction(dim KeyDimension, sceneAutomationID string) string {
	return keyOf(dim, "_action_", sceneAutomationID)
}

// set 统一将字符串或结构体写入 Redis。
func (c *Cache) set(key string, value interface{}) error {
	var valueStr string
	if val, ok := value.(string); ok {
		valueStr = val
	} else {
		valBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("json marshal failed for key %s: %w", key, err)
		}
		valueStr = string(valBytes)
	}
	return c.client.Set(context.Background(), key, valueStr, c.expiredIn).Err()
}

// scan 识别“未命中”“明确无任务”“正常数据”三种返回状态。
func scan(stringCmd *redis.StringCmd, val interface{}) (int, error) {
	str, err := stringCmd.Result()
	if err == redis.Nil {
		return ResultNotFound, nil
	} else if err != nil {
		return 0, err
	}
	if str == ContentNoTask {
		return ResultNoTask, nil
	}
	return ResultOK, stringCmd.Scan(val)
}

func (c *Cache) get(key string) *redis.StringCmd {
	return c.client.Get(context.Background(), key)
}
