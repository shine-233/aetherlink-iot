// telemetry_ws_subs.go caches the "does anyone watch this device" check.
//
// The WS gateway marks watched devices with the Redis key ws:sub:<id>. Before
// this cache every telemetry message paid one EXISTS round-trip, even for the
// vast majority of devices nobody has open. Results (positive and negative) are
// now kept for wsSubCacheTTL per device; Redis errors are never cached.
//
// Trade-off: a freshly opened device page may miss up to wsSubCacheTTL of
// telemetry, and a closed page may receive up to wsSubCacheTTL of extra
// publishes (the gateway drops those). The Redis channel and event format are
// unchanged.
package uplink

import (
	"errors"
	"sync"
	"time"
)

var errRedisNotInitialized = errors.New("redis client is not initialized")

const (
	wsSubCacheTTL        = time.Second
	wsSubCacheSweepEvery = 10 * time.Second
)

type wsSubEntry struct {
	exists  bool
	expires int64 // unix nanos
}

type wsSubCache struct {
	mu      sync.Mutex
	entries map[string]wsSubEntry
	ttl     time.Duration
	now     func() time.Time
}

func newWSSubCache(ttl time.Duration) *wsSubCache {
	return &wsSubCache{entries: make(map[string]wsSubEntry), ttl: ttl, now: time.Now}
}

// lookup returns (exists, true) for a live entry.
func (c *wsSubCache) lookup(deviceID string) (bool, bool) {
	if c == nil {
		return false, false
	}
	now := c.now().UnixNano()
	c.mu.Lock()
	e, ok := c.entries[deviceID]
	c.mu.Unlock()
	if !ok || now >= e.expires {
		return false, false
	}
	return e.exists, true
}

func (c *wsSubCache) store(deviceID string, exists bool) {
	if c == nil || c.ttl <= 0 {
		return
	}
	expires := c.now().Add(c.ttl).UnixNano()
	c.mu.Lock()
	c.entries[deviceID] = wsSubEntry{exists: exists, expires: expires}
	c.mu.Unlock()
}

// sweep removes expired entries; returns how many were removed.
func (c *wsSubCache) sweep() int {
	if c == nil {
		return 0
	}
	now := c.now().UnixNano()
	removed := 0
	c.mu.Lock()
	for k, e := range c.entries {
		if now >= e.expires {
			delete(c.entries, k)
			removed++
		}
	}
	c.mu.Unlock()
	return removed
}

func (c *wsSubCache) len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
