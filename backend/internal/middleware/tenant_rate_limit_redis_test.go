// 文件用途：限流 Redis 后端（集群共享配额）契约测试（miniredis）——固定窗口翻转、
// 独立配额、Retry-After 计算、memory 后端默认选择与 redis 缺失回退。
package middleware

import (
	"context"
	"testing"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisRateStoreFixedWindowAndIsolation(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := newRedisRateStore(client, 2)
	// 首窗口：2 次放行，第 3 次拒绝并给出 Retry-After。
	for i := 0; i < 2; i++ {
		allowed, retry := store.allow(context.Background(), "tenant-a")
		require.True(t, allowed, "request %d should pass", i+1)
		assert.Zero(t, retry)
	}
	allowed, retryAfter := store.allow(context.Background(), "tenant-a")
	assert.False(t, allowed)
	assert.Positive(t, retryAfter)
	assert.LessOrEqual(t, retryAfter, int64(61))

	// 租户隔离：tenant-b 有独立配额。
	allowed, _ = store.allow(context.Background(), "tenant-b")
	assert.True(t, allowed)

	// 窗口翻转：FastForward 越过窗口后计数重置（Redis 键过期）。
	server.FastForward(tenantRateWindow + time.Second)
	allowed, _ = store.allow(context.Background(), "tenant-a")
	assert.True(t, allowed, "new window must reset the counter")
}

func TestRedisRateStoreFailOpenOnRedisDown(t *testing.T) {
	// 无 server 的客户端：连接失败 → fail-open 放行，绝不阻塞业务。
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	store := newRedisRateStore(client, 1)
	allowed, retry := store.allow(context.Background(), "tenant-a")
	assert.True(t, allowed, "redis 故障必须 fail-open")
	assert.Zero(t, retry)
}

func TestTenantRateStoreFromConfigDefaults(t *testing.T) {
	key := tenantRateLimitBackendKey
	prevBackend := viper.Get(key)
	oldRedis := global.REDIS
	global.REDIS = nil
	t.Cleanup(func() {
		viper.Set(key, prevBackend)
		global.REDIS = oldRedis
	})

	// 默认 memory。
	viper.Set(key, "")
	store := tenantRateStoreFromConfig(10)
	_, ok := store.(*tenantRateLimiter)
	require.True(t, ok, "默认必须选择 memory 后端")

	// 显式 redis 但全局客户端未就绪 → 回退 memory（限流可降级不可缺失）。
	viper.Set(key, "redis")
	fallback := tenantRateStoreFromConfig(10)
	_, ok = fallback.(*tenantRateLimiter)
	require.True(t, ok, "Redis 未就绪必须回退 memory")
}
