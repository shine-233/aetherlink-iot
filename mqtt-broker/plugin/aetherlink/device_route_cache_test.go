// 文件用途：验证设备路由微缓存的容量、时效与失效语义。
// 核心逻辑：纯缓存层单测——拷贝隔离、TTL 过期、容量满载放弃写入、invalidate 即时生效。

package aetherlink

import (
	"testing"
	"time"
)

func routeCacheTestDevice(id string) *Device {
	name := "device-" + id
	return &Device{ID: id, Name: &name, TenantID: "tenant-1", IsEnabled: "enabled", ActivateFlag: "active"}
}

func TestDeviceRouteCacheRoundTripAndCopyIsolation(t *testing.T) {
	cache := newDeviceRouteCache(time.Minute, 8)
	original := routeCacheTestDevice("d1")
	cache.set("d1", original)

	got, ok := cache.get("d1")
	if !ok || got.ID != "d1" {
		t.Fatalf("expected cached device, got ok=%v", ok)
	}
	got.IsEnabled = "disabled"
	again, _ := cache.get("d1")
	if again.IsEnabled != "enabled" {
		t.Fatal("mutating the returned copy must not affect the cached entry")
	}
}

func TestDeviceRouteCacheExpiry(t *testing.T) {
	cache := newDeviceRouteCache(15*time.Millisecond, 8)
	cache.set("d2", routeCacheTestDevice("d2"))
	if _, ok := cache.get("d2"); !ok {
		t.Fatal("fresh entry should hit")
	}
	time.Sleep(25 * time.Millisecond)
	if _, ok := cache.get("d2"); ok {
		t.Fatal("expired entry must not be served")
	}
}

func TestDeviceRouteCacheCapacityFallsBackToMiss(t *testing.T) {
	cache := newDeviceRouteCache(time.Minute, 2)
	cache.set("a", routeCacheTestDevice("a"))
	cache.set("b", routeCacheTestDevice("b"))
	cache.set("c", routeCacheTestDevice("c")) // 满载：放弃写入 c，不驱逐 a/b

	if _, ok := cache.get("c"); ok {
		t.Fatal("overflow entry should not be cached")
	}
	for _, id := range []string{"a", "b"} {
		if _, ok := cache.get(id); !ok {
			t.Fatalf("existing entry %q must survive overflow attempt", id)
		}
	}
	// 同 key 覆盖不受容量限制影响（刷新属于既有条目）。
	cache.set("a", routeCacheTestDevice("a"))
	if _, ok := cache.get("a"); !ok {
		t.Fatal("refreshing an existing entry should always succeed")
	}
}

func TestDeviceRouteCacheInvalidate(t *testing.T) {
	cache := newDeviceRouteCache(time.Minute, 8)
	cache.set("x", routeCacheTestDevice("x"))
	cache.invalidate("x")
	if _, ok := cache.get("x"); ok {
		t.Fatal("invalidated entry must miss immediately")
	}
	cache.invalidate("") // 空 id 是安全 no-op
}

func TestNewDeviceRouteCacheConfigFallbacks(t *testing.T) {
	cache := newDeviceRouteCache(0, -1)
	if cache.ttl != defaultDeviceRouteCacheTTL || cache.max != defaultDeviceRouteCacheMaxEntries {
		t.Fatalf("invalid config should fall back to defaults, got ttl=%v max=%d", cache.ttl, cache.max)
	}
}
