// 文件用途：覆盖凭证缓存按设备失效的解析、索引/清除与 Pub/Sub 端到端行为。
// 核心逻辑：miniredis 驱动真实 Redis 语义，锁定 channel/payload 跨服务契约与幂等语义。
// 关键注意事项：channel 字面量必须与 backend/internal/service/device_voucher_cache_invalidation.go 一致。

package aetherlink

import (
	"encoding/json"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"gopkg.in/redis.v5"
)

func TestVoucherCacheInvalidationChannelContract(t *testing.T) {
	const contractChannel = "aetherlink:device-voucher:cache-invalidate"
	if VoucherCacheInvalidationChannel != contractChannel {
		t.Fatalf("channel %q drifted from cross-service contract %q", VoucherCacheInvalidationChannel, contractChannel)
	}
}

func TestParseVoucherCacheInvalidationMessage(t *testing.T) {
	valid, ok := parseVoucherCacheInvalidationMessage(`{"version":1,"device_id":"device-1"}`)
	if !ok || valid.Version != 1 || valid.DeviceID != "device-1" {
		t.Fatalf("valid payload parsed incorrectly: %+v ok=%v", valid, ok)
	}
	for name, payload := range map[string]string{
		"empty":         "",
		"not-json":      "device-1",
		"missing-id":    `{"version":1}`,
		"wrong-version": `{"version":2,"device_id":"device-1"}`,
		"blank-id":      `{"version":1,"device_id":"   "}`,
		"broken-json":   `{"version":1,"device_id":`,
	} {
		if _, ok := parseVoucherCacheInvalidationMessage(payload); ok {
			t.Fatalf("%s payload should be rejected", name)
		}
	}
}

func newVoucherCacheTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	previous := redisCache
	redisCache = client
	t.Cleanup(func() { redisCache = previous })
	return server, client
}

func TestIndexAndEvictVoucherCacheForDevice(t *testing.T) {
	_, client := newVoucherCacheTestRedis(t)

	keyA := voucherCacheKey("voucher-a")
	keyB := voucherCacheKey("voucher-b")
	if err := SetStr(keyA, "device-1", defaultCacheTTL); err != nil {
		t.Fatalf("seed cache A: %v", err)
	}
	if err := indexVoucherCacheKeyForDevice("device-1", keyA, defaultCacheTTL); err != nil {
		t.Fatalf("index cache A: %v", err)
	}
	if err := SetStr(keyB, "device-1", defaultCacheTTL); err != nil {
		t.Fatalf("seed cache B: %v", err)
	}
	if err := indexVoucherCacheKeyForDevice("device-1", keyB, defaultCacheTTL); err != nil {
		t.Fatalf("index cache B: %v", err)
	}

	evicted, err := evictVoucherCacheForDevice("device-1")
	if err != nil {
		t.Fatalf("evict: %v", err)
	}
	if evicted != 2 {
		t.Fatalf("evicted = %d, want 2", evicted)
	}
	for _, key := range []string{keyA, keyB, voucherCacheIndexKey("device-1")} {
		exists, err := client.Exists(key).Result()
		if err != nil || exists {
			t.Fatalf("key %q should be deleted (exists=%v err=%v)", key, exists, err)
		}
	}

	// 幂等：再次失效返回 0。
	if evicted, err := evictVoucherCacheForDevice("device-1"); err != nil || evicted != 0 {
		t.Fatalf("second evict = (%d,%v), want (0,nil)", evicted, err)
	}
}

func TestVoucherCacheInvalidationMonitorEvictsPublishedDevice(t *testing.T) {
	_, client := newVoucherCacheTestRedis(t)

	targetKey := voucherCacheKey("voucher-live")
	if err := SetStr(targetKey, "device-live", defaultCacheTTL); err != nil {
		t.Fatalf("seed live mapping: %v", err)
	}
	if err := indexVoucherCacheKeyForDevice("device-live", targetKey, defaultCacheTTL); err != nil {
		t.Fatalf("index live mapping: %v", err)
	}

	monitor := newVoucherCacheInvalidationMonitor(subscribeRedisVoucherCacheInvalidations)
	if err := monitor.Start(); err != nil {
		t.Fatalf("start monitor: %v", err)
	}
	t.Cleanup(func() { _ = monitor.Close() })

	payload, _ := json.Marshal(voucherCacheInvalidationMessage{Version: 1, DeviceID: "device-live"})
	if err := client.Publish(VoucherCacheInvalidationChannel, string(payload)).Err(); err != nil {
		t.Fatalf("publish invalidation: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		exists, err := client.Exists(targetKey).Result()
		if err == nil && !exists {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("monitor did not evict the published device mapping in time")
}

func TestIndexVoucherCacheKeyForDeviceToleratesMissingRedis(t *testing.T) {
	previous := redisCache
	redisCache = nil
	t.Cleanup(func() { redisCache = previous })
	if err := indexVoucherCacheKeyForDevice("device-1", "key", defaultCacheTTL); err != nil {
		t.Fatalf("indexing without redis should be a no-op, got %v", err)
	}
}
