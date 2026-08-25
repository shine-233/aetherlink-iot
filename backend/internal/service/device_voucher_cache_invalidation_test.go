// 文件用途：验证按设备失效 broker 凭证缓存的发布契约（channel、payload、降级行为）。
// 核心逻辑：miniredis 订阅真实通道断言消息形状；nil-REDIS 与空 device_id 走拒绝路径。
// 关键注意事项：channel 字面量必须与 mqtt-broker/plugin/aetherlink/voucher_cache_invalidation.go
//   的 VoucherCacheInvalidationChannel 一致。

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"aetherlink-iot/backend/pkg/global"
)

func TestDeviceVoucherCacheInvalidationChannelContract(t *testing.T) {
	const contractChannel = "aetherlink:device-voucher:cache-invalidate"
	if DeviceVoucherCacheInvalidationChannel != contractChannel {
		t.Fatalf("channel %q drifted from cross-service contract %q", DeviceVoucherCacheInvalidationChannel, contractChannel)
	}
}

func TestPublishDeviceVoucherCacheInvalidationDeliversContractPayload(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pubsub := client.Subscribe(ctx, DeviceVoucherCacheInvalidationChannel)
	t.Cleanup(func() { _ = pubsub.Close() })
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatalf("confirm subscription: %v", err)
	}

	oldRedis := global.REDIS
	global.REDIS = client
	t.Cleanup(func() { global.REDIS = oldRedis })

	if err := publishDeviceVoucherCacheInvalidation(ctx, "device-1"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	message, err := pubsub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("receive message: %v", err)
	}
	if message.Channel != DeviceVoucherCacheInvalidationChannel {
		t.Fatalf("message channel = %q, want %q", message.Channel, DeviceVoucherCacheInvalidationChannel)
	}
	var payload deviceVoucherCacheInvalidationPayload
	if err := json.Unmarshal([]byte(message.Payload), &payload); err != nil {
		t.Fatalf("decode payload %q: %v", message.Payload, err)
	}
	if payload.Version != 1 || payload.DeviceID != "device-1" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestPublishDeviceVoucherCacheInvalidationRejectsInvalidInput(t *testing.T) {
	oldRedis := global.REDIS
	global.REDIS = nil
	t.Cleanup(func() { global.REDIS = oldRedis })

	if err := publishDeviceVoucherCacheInvalidation(context.Background(), ""); err == nil {
		t.Fatal("empty device id should be rejected")
	}
	if err := publishDeviceVoucherCacheInvalidation(context.Background(), "device-1"); err == nil {
		t.Fatal("nil REDIS should be rejected")
	}
}
