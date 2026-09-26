// 文件用途：TB-17R 传输维度限额发布——把租户套餐的传输日限额写入 Redis，供 MQTT broker 在
// 连接认证路径读取执法（见 mqtt-broker/plugin/aetherlink/transport_quota.go）。
// 核心逻辑：PublishTransportLimit 以 aetherlink:quota:transport_limit:{tenant} 为键写入限额十进制
// 字符串（TTL 7 天）；limit<=0 时删除键——broker 缺键即按"未配置执法阈值"放行，与
// backend/internal/quota/decision.go 的 DecideDailyQuota（limit<=0 不执法）口径一致。
// 关键注意事项：键名/取值格式/TTL 为跨服务契约（broker 侧同名常量 transportLimitKeyPrefix 双向注释，
// 任一侧变更需双端同步并更新契约测试）；发布失败只返回错误由调用方告警，不得阻断套餐变更主流程
// （broker 侧 fail-open 兜底）。Redis 实例与 broker 共享（deploy 侧同库配置）。
// 重构建议：若引入套餐周期性重发布（cron 自愈），复用本函数接入调度器即可，键契约不变。
package quota

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/global"
)

// TransportLimitTTL 传输限额缓存 TTL。套餐变更会覆盖写；TTL 仅兜底"绕过 backend 直改库导致键陈旧"
// 与租户弃用后的键残留——改小（分钟级）会让"长期不变更套餐的租户"在 TTL 到期后静默失去执法
// （broker 缺键 fail-open），故取 7 天而非与 backend 进程内限额缓存同量级。
const TransportLimitTTL = 7 * 24 * time.Hour

// TransportLimitKeyPrefix 传输限额缓存键前缀。跨服务契约：mqtt-broker/plugin/aetherlink/
// transport_quota.go 的 transportLimitKeyPrefix 必须与本常量一致。
const TransportLimitKeyPrefix = "aetherlink:quota:transport_limit:"

// TransportLimitKey 返回租户传输限额缓存键（broker 按本键只读，双端必须一致）。
func TransportLimitKey(tenantID string) string {
	return TransportLimitKeyPrefix + tenantID
}

// transportLimitStore 限额缓存写入抽象（单测注入假实现，避免依赖真实 Redis）。
type transportLimitStore interface {
	// SetRaw 以固定 TTL 写入字符串值。
	SetRaw(key, value string, ttl time.Duration) error
	// DelRaw 删除键（键不存在不算错误）。
	DelRaw(key string) error
}

// redisTransportLimitStore 生产适配：写 global.REDIS（与 MQTT broker 共享的 Redis 实例）。
type redisTransportLimitStore struct{}

// errRedisNotInitialized Redis 未初始化（降级/单测环境）时的统一错误：调用方 fail-open 告警。
var errRedisNotInitialized = errors.New("quota: redis not initialized for transport limit publish")

func (redisTransportLimitStore) SetRaw(key, value string, ttl time.Duration) error {
	if global.REDIS == nil {
		return errRedisNotInitialized
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCallTimeout)
	defer cancel()
	return global.REDIS.Set(ctx, key, value, ttl).Err()
}

func (redisTransportLimitStore) DelRaw(key string) error {
	if global.REDIS == nil {
		return errRedisNotInitialized
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCallTimeout)
	defer cancel()
	return global.REDIS.Del(ctx, key).Err()
}

// PublishTransportLimit 把租户传输日限额写入 Redis（billing 套餐变更成功后调用）。
// limit<=0 表示套餐未配置传输执法阈值：删除缓存键，让 broker 按未配置阈值处理（不执法）。
// 返回错误仅表示 Redis 不可用或入参非法——调用方应告警而非阻断套餐变更。
func PublishTransportLimit(tenantID string, limit int64) error {
	return PublishTransportLimitWith(redisTransportLimitStore{}, tenantID, limit)
}

// PublishTransportLimitWith 指定存储的发布入口（单测注入假实现）。
func PublishTransportLimitWith(store transportLimitStore, tenantID string, limit int64) error {
	if store == nil {
		return errors.New("quota: nil transport limit store")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return fmt.Errorf("quota: empty tenant for transport limit publish")
	}
	key := TransportLimitKey(tenantID)
	if limit <= 0 {
		// 未配置执法阈值：清掉旧键，broker 侧随键缺失回到不执法（不会用陈旧限额误伤）。
		return store.DelRaw(key)
	}
	return store.SetRaw(key, strconv.FormatInt(limit, 10), TransportLimitTTL)
}
