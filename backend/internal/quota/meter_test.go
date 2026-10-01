// 文件用途：TB-17 计量器（DailyMeter）与限额来源（BillingLimitSource）的单元测试。
// 核心逻辑：进程内计数路径（无 Redis）+ miniredis 路径双向覆盖——同日累加、跨日重置、
// DB 种子对齐（重启不丢当日累计）、落库 GREATEST 快照、限额缓存与 stale 回退。
// 关键注意事项：DB/Redis 全部替身化，不依赖真实中间件；时钟注入保证跨日用例确定性。
// 重构建议：新增 Redis 集群语义用例时继续用 miniredis，保持无外部依赖。
package quota

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore 可编程的持久化替身：seed 返回 DB 快照值，persist 记录落库调用。
type fakeStore struct {
	mu      sync.Mutex
	seed    map[string]int64 // key: tenant|date
	persist map[string]int64
	seedErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{seed: map[string]int64{}, persist: map[string]int64{}}
}

func (f *fakeStore) key(tenantID, date string) string { return tenantID + "|" + date }

func (f *fakeStore) seedLoader(tenantID, date string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seedErr != nil {
		return 0, f.seedErr
	}
	return f.seed[f.key(tenantID, date)], nil
}

func (f *fakeStore) persistFunc(tenantID, date string, calls int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.persist[f.key(tenantID, date)] = calls
	return nil
}

func newTestMeter(rdb *redis.Client, store *fakeStore, now time.Time) *DailyMeter {
	m := NewDailyMeter(rdb, time.Hour)
	m.SetClock(func() time.Time { return now })
	m.SetPersistence(store.seedLoader, store.persistFunc)
	return m
}

func TestDailyMeterMemoryIncrAccumulatesPerDay(t *testing.T) {
	now := fixedNow
	store := newFakeStore()
	meter := newTestMeter(nil, store, now) // 无 Redis → 进程内计数

	for i := int64(1); i <= 3; i++ {
		count, err := meter.Incr(context.Background(), "tenant-a")
		require.NoError(t, err)
		assert.Equal(t, i, count)
	}

	usage, err := meter.TodayUsage(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, int64(3), usage)

	// 租户隔离：tenant-b 从 0 起计。
	count, err := meter.Incr(context.Background(), "tenant-b")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestDailyMeterResetsOnDayRollover(t *testing.T) {
	now := fixedNow
	store := newFakeStore()
	meter := newTestMeter(nil, store, now)

	_, err := meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err)
	_, err = meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err)

	// 跨日：镜像按日切分，次日从 DB 种子（空）+1 重新起计。
	meter.SetClock(func() time.Time {
		return fixedNow.Add(24 * time.Hour)
	})
	count, err := meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "new day must start a fresh counter")
}

func TestDailyMeterSeedsFromPersistedSnapshotOnColdStart(t *testing.T) {
	now := fixedNow
	store := newFakeStore()
	store.seed["tenant-a|2026-09-25"] = 1000 // DB 已有当日落库值

	meter := newTestMeter(nil, store, now)
	count, err := meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, int64(1001), count, "cold start must continue from persisted snapshot")

	usage, err := meter.TodayUsage(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, int64(1001), usage)
}

func TestDailyMeterRedisIncrSharesCountAndSetsTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	now := fixedNow
	store := newFakeStore()
	meter := newTestMeter(rdb, store, now)

	for i := int64(1); i <= 2; i++ {
		count, err := meter.Incr(context.Background(), "tenant-a")
		require.NoError(t, err)
		assert.Equal(t, i, count)
	}

	key := "aetherlink:quota:api_daily:2026-09-25:tenant-a"
	val, err := mr.Get(key)
	require.NoError(t, err)
	assert.Equal(t, "2", val)

	ttl := mr.TTL(key)
	assert.Greater(t, ttl, time.Duration(0), "daily key must carry a TTL")
	assert.LessOrEqual(t, ttl, 24*time.Hour+6*time.Minute, "TTL bounded to day end + buffer")
}

func TestDailyMeterRedisSeedsMissingKeyFromSnapshot(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	store := newFakeStore()
	store.seed["tenant-a|2026-09-25"] = 500 // Redis 数据丢失（重启）后的 DB 对齐

	meter := newTestMeter(rdb, store, fixedNow)
	count, err := meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, int64(501), count, "missing redis key must seed from DB snapshot")
}

func TestDailyMeterRedisFailureFallsBackToMemoryAndFailsOpen(t *testing.T) {
	// 指向已关闭的 miniredis，模拟 Redis 故障。
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer func() { _ = rdb.Close() }()

	store := newFakeStore()
	meter := newTestMeter(rdb, store, fixedNow)

	count, err := meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err, "metering must fail open, not error out the request path")
	assert.Equal(t, int64(1), count, "fallback in-memory counting still works")

	usage, err := meter.TodayUsage(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage)
}

func TestDailyMeterSeedErrorPropagatesAsFailOpenSignal(t *testing.T) {
	store := newFakeStore()
	store.seedErr = errors.New("db down")
	meter := newTestMeter(nil, store, fixedNow)

	_, err := meter.Incr(context.Background(), "tenant-a")
	assert.Error(t, err, "seed failure must surface so callers fail open")

	usage, err := meter.TodayUsage(context.Background(), "tenant-a")
	assert.Error(t, err)
	assert.Zero(t, usage)
}

func TestDailyMeterFlushOncePersistsCanonicalCounts(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	now := fixedNow
	store := newFakeStore()
	meter := newTestMeter(rdb, store, now)

	_, err := meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err)
	_, err = meter.Incr(context.Background(), "tenant-a")
	require.NoError(t, err)

	// 他实例把权威计数推到 9：落库必须取 Redis 权威值而非本进程镜像 2。
	require.NoError(t, mr.Set("aetherlink:quota:api_daily:2026-09-25:tenant-a", "9"))

	meter.FlushOnce(context.Background())
	assert.Equal(t, int64(9), store.persist["tenant-a|2026-09-25"],
		"flush must persist the canonical redis count")

	usage, err := meter.TodayUsage(context.Background(), "tenant-a")
	require.NoError(t, err)
	assert.Equal(t, int64(9), usage)
}

func TestBillingLimitSourceReadsPlanThroughDalCacheAndStaleFallback(t *testing.T) {
	// 无 DB 环境：dal 返回 not-initialized 错误 → 无缓存时报错（fail-open 信号）、
	// 有缓存时回退 stale 值。缓存命中路径用同实例注入缓存验证。
	source := NewBillingLimitSource(time.Minute)
	source.now = func() time.Time { return fixedNow }

	_, err := source.DailyAPILimit("tenant-a")
	assert.Error(t, err, "no cache + no db must surface error for fail-open")

	source.mu.Lock()
	source.cache["tenant-a"] = limitCacheEntry{limit: 5000, expiresAt: fixedNow.Add(-time.Second)}
	source.mu.Unlock()

	limit, err := source.DailyAPILimit("tenant-a")
	require.NoError(t, err, "stale cache must be served when db unavailable")
	assert.Equal(t, int64(5000), limit)
}
