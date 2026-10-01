package uplink

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestWSSubscriberCheckIsCachedPerDevice(t *testing.T) {
	f := NewTelemetryUplink(TelemetryUplinkConfig{Logger: quietLogger()})
	t.Cleanup(f.Stop)
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	f.wsSubs.now = clock.now

	calls := map[string]int{}
	watched := map[string]bool{"watched": true}
	f.wsSubExists = func(_ context.Context, id string) (bool, error) {
		calls[id]++
		return watched[id], nil
	}

	ctx := context.Background()
	for i := 0; i < 50; i++ {
		for _, id := range []string{"watched", "idle"} {
			got, err := f.hasWSSubscriber(ctx, id)
			if err != nil || got != watched[id] {
				t.Fatalf("hasWSSubscriber(%s) = %v, %v", id, got, err)
			}
		}
	}
	if calls["watched"] != 1 || calls["idle"] != 1 {
		t.Fatalf("EXISTS calls inside TTL = %v, want one per device", calls)
	}

	// A new subscriber becomes visible once the negative entry expires.
	watched["idle"] = true
	clock.advance(wsSubCacheTTL)
	if got, _ := f.hasWSSubscriber(ctx, "idle"); !got || calls["idle"] != 2 {
		t.Fatalf("expired entry not re-checked: got=%v calls=%d", got, calls["idle"])
	}

	if removed := f.wsSubs.sweep(); removed != 1 || f.wsSubs.len() != 1 {
		t.Fatalf("sweep removed %d (len %d), want 1 expired entry gone", removed, f.wsSubs.len())
	}
}

func TestWSSubscriberErrorsAreNotCached(t *testing.T) {
	f := NewTelemetryUplink(TelemetryUplinkConfig{Logger: quietLogger()})
	t.Cleanup(f.Stop)
	calls := 0
	f.wsSubExists = func(context.Context, string) (bool, error) {
		calls++
		if calls == 1 {
			return false, errors.New("redis blip")
		}
		return true, nil
	}
	if _, err := f.hasWSSubscriber(context.Background(), "d"); err == nil {
		t.Fatal("expected error on first check")
	}
	if got, err := f.hasWSSubscriber(context.Background(), "d"); err != nil || !got {
		t.Fatalf("second check = %v, %v; error must not have been cached", got, err)
	}
}

func TestStatusOnlineDeliveriesRunOnceInOrderAfterDelay(t *testing.T) {
	f := NewStatusUplink(StatusUplinkConfig{Logger: quietLogger()})
	var mu sync.Mutex
	var order []string
	done := make(chan struct{})
	start := time.Now()
	f.scheduleOnlineDeliveries(&model.Device{ID: "d1"}, 30*time.Millisecond, func(d *model.Device) {
		mu.Lock()
		order = append(order, "expected:"+d.ID, "shadow:"+d.ID)
		mu.Unlock()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("online deliveries never ran")
	}
	if time.Since(start) < 30*time.Millisecond {
		t.Fatal("deliveries ran before the delay")
	}
	if len(order) != 2 || order[0] != "expected:d1" || order[1] != "shadow:d1" {
		t.Fatalf("order = %v", order)
	}
}

func TestStatusOnlineDeliveriesSkippedAfterStop(t *testing.T) {
	f := NewStatusUplink(StatusUplinkConfig{Logger: quietLogger()})
	ran := make(chan struct{}, 1)
	f.scheduleOnlineDeliveries(&model.Device{ID: "d1"}, 20*time.Millisecond, func(*model.Device) { ran <- struct{}{} })
	f.Stop()
	select {
	case <-ran:
		t.Fatal("deliveries ran after Stop")
	case <-time.After(100 * time.Millisecond):
	}
}
