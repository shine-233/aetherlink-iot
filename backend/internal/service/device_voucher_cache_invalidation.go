// 文件用途：凭证轮换/删除后按 device_id 通知 broker 失效其 voucher→device 认证缓存。
// 核心逻辑：向固定 Redis Pub/Sub 通道发布结构化失效命令；broker 侧
//
//	mqtt-broker/plugin/aetherlink/voucher_cache_invalidation.go 订阅并按反向索引清键。
//
// 关键注意事项：通道常量与 broker 侧 VoucherCacheInvalidationChannel 保持一致，
//
//	任一侧变更必须双端同步并更新两侧契约测试。本通道是残窗收口机制——尤其覆盖
//	Phase 2b 后"旧明文不可得、无法直删 HMAC 键"的场景（hardening-plan 遗留#3）；
//	发布失败仅记录告警，不阻断主流程（残留映射仍受 broker 缓存 TTL ≤1h 兜底）。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/pkg/global"
)

// DeviceVoucherCacheInvalidationChannel 是 backend→broker 的凭证缓存失效命令通道。
// 契约对端：mqtt-broker/plugin/aetherlink/voucher_cache_invalidation.go。
const DeviceVoucherCacheInvalidationChannel = "aetherlink:device-voucher:cache-invalidate"

type deviceVoucherCacheInvalidationPayload struct {
	Version  int    `json:"version"`
	DeviceID string `json:"device_id"`
}

// publishDeviceVoucherCacheInvalidation 向 broker 广播按设备失效凭证缓存的命令。
func publishDeviceVoucherCacheInvalidation(ctx context.Context, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("publish device voucher cache invalidation: empty device id")
	}
	if global.REDIS == nil {
		return fmt.Errorf("redis is not initialized for device voucher cache invalidation")
	}
	payload, err := json.Marshal(deviceVoucherCacheInvalidationPayload{Version: 1, DeviceID: deviceID})
	if err != nil {
		return fmt.Errorf("encode device voucher cache invalidation payload: %w", err)
	}
	if _, err := global.REDIS.Publish(ctx, DeviceVoucherCacheInvalidationChannel, string(payload)).Result(); err != nil {
		return fmt.Errorf("publish device voucher cache invalidation: %w", err)
	}
	return nil
}

// notifyBrokerVoucherCacheInvalidation 发布失效命令并把失败降级为告警日志。
func notifyBrokerVoucherCacheInvalidation(ctx context.Context, deviceID string) {
	if err := publishDeviceVoucherCacheInvalidation(ctx, deviceID); err != nil {
		logrus.Warn("[Device][VoucherCache] notify broker invalidation failed; residual window falls back to broker cache TTL:", err)
	}
}
