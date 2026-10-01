// 文件用途：传输维度配额展示数据源（ReadTransportDailyUsage / ReadTransportDailyLimit）单元测试。
// 核心逻辑：miniredis 驱动真实 Redis 语义——计数键读回、缺键 0、限额解析与TrimSpace、
// 非法值/<=0 归一 unlimited、REDIS 未初始化报错、键契约字面量钉死。
// 关键注意事项：计数键前缀 aetherlink:quota:transport_daily: 与限额键前缀
// aetherlink:quota:transport_limit: 均为跨服务契约（broker 侧 transport_quota.go），本文件钉死防漂移。
// 重构建议：若增加 DB 回落路径，补回落优先级用例（Redis 权威、DB 兜底）。
package quota

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aetherlink-iot/backend/pkg/global"
)

// newTransportReaderTestRedis 启动 miniredis 并临时替换 global.REDIS。
func newTransportReaderTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	oldRedis := global.REDIS
	global.REDIS = rdb
	t.Cleanup(func() { global.REDIS = oldRedis })
	return mr, rdb
}

// TestTransportDailyCountKeyPinsCrossServiceContract 钉死计数键契约：
// mqtt-broker/plugin/aetherlink/transport_quota.go 的 transportDailyCountKeyPrefix 必须与本侧一致。
func TestTransportDailyCountKeyPinsCrossServiceContract(t *testing.T) {
	const contractPrefix = "aetherlink:quota:transport_daily:"
	assert.Equal(t, contractPrefix, TransportDailyCountKeyPrefix)
	assert.Equal(t, contractPrefix+"2026-09-26:tenant-1", TransportDailyCountKey("2026-09-26", "tenant-1"))
}

// TestReadTransportDailyUsageReadsBrokerCounter broker 已 INCR 的计数键可被读回，日期为 UTC 当日。
func TestReadTransportDailyUsageReadsBrokerCounter(t *testing.T) {
	_, rdb := newTransportReaderTestRedis(t)
	ctx := context.Background()

	date := UsageDate(time.Now())
	require.NoError(t, rdb.Set(ctx, TransportDailyCountKey(date, "tenant-1"), "42", time.Hour).Err())

	used, gotDate, err := ReadTransportDailyUsage(ctx, "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, int64(42), used)
	assert.Equal(t, date, gotDate)
}

// TestReadTransportDailyUsageMissingKeyIsZero 当日无连接（缺键）是正常态：计数 0 且无错误。
func TestReadTransportDailyUsageMissingKeyIsZero(t *testing.T) {
	_, _ = newTransportReaderTestRedis(t)
	ctx := context.Background()

	used, gotDate, err := ReadTransportDailyUsage(ctx, "tenant-fresh")
	require.NoError(t, err)
	assert.Equal(t, int64(0), used)
	assert.Equal(t, UsageDate(time.Now()), gotDate)
}

// TestReadTransportDailyUsageFailures 读取失败必须上抛（调用方 fail-open），空租户拒绝。
func TestReadTransportDailyUsageFailures(t *testing.T) {
	mr, _ := newTransportReaderTestRedis(t)
	ctx := context.Background()

	// 空租户 fail-closed 于入参侧。
	_, _, err := ReadTransportDailyUsage(ctx, "  ")
	require.Error(t, err)

	// REDIS 未初始化：报错而非 panic。
	oldRedis := global.REDIS
	global.REDIS = nil
	t.Cleanup(func() { global.REDIS = oldRedis })
	_, _, err = ReadTransportDailyUsage(ctx, "tenant-1")
	require.Error(t, err)

	// Redis 恢复但宕机：上抛连接错误。
	global.REDIS = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = global.REDIS.Close() })
	mr.Close()
	_, _, err = ReadTransportDailyUsage(ctx, "tenant-1")
	require.Error(t, err)
}

// TestReadTransportDailyLimitFromPublishedCache backend 发布的限额缓存可被读回（含空白容差）。
func TestReadTransportDailyLimitFromPublishedCache(t *testing.T) {
	_, rdb := newTransportReaderTestRedis(t)
	ctx := context.Background()

	require.NoError(t, rdb.Set(ctx, TransportLimitKey("tenant-1"), "500000", time.Hour).Err())
	limit, err := ReadTransportDailyLimit(ctx, "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, int64(500000), limit)

	require.NoError(t, rdb.Set(ctx, TransportLimitKey("tenant-2"), " 10000 ", time.Hour).Err())
	limit, err = ReadTransportDailyLimit(ctx, "tenant-2")
	require.NoError(t, err)
	assert.Equal(t, int64(10000), limit, "surrounding whitespace must be tolerated (broker parity)")
}

// TestReadTransportDailyLimitUnconfiguredIsZero 缺键/非法值/<=0 与 broker 侧同口径归一为 0（不执法）。
func TestReadTransportDailyLimitUnconfiguredIsZero(t *testing.T) {
	_, rdb := newTransportReaderTestRedis(t)
	ctx := context.Background()

	for name, raw := range map[string]string{
		"missing-key":  "",    // 不写键
		"zero":         "0",   // 发布了 0：未配置阈值
		"negative":     "-5",  // 发布了负数：同上
		"not-a-number": "abc", // 值损坏：按未配置处理而非错误（broker 同口径）
	} {
		t.Run(name, func(t *testing.T) {
			if raw != "" {
				require.NoError(t, rdb.Set(ctx, TransportLimitKey("tenant-"+name), raw, time.Hour).Err())
			}
			limit, err := ReadTransportDailyLimit(ctx, "tenant-"+name)
			require.NoError(t, err)
			assert.Equal(t, int64(0), limit)
		})
	}

	// REDIS 未初始化：上抛错误（调用方按 unlimited 展示，与 broker fail-open 实况一致）。
	oldRedis := global.REDIS
	global.REDIS = nil
	t.Cleanup(func() { global.REDIS = oldRedis })
	_, err := ReadTransportDailyLimit(ctx, "tenant-1")
	require.Error(t, err)
}
