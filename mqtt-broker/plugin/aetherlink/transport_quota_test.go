// 文件用途：TB-17R 传输日配额 broker 侧单测——计数、阈值判定与 fail-open 语义钉死。
// 核心逻辑：miniredis 驱动真实 Redis 语义覆盖 allowMQTTTransportDailyQuota 全分支：计数递增、
// 键格式、新建键 TTL、used==limit 放行、used>limit 拒绝（被拒同样计数）、限额缺失/非法/<=0 不执法、
// Redis 故障与未就绪放行、UTC 日界切换新键重新起计。
// 关键注意事项：限额/计数键字面量与 backend/internal/quota/transport_limit_publisher.go 为跨服务契约，
// 本文件钉死防漂移；时钟经 transportQuotaNow 注入，验证跨日用例的确定性。
// 重构建议：若计数点迁到 publish 路径（按消息计量），阈值用例随迁并保持断言口径不变。
package aetherlink

import (
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"gopkg.in/redis.v5"
)

// fixedQuotaNow 单测基准时刻（UTC 上午，距日界足够远，避免用例对秒级取整敏感）。
var fixedQuotaNow = time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)

// newTransportQuotaTestRedis 启动 miniredis 并临时替换包级 redisCache（与
// voucher_cache_invalidation_test.go 的既定替身模式一致）。
func newTransportQuotaTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	previous := redisCache
	redisCache = client
	t.Cleanup(func() { redisCache = previous })
	return server, client
}

// withTransportQuotaClock 注入固定时钟（生产路径 transportQuotaNow=time.Now）。
func withTransportQuotaClock(t *testing.T, now time.Time) {
	t.Helper()
	previous := transportQuotaNow
	transportQuotaNow = func() time.Time { return now }
	t.Cleanup(func() { transportQuotaNow = previous })
}

// seedTransportLimit 在 Redis 中写入租户限额缓存（模拟 backend 发布）。
func seedTransportLimit(t *testing.T, client *redis.Client, tenantID string, limit string) {
	t.Helper()
	if err := client.Set(transportLimitKey(tenantID), limit, time.Hour).Err(); err != nil {
		t.Fatalf("seed transport limit: %v", err)
	}
}

// TestTransportQuotaKeysPinCrossServiceContract 钉死跨服务键契约字面量：
// backend/internal/quota/transport_limit_publisher.go 的 TransportLimitKey 前缀必须与本侧一致。
func TestTransportQuotaKeysPinCrossServiceContract(t *testing.T) {
	const contractLimitPrefix = "aetherlink:quota:transport_limit:"
	const contractCountPrefix = "aetherlink:quota:transport_daily:"
	if transportLimitKeyPrefix != contractLimitPrefix {
		t.Fatalf("limit key prefix %q drifted from cross-service contract %q", transportLimitKeyPrefix, contractLimitPrefix)
	}
	if transportDailyCountKeyPrefix != contractCountPrefix {
		t.Fatalf("count key prefix %q drifted from contract %q", transportDailyCountKeyPrefix, contractCountPrefix)
	}
	if got := transportLimitKey("tenant-1"); got != contractLimitPrefix+"tenant-1" {
		t.Fatalf("transportLimitKey = %q, want %q", got, contractLimitPrefix+"tenant-1")
	}
	if got := transportDailyCountKey("2026-09-26", "tenant-1"); got != contractCountPrefix+"2026-09-26:tenant-1" {
		t.Fatalf("transportDailyCountKey = %q, want date-embedded key", got)
	}
}

// TestAllowTransportDailyQuotaCountsAndDeniesOverLimit 阈值语义（与 DecideDailyQuota 一致）：
// used<limit 放行、used==limit 放行（下一次才拒）、used>limit 拒绝；被拒连接同样计入当日用量。
func TestAllowTransportDailyQuotaCountsAndDeniesOverLimit(t *testing.T) {
	_, client := newTransportQuotaTestRedis(t)
	withTransportQuotaClock(t, fixedQuotaNow)
	seedTransportLimit(t, client, "tenant-a", "2")

	usedKey := transportDailyCountKey("2026-09-26", "tenant-a")

	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("first connect (used=1 <= limit=2) must be allowed")
	}
	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("second connect (used=2 == limit=2) must be allowed; next one is denied")
	}
	if allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("third connect (used=3 > limit=2) must be denied")
	}

	val, err := client.Get(usedKey).Result()
	if err != nil || val != "3" {
		t.Fatalf("denied connect must still be counted: key=%q val=%q err=%v", usedKey, val, err)
	}
	if !strings.Contains(usedKey, "2026-09-26") {
		t.Fatalf("count key must embed UTC date: %q", usedKey)
	}
}

// TestAllowTransportDailyQuotaTenantIsolation 租户隔离：不同租户各自起计，互不影响。
func TestAllowTransportDailyQuotaTenantIsolation(t *testing.T) {
	_, client := newTransportQuotaTestRedis(t)
	withTransportQuotaClock(t, fixedQuotaNow)
	seedTransportLimit(t, client, "tenant-a", "1")

	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("tenant-a first connect must be allowed")
	}
	if allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("tenant-a second connect must be denied")
	}
	// tenant-b 无限额缓存（backend 未发布）：不执法，不受 tenant-a 超限影响。
	if !allowMQTTTransportDailyQuota("tenant-b") {
		t.Fatal("tenant-b without published limit must be allowed (not enforced)")
	}
}

// TestAllowTransportDailyQuotaSetsTTLOnlyOnNewKey 仅新建键设 TTL：已存在的计数键不被续期。
func TestAllowTransportDailyQuotaSetsTTLOnlyOnNewKey(t *testing.T) {
	server, client := newTransportQuotaTestRedis(t)
	withTransportQuotaClock(t, fixedQuotaNow)
	seedTransportLimit(t, client, "tenant-a", "100")

	usedKey := transportDailyCountKey("2026-09-26", "tenant-a")
	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("first connect must be allowed")
	}

	ttl := server.TTL(usedKey)
	if ttl <= 0 {
		t.Fatalf("new count key must carry a TTL, got %v", ttl)
	}
	wantMax := time.Duration(secondsToNextUTCMidnight(fixedQuotaNow))*time.Second + transportCountKeyTTLBuffer
	if ttl > wantMax {
		t.Fatalf("TTL %v exceeds day end + buffer %v", ttl, wantMax)
	}

	// 人为缩短 TTL 后再连一次：非新建键不得被续期。
	server.SetTTL(usedKey, 90*time.Second)
	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("second connect must be allowed")
	}
	if got := server.TTL(usedKey); got != 90*time.Second {
		t.Fatalf("existing key TTL must not be refreshed, got %v want 90s", got)
	}
}

// TestAllowTransportDailyQuotaFailOpenOnRedisIssues fail-open：Redis 故障与未就绪一律放行。
func TestAllowTransportDailyQuotaFailOpenOnRedisIssues(t *testing.T) {
	server, _ := newTransportQuotaTestRedis(t)
	withTransportQuotaClock(t, fixedQuotaNow)

	// 场景一：Redis 客户端在但服务不可用（Get/Incr 均失败）→ 放行。
	server.Close()
	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("redis outage must fail open (allow)")
	}

	// 场景二：redisCache 为 nil（进程启动早期/测试环境）→ 放行。
	previous := redisCache
	redisCache = nil
	t.Cleanup(func() { redisCache = previous })
	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("nil redis client must fail open (allow)")
	}
}

// TestAllowTransportDailyQuotaLimitNotEnforced 限额缺失/非法/<=0 视为未配置执法阈值（不执法）；
// 但计量仍然发生（与 backend"先计量后执法"的 ObserveAndDecide 顺序一致）。
func TestAllowTransportDailyQuotaLimitNotEnforced(t *testing.T) {
	_, client := newTransportQuotaTestRedis(t)
	withTransportQuotaClock(t, fixedQuotaNow)

	cases := []struct {
		name  string
		limit string // 空串表示完全不写限额缓存
	}{
		{name: "missing-key", limit: ""},
		{name: "zero", limit: "0"},
		{name: "negative", limit: "-5"},
		{name: "not-a-number", limit: "abc"},
		{name: "empty-string", limit: "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个子用例独立租户，避免共享计数键造成跨用例累计。
			tenantID := "tenant-" + strings.ReplaceAll(tc.name, "-", "_")
			if tc.limit != "" {
				seedTransportLimit(t, client, tenantID, tc.limit)
			}
			if !allowMQTTTransportDailyQuota(tenantID) {
				t.Fatalf("limit %q must be treated as unconfigured (allow)", tc.limit)
			}
			val, err := client.Get(transportDailyCountKey("2026-09-26", tenantID)).Result()
			if err != nil || val != "1" {
				t.Fatalf("metering must happen even without enforcement: val=%q err=%v", val, err)
			}
		})
	}
}

// TestAllowTransportDailyQuotaUnattributableTenant 无法归因租户：放行且不触碰 Redis。
func TestAllowTransportDailyQuotaUnattributableTenant(t *testing.T) {
	server, client := newTransportQuotaTestRedis(t)
	withTransportQuotaClock(t, fixedQuotaNow)
	seedTransportLimit(t, client, "tenant-a", "0")

	for _, tenantID := range []string{"", "   "} {
		if !allowMQTTTransportDailyQuota(tenantID) {
			t.Fatalf("empty tenant %q must be allowed", tenantID)
		}
	}
	if keys := server.Keys(); len(keys) != 1 || keys[0] != transportLimitKey("tenant-a") {
		t.Fatalf("unattributable tenant must not touch redis, keys=%v", keys)
	}
}

// TestAllowTransportDailyQuotaUTCDateRoll 跨 UTC 日界：次日启用新计数键，配额重新起计。
func TestAllowTransportDailyQuotaUTCDateRoll(t *testing.T) {
	_, client := newTransportQuotaTestRedis(t)
	seedTransportLimit(t, client, "tenant-a", "1")

	withTransportQuotaClock(t, time.Date(2026, 9, 26, 23, 59, 58, 0, time.UTC))
	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("connect before midnight must be allowed")
	}
	if allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("same-day second connect must be denied (limit=1)")
	}

	withTransportQuotaClock(t, time.Date(2026, 9, 27, 0, 0, 2, 0, time.UTC))
	if !allowMQTTTransportDailyQuota("tenant-a") {
		t.Fatal("connect on next UTC day must start a fresh quota and be allowed")
	}

	for date, want := range map[string]string{
		"2026-09-26": "2", // 当天两次连接：1 次放行 + 1 次被拒（被拒同样计入当日用量）
		"2026-09-27": "1", // 次日新键重新起计
	} {
		val, err := client.Get(transportDailyCountKey(date, "tenant-a")).Result()
		if err != nil || val != want {
			t.Fatalf("per-day counter for %s: val=%q want=%q err=%v", date, val, want, err)
		}
	}
}

// TestSecondsToNextUTCMidnightParity 与 backend/internal/quota/decision.go 的
// SecondsToNextUTCMidnight 语义逐点对齐（整天/一天末尾向上取整/起始后一秒）。
func TestSecondsToNextUTCMidnightParity(t *testing.T) {
	cases := []struct {
		at   time.Time
		want int64
	}{
		{time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), 86400},
		{time.Date(2026, 9, 26, 23, 59, 59, 0, time.UTC), 1},
		{time.Date(2026, 9, 26, 23, 59, 59, int(500*time.Millisecond), time.UTC), 1},
		{time.Date(2026, 9, 26, 0, 0, 1, 0, time.UTC), 86399},
		{time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC), 12 * 3600},
	}
	for _, tc := range cases {
		if got := secondsToNextUTCMidnight(tc.at); got != tc.want {
			t.Fatalf("secondsToNextUTCMidnight(%s) = %d, want %d", tc.at, got, tc.want)
		}
	}

	// 非 UTC 输入必须先归一到 UTC 再切日。
	shanghai := time.Date(2026, 9, 27, 7, 0, 0, 0, time.FixedZone("CST", 8*3600)) // UTC 2026-09-26 23:00
	if got := secondsToNextUTCMidnight(shanghai); got != 3600 {
		t.Fatalf("non-UTC input must normalize to UTC: got %d want 3600", got)
	}
}
