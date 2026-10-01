package aetherlink

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"gopkg.in/redis.v5"
)

// 共享订阅骨架必须保留各调用方原有的错误文案，且 nil 时返回无类型 nil 接口。
func TestRedisChannelSubscribersKeepPurposeSpecificErrors(t *testing.T) {
	previous := redisCache
	redisCache = nil
	t.Cleanup(func() { redisCache = previous })

	sessionSub, err := subscribeRedisMQTTSessionRevocations()
	if err == nil || err.Error() != "redis is not initialized for mqtt session revocation" || sessionSub != nil {
		t.Fatalf("session subscribe = (%v, %v)", sessionSub, err)
	}
	voucherSub, err := subscribeRedisVoucherCacheInvalidations()
	if err == nil || err.Error() != "redis is not initialized for voucher cache invalidation" || voucherSub != nil {
		t.Fatalf("voucher subscribe = (%v, %v)", voucherSub, err)
	}
}

func TestRedisChannelSubscriptionCloseIsIdempotent(t *testing.T) {
	server := miniredis.RunT(t)
	previous := redisCache
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	redisCache = client
	t.Cleanup(func() {
		_ = client.Close()
		redisCache = previous
	})

	subscription, err := subscribeRedisChannel("aetherlink:test:shared", "shared test")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := client.Publish("aetherlink:test:shared", "payload-1").Err(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got := <-subscription.Messages(); got != "payload-1" {
		t.Fatalf("message = %q", got)
	}
	first := subscription.Close()
	if second := subscription.Close(); second != first {
		t.Fatalf("second Close() = %v, want %v", second, first)
	}
	if _, open := <-subscription.Messages(); open {
		t.Fatal("messages channel must be closed after Close")
	}
}

func TestSubscriptionLifecycleShutdownWithoutStartIsNoop(t *testing.T) {
	var monitor voucherCacheInvalidationMonitor
	if err := monitor.Close(); err != nil {
		t.Fatalf("Close() before Start = %v", err)
	}
	var nilMonitor *mqttSessionRevocationMonitor
	if err := nilMonitor.Close(); err != nil {
		t.Fatalf("nil Close() = %v", err)
	}
}
