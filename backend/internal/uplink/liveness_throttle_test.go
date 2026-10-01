package uplink

import (
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

type countingHeartbeat struct {
	config     *service.HeartbeatConfig
	refreshErr error
	refreshed  int
}

func (h *countingHeartbeat) GetConfig(*model.Device) (*service.HeartbeatConfig, error) {
	return h.config, nil
}

func (h *countingHeartbeat) RefreshHeartbeat(*model.Device, *service.HeartbeatConfig) error {
	h.refreshed++
	return h.refreshErr
}

func newThrottledLiveness(hb heartbeatRefresher) (*deviceLiveness, *fakeClock) {
	l := newDeviceLiveness(hb, quietLogger())
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l.throttle.now = clock.now
	l.setOnline = func(string) (bool, error) { return false, nil }
	return l, clock
}

// TestHeartbeatThrottleRefreshesOncePerWindow: a 60s heartbeat gives a 2s
// window; 100 messages 10ms apart (1s total) cause exactly one SET, and the
// next message after the window refreshes again.
func TestHeartbeatThrottleRefreshesOncePerWindow(t *testing.T) {
	hb := &countingHeartbeat{config: &service.HeartbeatConfig{Heartbeat: 60}}
	l, clock := newThrottledLiveness(hb)
	device := &model.Device{ID: "d1", IsOnline: 1}

	for i := 0; i < 100; i++ {
		l.touch(device)
		clock.advance(10 * time.Millisecond)
	}
	if hb.refreshed != 1 {
		t.Fatalf("refreshes inside one window = %d, want 1", hb.refreshed)
	}
	clock.advance(heartbeatRefreshWindowMax)
	l.touch(device)
	if hb.refreshed != 2 {
		t.Fatalf("refreshes after window = %d, want 2", hb.refreshed)
	}
	// Other devices are throttled independently.
	l.touch(&model.Device{ID: "d2", IsOnline: 1})
	if hb.refreshed != 3 {
		t.Fatalf("second device refreshes = %d, want 3", hb.refreshed)
	}
}

func TestHeartbeatThrottleWindowScalesWithTTL(t *testing.T) {
	cases := map[int]time.Duration{0: 0, -1: 0, 5: 500 * time.Millisecond, 10: time.Second, 20: 2 * time.Second, 600: 2 * time.Second}
	for ttl, want := range cases {
		if got := heartbeatRefreshWindow(ttl); got != want {
			t.Fatalf("window(%d) = %v, want %v", ttl, got, want)
		}
	}
	if heartbeatTTLSeconds(&service.HeartbeatConfig{Heartbeat: 30, OnlineTimeout: 90}) != 30 ||
		heartbeatTTLSeconds(&service.HeartbeatConfig{OnlineTimeout: 90}) != 90 ||
		heartbeatTTLSeconds(&service.HeartbeatConfig{}) != 0 || heartbeatTTLSeconds(nil) != 0 {
		t.Fatal("TTL priority must mirror RefreshHeartbeat (heartbeat > online_timeout)")
	}
}

// TestHeartbeatThrottleForcesRefresh covers the cases where skipping would
// break the TTL contract: TTL change, failed SET, offline device, no TTL.
func TestHeartbeatThrottleForcesRefresh(t *testing.T) {
	t.Run("ttl change", func(t *testing.T) {
		hb := &countingHeartbeat{config: &service.HeartbeatConfig{Heartbeat: 60}}
		l, _ := newThrottledLiveness(hb)
		d := &model.Device{ID: "d1", IsOnline: 1}
		l.touch(d)
		hb.config = &service.HeartbeatConfig{Heartbeat: 30}
		l.touch(d)
		if hb.refreshed != 2 {
			t.Fatalf("refreshes = %d, want 2", hb.refreshed)
		}
	})
	t.Run("failed refresh retries", func(t *testing.T) {
		hb := &countingHeartbeat{config: &service.HeartbeatConfig{Heartbeat: 60}, refreshErr: errors.New("redis down")}
		l, _ := newThrottledLiveness(hb)
		d := &model.Device{ID: "d1", IsOnline: 1}
		l.touch(d)
		l.touch(d)
		if hb.refreshed != 2 {
			t.Fatalf("refreshes = %d, want 2", hb.refreshed)
		}
	})
	t.Run("offline device always refreshes", func(t *testing.T) {
		hb := &countingHeartbeat{config: &service.HeartbeatConfig{Heartbeat: 60}}
		l, _ := newThrottledLiveness(hb)
		l.touch(&model.Device{ID: "d1", IsOnline: 1})
		l.touch(&model.Device{ID: "d1", IsOnline: 0})
		if hb.refreshed != 2 {
			t.Fatalf("refreshes = %d, want 2", hb.refreshed)
		}
	})
	t.Run("nil throttle disables", func(t *testing.T) {
		hb := &countingHeartbeat{config: &service.HeartbeatConfig{Heartbeat: 60}}
		l, _ := newThrottledLiveness(hb)
		l.throttle = nil
		d := &model.Device{ID: "d1", IsOnline: 1}
		l.touch(d)
		l.touch(d)
		if hb.refreshed != 2 {
			t.Fatalf("refreshes = %d, want 2", hb.refreshed)
		}
	})
}

func TestHeartbeatThrottleSweepDropsStaleEntries(t *testing.T) {
	th := newHeartbeatThrottle()
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	th.now = clock.now
	th.lastSweep.Store(clock.t.UnixNano())
	th.record("old", 60)
	clock.advance(heartbeatThrottleSweepEvery + time.Second)
	th.record("new", 60) // triggers the sweep
	if _, ok := th.entries.Load("old"); ok {
		t.Fatal("stale entry survived the sweep")
	}
	if _, ok := th.entries.Load("new"); !ok {
		t.Fatal("fresh entry was swept")
	}
}
