// 文件用途：TB-17R 传输维度配额展示数据源——读取 MQTT broker 维护的传输日计数与限额缓存。
// 核心逻辑：ReadTransportDailyUsage 读 broker 原子 INCR 的当日计数键
// （aetherlink:quota:transport_daily:{UTC日期}:{tenant}）；ReadTransportDailyLimit 读 backend 在
// 套餐变更时发布的限额缓存（aetherlink:quota:transport_limit:{tenant}，见 transport_limit_publisher.go）。
// 关键注意事项：两个键都是跨服务契约——broker 侧 mqtt-broker/plugin/aetherlink/transport_quota.go
// 同名常量双向注释，任一侧变更需双端同步并更新契约测试；缺键/非法值归一为 0（与 broker 侧
// transportDailyLimit 的"缺失即不执法"口径一致），连接类错误上抛由调用方 fail-open（展示 unlimited）。
// 重构建议：若后续把传输计量落库（对齐 api_usage_daily 模式），在本文件加 DB 回落即可，键契约不变。
package quota

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/redis/go-redis/v9"
)

// TransportDailyCountKeyPrefix 传输日计数键前缀。跨服务契约：mqtt-broker/plugin/aetherlink/
// transport_quota.go 的 transportDailyCountKeyPrefix 必须与本常量一致（broker 写，backend 只读展示）。
const TransportDailyCountKeyPrefix = "aetherlink:quota:transport_daily:"

// TransportDailyCountKey 返回租户当日传输计数键（日期内嵌 UTC，与 broker 侧 transportDailyCountKey 同构）。
func TransportDailyCountKey(date, tenantID string) string {
	return TransportDailyCountKeyPrefix + date + ":" + tenantID
}

// ReadTransportDailyUsage 读取租户当日传输计数（broker 每次 MQTT 连接认证原子 INCR 的权威值）。
// 返回计数与计量日（UTC，YYYY-MM-DD）。键缺失（当日无连接）返回 0；Redis 不可用时返回错误，
// 调用方必须 fail-open（按 0 展示），不得因读取失败阻断查询端点。
func ReadTransportDailyUsage(ctx context.Context, tenantID string) (int64, string, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, "", fmt.Errorf("quota: empty tenant for transport usage read")
	}
	if global.REDIS == nil {
		return 0, "", errRedisNotInitialized
	}
	date := UsageDate(time.Now())
	ctx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	val, err := global.REDIS.Get(ctx, TransportDailyCountKey(date, tenantID)).Result()
	if err == redis.Nil {
		return 0, date, nil // 当日尚无连接：计数为 0（缺键是正常态，不是错误）
	}
	if err != nil {
		return 0, date, err
	}
	used, parseErr := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if parseErr != nil {
		return 0, date, fmt.Errorf("quota: transport daily count %q not an integer: %w", val, parseErr)
	}
	return used, date, nil
}

// ReadTransportDailyLimit 读取 broker 实际执法使用的传输限额缓存值。
// 返回 0 表示"未配置执法阈值"（键缺失/未发布/非法值/<=0——与 broker transportDailyLimit 的
// 不执法口径一致），调用方按 unlimited 展示；Redis 不可用时返回错误（同样落到 unlimited 展示，
// 因为 broker 在 Redis 故障时同样 fail-open 放行——展示与执法实况同口径）。
func ReadTransportDailyLimit(ctx context.Context, tenantID string) (int64, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, fmt.Errorf("quota: empty tenant for transport limit read")
	}
	if global.REDIS == nil {
		return 0, errRedisNotInitialized
	}
	ctx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	val, err := global.REDIS.Get(ctx, TransportLimitKey(tenantID)).Result()
	if err == redis.Nil {
		return 0, nil // backend 未发布（如旧版 backend / 键过期）：broker 不执法，展示 unlimited
	}
	if err != nil {
		return 0, err
	}
	limit, parseErr := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if parseErr != nil || limit <= 0 {
		// 值非法或 <=0：与 broker 侧同口径视为未配置阈值，而非错误。
		return 0, nil
	}
	return limit, nil
}
