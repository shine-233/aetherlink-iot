// 文件用途：TB-17R 传输限额发布器（PublishTransportLimit）单元测试——键契约、取值/TTL 与降级分支。
// 核心逻辑：假存储记录 Set/Del 调用，钉死"limit>0 写入带 TTL 的十进制字符串、limit<=0 删键、
// 空 tenant 拒绝"；另经 miniredis+global.REDIS 覆盖生产适配器的真实写路径。
// 关键注意事项：键字面量 aetherlink:quota:transport_limit: 为跨服务契约（broker 侧
// transport_quota.go 同名常量），本文件钉死防单侧漂移；不依赖真实 Redis。
// 重构建议：若增加发布字段（如 API 限额），扩展断言表而非新增测试函数。
package quota

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aetherlink-iot/backend/pkg/global"
)

// fakeTransportLimitStore 记录写入/删除调用的假存储。
type fakeTransportLimitStore struct {
	setKVP map[string]string // key → value
	setTTL map[string]time.Duration
	delKey map[string]struct{}
	err    error // 注入 Set 失败
}

func newFakeTransportLimitStore() *fakeTransportLimitStore {
	return &fakeTransportLimitStore{
		setKVP: map[string]string{},
		setTTL: map[string]time.Duration{},
		delKey: map[string]struct{}{},
	}
}

func (f *fakeTransportLimitStore) SetRaw(key, value string, ttl time.Duration) error {
	if f.err != nil {
		return f.err
	}
	f.setKVP[key] = value
	f.setTTL[key] = ttl
	return nil
}

func (f *fakeTransportLimitStore) DelRaw(key string) error {
	delete(f.setKVP, key)
	delete(f.setTTL, key)
	f.delKey[key] = struct{}{}
	return nil
}

// TestTransportLimitKeyPinsCrossServiceContract 钉死跨服务键契约：
// mqtt-broker/plugin/aetherlink/transport_quota.go 的 transportLimitKeyPrefix 必须与本侧一致。
func TestTransportLimitKeyPinsCrossServiceContract(t *testing.T) {
	const contractPrefix = "aetherlink:quota:transport_limit:"
	assert.Equal(t, contractPrefix, TransportLimitKeyPrefix)
	assert.Equal(t, contractPrefix+"tenant-1", TransportLimitKey("tenant-1"))
}

// TestPublishTransportLimitWritesValueWithTTL limit>0：写入十进制字符串，TTL 为跨服务契约常量。
func TestPublishTransportLimitWritesValueWithTTL(t *testing.T) {
	store := newFakeTransportLimitStore()
	require.NoError(t, PublishTransportLimitWith(store, "tenant-1", 10000))

	val, ok := store.setKVP["aetherlink:quota:transport_limit:tenant-1"]
	require.True(t, ok, "limit cache key must be written")
	assert.Equal(t, "10000", val)
	assert.Equal(t, TransportLimitTTL, store.setTTL["aetherlink:quota:transport_limit:tenant-1"])
	assert.Empty(t, store.delKey)
}

// TestPublishTransportLimitDeletesKeyForUnconfigured limit<=0：删键，让 broker 回到不执法。
func TestPublishTransportLimitDeletesKeyForUnconfigured(t *testing.T) {
	for _, limit := range []int64{0, -1} {
		store := newFakeTransportLimitStore()
		store.setKVP["aetherlink:quota:transport_limit:tenant-1"] = "stale" // 预置陈旧值
		require.NoError(t, PublishTransportLimitWith(store, "tenant-1", limit))
		assert.NotContains(t, store.setKVP, "aetherlink:quota:transport_limit:tenant-1", "limit=%d", limit)
		assert.Contains(t, store.delKey, "aetherlink:quota:transport_limit:tenant-1", "limit=%d", limit)
	}
}

// TestPublishTransportLimitRejectsEmptyTenant 空/空白 tenant 拒绝发布（fail-closed 于入参侧）。
func TestPublishTransportLimitRejectsEmptyTenant(t *testing.T) {
	for _, tenantID := range []string{"", "   "} {
		store := newFakeTransportLimitStore()
		require.Error(t, PublishTransportLimitWith(store, tenantID, 100))
		assert.Empty(t, store.setKVP)
		assert.Empty(t, store.delKey)
	}
	require.Error(t, PublishTransportLimitWith(nil, "tenant-1", 100))
}

// TestPublishTransportLimitPropagatesStoreError 存储失败原样上抛（调用方告警而非阻断主流程）。
func TestPublishTransportLimitPropagatesStoreError(t *testing.T) {
	store := newFakeTransportLimitStore()
	store.err = errors.New("redis down")
	err := PublishTransportLimitWith(store, "tenant-1", 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis down")
}

// TestPublishTransportLimitAgainstMiniredis 生产适配器端到端：global.REDIS 真实写入可读回。
func TestPublishTransportLimitAgainstMiniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	oldRedis := global.REDIS
	global.REDIS = rdb
	t.Cleanup(func() { global.REDIS = oldRedis })

	require.NoError(t, PublishTransportLimit("tenant-1", 500000))
	val, err := mr.Get("aetherlink:quota:transport_limit:tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "500000", val)
	assert.Greater(t, mr.TTL("aetherlink:quota:transport_limit:tenant-1"), time.Duration(0), "published key must carry TTL")

	// 未配置阈值：删除已发布的键。
	require.NoError(t, PublishTransportLimit("tenant-1", 0))
	_, err = mr.Get("aetherlink:quota:transport_limit:tenant-1")
	require.Error(t, err, "unconfigured limit must delete the published key")

	// Redis 未初始化：返回错误（调用方 fail-open 告警），不 panic。
	global.REDIS = nil
	require.Error(t, PublishTransportLimit("tenant-1", 100))
}
