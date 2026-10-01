package aetherlink

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"gopkg.in/redis.v5"
)

func installHotpathTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	s := miniredis.RunT(t)
	prev := redisCache
	redisCache = redis.NewClient(&redis.Options{Addr: s.Addr()})
	now := time.Unix(10_000, 0)
	hotpathCacheNow = func() time.Time { return now }
	t.Cleanup(func() {
		_ = redisCache.Close()
		redisCache = prev
		hotpathCacheNow = time.Now
	})
	return s
}

func TestDeviceDebugConfigCachedIncludingNegative(t *testing.T) {
	s := installHotpathTestRedis(t)
	devDebugNow = func() time.Time { return time.Unix(10_000, 0) }
	t.Cleanup(func() { devDebugNow = time.Now })

	if _, enabled, err := GetDeviceDebugConfig("dev-neg"); err != nil || enabled {
		t.Fatalf("missing cfg: enabled=%v err=%v", enabled, err)
	}
	// Negative result is cached: a config appearing within the TTL is not seen yet...
	if err := SetRedisForJsondata(devDebugCfgKey("dev-neg"), DeviceDebugConfig{Enabled: true, MaxItems: 5}, 0); err != nil {
		t.Fatal(err)
	}
	if _, enabled, _ := GetDeviceDebugConfig("dev-neg"); enabled {
		t.Fatal("negative entry should be served from cache within TTL")
	}
	// ...but becomes visible once the negative entry's TTL expires.
	hotpathCacheNow = func() time.Time { return time.Unix(10_000, 0).Add(devDebugCfgCacheTTL + time.Millisecond) }
	cfg, enabled, err := GetDeviceDebugConfig("dev-neg")
	if err != nil || !enabled || cfg.MaxItems != 5 {
		t.Fatalf("after negative TTL expiry: cfg=%+v enabled=%v err=%v", cfg, enabled, err)
	}

	// Positive hit survives Redis deletion until TTL expiry, then refreshes.
	s.Del(devDebugCfgKey("dev-neg"))
	if _, enabled, _ := GetDeviceDebugConfig("dev-neg"); !enabled {
		t.Fatal("positive entry should be cached within TTL")
	}
	hotpathCacheNow = func() time.Time { return time.Unix(10_000, 0).Add(2*devDebugCfgCacheTTL + 2*time.Millisecond) }
	if _, enabled, _ := GetDeviceDebugConfig("dev-neg"); enabled {
		t.Fatal("expired entry must be re-read from Redis")
	}
}

func TestDeviceDebugConfigExpireAtEvaluatedOnEveryCall(t *testing.T) {
	installHotpathTestRedis(t)
	now := int64(20_000)
	devDebugNow = func() time.Time { return time.Unix(now, 0) }
	t.Cleanup(func() { devDebugNow = time.Now })
	if err := SetRedisForJsondata(devDebugCfgKey("dev-exp"), DeviceDebugConfig{Enabled: true, ExpireAt: now + 1}, 0); err != nil {
		t.Fatal(err)
	}
	if _, enabled, _ := GetDeviceDebugConfig("dev-exp"); !enabled {
		t.Fatal("want enabled before expire_at")
	}
	now += 2
	if _, enabled, _ := GetDeviceDebugConfig("dev-exp"); enabled {
		t.Fatal("cached config must still honor expire_at")
	}
}

func TestDeviceDebugConfigCacheIgnoresEntriesFromOtherClient(t *testing.T) {
	installHotpathTestRedis(t)
	if _, enabled, _ := GetDeviceDebugConfig("dev-cli"); enabled {
		t.Fatal("unexpected enabled")
	}
	// Swap to a fresh Redis that has the config: stale entry of the old client is ignored.
	s2 := miniredis.RunT(t)
	old := redisCache
	redisCache = redis.NewClient(&redis.Options{Addr: s2.Addr()})
	t.Cleanup(func() { _ = redisCache.Close(); redisCache = old })
	if err := SetRedisForJsondata(devDebugCfgKey("dev-cli"), DeviceDebugConfig{Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	if _, enabled, _ := GetDeviceDebugConfig("dev-cli"); !enabled {
		t.Fatal("entry cached for a different redis client must not be served")
	}
}

func TestCompiledTopicMappingsCachedUntilTTL(t *testing.T) {
	installHotpathTestRedis(t)
	rows := cachedTopicMappings{Loaded: true, Rows: []DeviceTopicMapping{
		{SourceTopic: "bad/#", TargetTopic: "x"},
		{SourceTopic: "raw/{device_number}/up", TargetTopic: "devices/telemetry"},
	}}
	if err := SetRedisForJsondata(cacheKeyUp("cfg-1"), rows, 0); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	svc := NewTopicMapService()
	target, ok := svc.ResolveUpTarget(ctx, "cfg-1", "raw/d1/up")
	if !ok || target != "devices/telemetry" {
		t.Fatalf("resolve = %q %v", target, ok)
	}
	// Change Redis behind the cache: still served locally within TTL.
	rows.Rows[1].TargetTopic = "devices/changed"
	if err := SetRedisForJsondata(cacheKeyUp("cfg-1"), rows, 0); err != nil {
		t.Fatal(err)
	}
	if target, _ := svc.ResolveUpTarget(ctx, "cfg-1", "raw/d1/up"); target != "devices/telemetry" {
		t.Fatalf("want cached target, got %q", target)
	}
	// TTL expiry refreshes from Redis.
	hotpathCacheNow = func() time.Time { return time.Unix(10_000, 0).Add(topicMapLocalCacheTTL + time.Millisecond) }
	if target, _ := svc.ResolveUpTarget(ctx, "cfg-1", "raw/d1/up"); target != "devices/changed" {
		t.Fatalf("want refreshed target, got %q", target)
	}
}

func TestTopicMapLocalCacheLRUBound(t *testing.T) {
	c := newTopicMapLocalCache(2)
	for _, id := range []string{"a", "b", "c"} {
		c.put(nil, topicMapLocalKey{deviceConfigID: id, direction: DirectionUp}, nil)
	}
	if c.len() != 2 {
		t.Fatalf("len = %d, want 2", c.len())
	}
	if _, ok := c.get(nil, topicMapLocalKey{deviceConfigID: "a", direction: DirectionUp}); ok {
		t.Fatal("oldest entry should be evicted")
	}
}

func TestResolveDownSourceParsesPayloadOnceAndMatchesLegacy(t *testing.T) {
	id := "set"
	mappings := []DeviceTopicMapping{
		{SourceTopic: "d/{device_number}/other", TargetTopic: "devices/command/+/+", DataIdentifier: strPtr("nomatch")},
		{SourceTopic: "d/{device_number}/set", TargetTopic: "devices/command/+/+", DataIdentifier: &id},
	}
	src, out, ok := resolveDownSourceFromMappings(mappings, "devices/command/n1/x", "n1", []byte(`{"method":"set","params":{"a":1}}`))
	if !ok || src != "d/n1/set" || string(out) != `{"a":1}` {
		t.Fatalf("got %q %s %v", src, out, ok)
	}
	if _, _, ok := resolveDownSourceFromMappings(mappings, "devices/command/n1/x", "n1", []byte(`not json`)); ok {
		t.Fatal("unparseable payload must not match data-identifier mappings")
	}
}

func strPtr(s string) *string { return &s }

func (c *topicMapLocalCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
