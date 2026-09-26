// 文件用途：TB-17R 传输维度租户日配额——MQTT CONNECT 认证路径上的轻量执法（Redis 计数+套餐限额缓存）。
// 核心逻辑：设备认证通过后按租户原子 INCR 当日传输计数键（aetherlink:quota:transport_daily:{date}:{tenant}），
// 与 backend 在套餐变更时写入的限额缓存（aetherlink:quota:transport_limit:{tenant}）比较；
// used > limit 拒绝本次 CONNECT，used == limit 放行（下一次才拒），与 backend/internal/quota/decision.go
// 的 DecideDailyQuota 判定语义一致（被拒连接同样计入当日用量）。
// 关键注意事项：fail-open 是硬约束——Redis 未就绪/故障、限额缓存缺失或 <=0、无法归因租户一律放行；
// 限额缓存键为跨服务契约（backend/internal/quota/transport_limit_publisher.go 双向注释，任一侧变更需同步）；
// 计数键按 UTC 日期内嵌、仅新建键设 TTL（至次日零点+缓冲）。计数粒度为连接级（非逐消息）；
// CoAP/TCP 网关同构接线按批次范围排除。Redis 调用沿用 db.go 的包级客户端与超时口径。
// 重构建议：若需按消息计量，把计数点迁到 publish 钩子并复用本文件的键契约与判定函数。
package aetherlink

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// transportLimitKeyPrefix 租户传输限额缓存键前缀。跨服务契约：backend/internal/quota/
	// transport_limit_publisher.go 的 TransportLimitKeyPrefix 必须与本常量一致（套餐变更时写入，
	// 本插件在认证路径只读）。
	transportLimitKeyPrefix = "aetherlink:quota:transport_limit:"
	// transportDailyCountKeyPrefix 租户传输日计数键前缀；键内嵌 UTC 日期，次日自然启用新键。
	transportDailyCountKeyPrefix = "aetherlink:quota:transport_daily:"
	// transportCountKeyTTLBuffer 计数键在次日零点后多保留的缓冲时长（与 backend meterKeyTTLBuffer 同口径）。
	transportCountKeyTTLBuffer = 5 * time.Minute
)

// transportUsageDateLayout 计量日格式（UTC 日期，YYYY-MM-DD），与 backend/internal/quota 的
// usageDateLayout 保持一致，保证双端日期切分口径相同。
const transportUsageDateLayout = "2006-01-02"

// transportQuotaNow 时钟取值点（单测注入推进 UTC 日界；生产恒为 time.Now）。
var transportQuotaNow = time.Now

// errMQTTTransportQuotaExceeded 租户传输日配额超限；由 GMQTT 翻译成认证失败的 CONNACK。
var errMQTTTransportQuotaExceeded = errors.New("mqtt transport daily quota exceeded")

// transportQuotaWarnMu 保护下方 fail-open 告警节流状态（认证热路径，避免 Redis 故障时日志风暴）。
var (
	transportQuotaWarnMu      sync.Mutex
	transportQuotaLastWarnAt  time.Time
	transportQuotaFailOpenCnt uint64
)

// transportLimitKey 租户传输限额缓存键（与 backend TransportLimitKey 同构，跨服务契约键）。
func transportLimitKey(tenantID string) string {
	return transportLimitKeyPrefix + tenantID
}

// transportDailyCountKey 当日传输计数键：日期内嵌，跨日自然切换新键。
func transportDailyCountKey(date, tenantID string) string {
	return transportDailyCountKeyPrefix + date + ":" + tenantID
}

// transportUsageDate 返回时刻 t 对应的计量日（UTC 日期字符串），与 backend quota.UsageDate 同口径。
func transportUsageDate(t time.Time) string {
	return t.UTC().Format(transportUsageDateLayout)
}

// secondsToNextUTCMidnight 返回距下一个 UTC 零点的秒数（向上取整，最小 1）。
// 与 backend/internal/quota/decision.go 的 SecondsToNextUTCMidnight 语义一致（跨服务同口径，勿单侧漂移）。
func secondsToNextUTCMidnight(t time.Time) int64 {
	u := t.UTC()
	next := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	secs := int64(math.Ceil(next.Sub(u).Seconds()))
	if secs < 1 {
		secs = 1
	}
	return secs
}

// transportCountTTL 计数键 TTL：至次日 UTC 零点 + 缓冲（给在途判定留读取窗口）。
func transportCountTTL(now time.Time) time.Duration {
	return time.Duration(secondsToNextUTCMidnight(now))*time.Second + transportCountKeyTTLBuffer
}

// allowMQTTTransportDailyQuota 认证钩子入口：按租户判定本次 CONNECT 是否放行（TB-17R）。
// 返回 false 仅当计数与限额均可用且 used > limit；其余一切情形放行（fail-open）。
func allowMQTTTransportDailyQuota(tenantID string) bool {
	if redisCache == nil {
		// Redis 未就绪（进程启动早期或测试环境）：fail-open，不阻断认证。
		return true
	}
	return observeTransportDailyQuota(strings.TrimSpace(tenantID), transportQuotaNow())
}

// observeTransportDailyQuota 执行一次"计量→读限额→判定"：先原子累加当日计数（被拒连接同样计入，
// 计费口径与 backend quota.Service.ObserveAndDecide 相同），再读限额缓存做阈值判定。
func observeTransportDailyQuota(tenantID string, now time.Time) bool {
	if tenantID == "" {
		// 无法归因租户的连接不执法（也不计量），避免聚成一个 key 互相误伤。
		return true
	}

	used, err := incrTransportDailyCount(tenantID, now)
	if err != nil {
		// 计数不可用：fail-open（执法依赖计数，计数不可用即无依据）。
		warnTransportQuotaFailOpen(tenantID, err)
		return true
	}

	limit, enforced := transportDailyLimit(tenantID)
	if !enforced {
		// 限额缓存缺失或 <=0：与 DecideDailyQuota 的"未配置执法阈值"口径一致，放行。
		return true
	}
	return used <= limit
}

// incrTransportDailyCount 原子累加当日传输计数并返回累加后的值（含本次）；仅对新建键设置 TTL。
func incrTransportDailyCount(tenantID string, now time.Time) (int64, error) {
	key := transportDailyCountKey(transportUsageDate(now), tenantID)
	used, err := redisCache.Incr(key).Result()
	if err != nil {
		return 0, err
	}
	if used == 1 {
		// 新计数键：设至次日零点+缓冲的 TTL；失败只影响 Redis 键回收，不影响本次判定。
		if expireErr := redisCache.Expire(key, transportCountTTL(now)).Err(); expireErr != nil && Log != nil {
			Log.Warn("set transport daily count ttl failed",
				zap.String("key", key),
				zap.Error(expireErr),
			)
		}
	}
	return used, nil
}

// transportDailyLimit 读取租户限额缓存。第二个返回值为 false 表示不执法：
// 键缺失（backend 未发布/已过期）、值不可解析或 limit<=0 均视为"未配置执法阈值"，
// 与 backend/internal/quota/limit.go 的容错口径一致——无限额不封人。
func transportDailyLimit(tenantID string) (int64, bool) {
	raw, err := redisCache.Get(transportLimitKey(tenantID)).Result()
	if err != nil {
		// redis.Nil（键不存在）与其余读错误同口径处理：读不到限额即不执法（fail-open）。
		return 0, false
	}
	limit, parseErr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if parseErr != nil || limit <= 0 {
		return 0, false
	}
	return limit, true
}

// warnTransportQuotaFailOpen 记录 fail-open（每分钟至多一条，防止 Redis 故障期间认证路径日志风暴）。
func warnTransportQuotaFailOpen(tenantID string, err error) {
	transportQuotaWarnMu.Lock()
	defer transportQuotaWarnMu.Unlock()
	transportQuotaFailOpenCnt++
	if Log == nil {
		return
	}
	now := time.Now()
	if now.Sub(transportQuotaLastWarnAt) < time.Minute {
		return
	}
	transportQuotaLastWarnAt = now
	Log.Warn("mqtt transport quota fail-open",
		zap.String("tenant_id", tenantID),
		zap.Uint64("count", transportQuotaFailOpenCnt),
		zap.Error(err),
	)
}
