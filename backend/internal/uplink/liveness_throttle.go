// liveness_throttle.go coalesces heartbeat-key refreshes for chatty devices.
//
// Every business uplink used to issue a Redis SET device:<id>:heartbeat (or
// :timeout) with TTL = the configured interval. A device reporting at 10 Hz paid
// 10 SETs per second for a key whose expiry is measured in tens of seconds.
//
// The throttle skips a refresh when the same key (same device, same TTL) was
// refreshed less than `window` ago. Contract: the key still expires no earlier
// than TTL - window after the device's last message, and window is at most
// TTL/10 (capped at 2s). A config change (different TTL) or a failed refresh
// always forces the next SET. Auto-online is never throttled.
package uplink

import (
	"sync"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/internal/service"
)

const (
	heartbeatRefreshWindowMax = 2 * time.Second
	// heartbeatRefreshWindowDivisor keeps the window to a tenth of the TTL so
	// a device sending right at the interval boundary is never cut short by
	// more than 10%.
	heartbeatRefreshWindowDivisor = 10
	heartbeatThrottleSweepEvery   = time.Minute
)

type heartbeatStamp struct {
	at  int64 // unix nanos of the last successful refresh
	ttl int   // seconds; identifies which key/TTL was written
}

type heartbeatThrottle struct {
	entries   sync.Map // deviceID -> heartbeatStamp
	now       func() time.Time
	lastSweep atomic.Int64
}

func newHeartbeatThrottle() *heartbeatThrottle {
	return &heartbeatThrottle{now: time.Now}
}

// heartbeatTTLSeconds mirrors HeartbeatService.RefreshHeartbeat's priority
// (heartbeat > online_timeout) and returns the TTL it will write.
func heartbeatTTLSeconds(config *service.HeartbeatConfig) int {
	if config == nil {
		return 0
	}
	if config.Heartbeat > 0 {
		return config.Heartbeat
	}
	if config.OnlineTimeout > 0 {
		return config.OnlineTimeout
	}
	return 0
}

// heartbeatRefreshWindow is min(TTL/10, 2s); 0 disables throttling.
func heartbeatRefreshWindow(ttlSeconds int) time.Duration {
	if ttlSeconds <= 0 {
		return 0
	}
	w := time.Duration(ttlSeconds) * time.Second / heartbeatRefreshWindowDivisor
	if w > heartbeatRefreshWindowMax {
		w = heartbeatRefreshWindowMax
	}
	return w
}

// fresh reports whether deviceID's key with this TTL was refreshed within the window.
func (t *heartbeatThrottle) fresh(deviceID string, ttlSeconds int) bool {
	window := heartbeatRefreshWindow(ttlSeconds)
	if t == nil || window <= 0 {
		return false
	}
	v, ok := t.entries.Load(deviceID)
	if !ok {
		return false
	}
	stamp := v.(heartbeatStamp)
	return stamp.ttl == ttlSeconds && t.now().UnixNano()-stamp.at < int64(window)
}

func (t *heartbeatThrottle) record(deviceID string, ttlSeconds int) {
	if t == nil {
		return
	}
	now := t.now().UnixNano()
	t.entries.Store(deviceID, heartbeatStamp{at: now, ttl: ttlSeconds})
	t.maybeSweep(now)
}

func (t *heartbeatThrottle) forget(deviceID string) {
	if t != nil {
		t.entries.Delete(deviceID)
	}
}

// maybeSweep drops stale stamps about once a minute so deleted or silent
// devices do not pin memory. Only one caller wins the CAS and sweeps.
func (t *heartbeatThrottle) maybeSweep(now int64) {
	last := t.lastSweep.Load()
	if now-last < int64(heartbeatThrottleSweepEvery) || !t.lastSweep.CompareAndSwap(last, now) {
		return
	}
	cutoff := now - int64(2*heartbeatRefreshWindowMax)
	t.entries.Range(func(k, v any) bool {
		if v.(heartbeatStamp).at < cutoff {
			t.entries.Delete(k)
		}
		return true
	})
}
