package mqttdebug

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newIndexedTestManager(tb testing.TB, uplink UplinkSource, tweak func(*Config)) *Manager {
	tb.Helper()
	config := Config{
		Broker:              "tcp://broker.invalid:1883",
		SessionTTL:          time.Minute,
		OpenCooldown:        time.Nanosecond,
		MaxSessions:         5000,
		MaxSessionsPerUser:  5000,
		MaxInboundPerSecond: 1 << 30,
		UplinkSource:        uplink,
		TransportFactory: func(config TransportConfig) (Transport, error) {
			return &fakeTransport{hooks: config.Hooks, handlers: map[string]func(IncomingMessage){}}, nil
		},
	}
	config.MaxInboundBytesPerSecond = 1 << 30
	if tweak != nil {
		tweak(&config)
	}
	manager := NewManager(config, nil)
	tb.Cleanup(manager.Stop)
	return manager
}

func testScope(user, device int) Scope {
	return Scope{
		TenantID:     "tenant-1",
		UserID:       fmt.Sprintf("user-%d", user),
		DeviceID:     fmt.Sprintf("device-%d", device),
		DeviceNumber: fmt.Sprintf("D-%d", device),
	}
}

func mustOpen(tb testing.TB, manager *Manager, scope Scope) Snapshot {
	tb.Helper()
	snapshot, err := manager.Open(context.Background(), scope)
	if err != nil {
		tb.Fatalf("open %+v: %v", scope, err)
	}
	return snapshot
}

func mustApply(tb testing.TB, manager *Manager, scope Scope, sessionID string, command Command) Snapshot {
	tb.Helper()
	snapshot, err := manager.Apply(context.Background(), scope, sessionID, command)
	if err != nil {
		tb.Fatalf("apply %+v: %v", command, err)
	}
	return snapshot
}

func inboundCount(messages []Message) int {
	count := 0
	for _, message := range messages {
		if message.Direction == "inbound" {
			count++
		}
	}
	return count
}

func (manager *Manager) indexSizesForTest() (byID, byScope, byDevice, users int) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return len(manager.index.byID), len(manager.index.byScope), len(manager.index.byDevice), len(manager.index.userCount)
}

func TestTrustedUplinkFanoutTargetsOnlyWatchingSessionsOfThatDevice(t *testing.T) {
	uplink := &fakeUplinkSource{}
	manager := newIndexedTestManager(t, uplink, nil)
	ctx := context.Background()

	// 1000 sessions on other devices, all subscribed to the same shared filter.
	others := make([]Snapshot, 0, 1000)
	otherScopes := make([]Scope, 0, 1000)
	for i := 0; i < 1000; i++ {
		scope := testScope(i, 10_000+i)
		opened := mustOpen(t, manager, scope)
		mustApply(t, manager, scope, opened.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry"})
		others = append(others, opened)
		otherScopes = append(otherScopes, scope)
	}
	watcherScope := testScope(1, 1)
	watcher := mustOpen(t, manager, watcherScope)
	mustApply(t, manager, watcherScope, watcher.SessionID, Command{Action: ActionSubscribe, Topic: "devices/event/+"})
	mustApply(t, manager, watcherScope, watcher.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry"})
	// Same device, different user, broker-only subscription: must not be targeted.
	idleScope := testScope(2, 1)
	idle := mustOpen(t, manager, idleScope)
	mustApply(t, manager, idleScope, idle.SessionID, Command{Action: ActionSubscribe, Topic: "devices/status/device-1"})
	// Same device id under another tenant: identity includes the tenant.
	foreignScope := testScope(3, 1)
	foreignScope.TenantID = "tenant-2"
	foreign := mustOpen(t, manager, foreignScope)
	mustApply(t, manager, foreignScope, foreign.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry"})

	incoming := TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-1", Topic: "devices/telemetry", Payload: []byte(`{"t":1}`)}
	targets := manager.trustedUplinkTargets(incoming, nil)
	if len(targets) != 1 || targets[0].id != watcher.SessionID {
		t.Fatalf("targets=%d, want only the watcher session", len(targets))
	}
	if miss := manager.trustedUplinkTargets(TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-1", Topic: "gateway/telemetry"}, nil); len(miss) != 0 {
		t.Fatalf("non-matching topic produced %d targets", len(miss))
	}

	uplink.emit(incoming)

	got, err := manager.Snapshot(ctx, watcherScope, watcher.SessionID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if inboundCount(got.Messages) != 1 {
		t.Fatalf("watcher inbound=%d, want 1", inboundCount(got.Messages))
	}
	for _, check := range []struct {
		scope Scope
		id    string
	}{{idleScope, idle.SessionID}, {foreignScope, foreign.SessionID}, {otherScopes[0], others[0].SessionID}, {otherScopes[999], others[999].SessionID}} {
		snapshot, err := manager.Snapshot(ctx, check.scope, check.id, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if inboundCount(snapshot.Messages) != 0 {
			t.Fatalf("session %s received a foreign uplink", check.id)
		}
	}
}

func TestCompiledTopicFilterMatchesGenericMatcher(t *testing.T) {
	filters := []string{"devices/telemetry", "devices/event/+", "+/+", "+", "a/+/c", "a//c", "a/+", ""}
	topics := []string{"devices/telemetry", "devices/event/x", "devices/event/", "devices/event", "devices/event/x/y",
		"a/b/c", "a//c", "a/", "a", "", "/", "x/y", "devices/telemetry/extra"}
	for _, filter := range filters {
		compiled := compileTopicFilter(filter)
		for _, topic := range topics {
			if got, want := compiled.matches(topic), mqttTopicFilterMatches(filter, topic); got != want {
				t.Errorf("filter %q topic %q: compiled=%v generic=%v", filter, topic, got, want)
			}
		}
	}
}

func TestTrustedUplinkPayloadIsBoundedOnce(t *testing.T) {
	uplink := &fakeUplinkSource{}
	manager := newIndexedTestManager(t, uplink, func(config *Config) { config.PayloadMaxBytes = 4 })
	scopes := []Scope{testScope(1, 1), testScope(2, 1)}
	ids := make([]string, len(scopes))
	for i, scope := range scopes {
		ids[i] = mustOpen(t, manager, scope).SessionID
		mustApply(t, manager, scope, ids[i], Command{Action: ActionSubscribe, Topic: "devices/telemetry"})
	}
	payload := []byte("abcdefgh")
	uplink.emit(TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-1", Topic: "devices/telemetry", Payload: payload})
	payload[0] = 'Z' // the observer may reuse its buffer after the handler returns
	for i, scope := range scopes {
		snapshot, err := manager.Snapshot(context.Background(), scope, ids[i], 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		last := snapshot.Messages[len(snapshot.Messages)-1]
		if last.Payload != "abcd" || !last.Truncated || last.Source != SubscriptionModeAcceptedUplink {
			t.Fatalf("session %d got %+v", i, last)
		}
	}
}

func TestUnsubscribeRemovesTrustedFilterFromFanout(t *testing.T) {
	uplink := &fakeUplinkSource{}
	manager := newIndexedTestManager(t, uplink, nil)
	scope := testScope(1, 1)
	opened := mustOpen(t, manager, scope)
	mustApply(t, manager, scope, opened.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry"})
	mustApply(t, manager, scope, opened.SessionID, Command{Action: ActionSubscribe, Topic: "devices/status/device-1", QoS: 1})
	snapshot := mustApply(t, manager, scope, opened.SessionID, Command{Action: ActionUnsubscribe, Topic: "devices/telemetry"})

	if len(snapshot.Subscriptions) != 1 || snapshot.Subscriptions[0] != "devices/status/device-1" {
		t.Fatalf("subscriptions=%v", snapshot.Subscriptions)
	}
	if detail := snapshot.SubscriptionDetails[0]; detail.Mode != SubscriptionModeBroker || detail.QoS == nil || *detail.QoS != 1 {
		t.Fatalf("detail=%+v", detail)
	}
	*snapshot.SubscriptionDetails[0].QoS = 0 // caller mutation must not leak into the shared view
	snapshot.Subscriptions[0] = "mutated"
	again, err := manager.Snapshot(context.Background(), scope, opened.SessionID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if again.Subscriptions[0] != "devices/status/device-1" || *again.SubscriptionDetails[0].QoS != 1 {
		t.Fatalf("snapshot aliases the subscription view: %+v", again.SubscriptionDetails)
	}

	if targets := manager.trustedUplinkTargets(TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-1", Topic: "devices/telemetry"}, nil); len(targets) != 0 {
		t.Fatalf("unsubscribed session still targeted")
	}
}

func TestSnapshotSubscriptionsAreSortedAndNeverNull(t *testing.T) {
	manager := newIndexedTestManager(t, &fakeUplinkSource{}, nil)
	scope := testScope(1, 1)
	opened := mustOpen(t, manager, scope)
	if opened.Subscriptions == nil || opened.SubscriptionDetails == nil {
		t.Fatal("empty subscription lists must be non-nil (JSON [])")
	}
	for _, topic := range []string{"gateway/telemetry", "devices/telemetry", "devices/event/+"} {
		mustApply(t, manager, scope, opened.SessionID, Command{Action: ActionSubscribe, Topic: topic})
	}
	snapshot, err := manager.Snapshot(context.Background(), scope, opened.SessionID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"devices/event/+", "devices/telemetry", "gateway/telemetry"}
	for i, topic := range want {
		if snapshot.Subscriptions[i] != topic || snapshot.SubscriptionDetails[i].Topic != topic || snapshot.SubscriptionDetails[i].QoS != nil {
			t.Fatalf("subscriptions=%v details=%+v", snapshot.Subscriptions, snapshot.SubscriptionDetails)
		}
	}
}

func TestReopenReplacesSessionAndIndexStaysConsistent(t *testing.T) {
	manager := newIndexedTestManager(t, &fakeUplinkSource{}, nil)
	ctx := context.Background()
	scope := testScope(1, 1)
	first := mustOpen(t, manager, scope)
	manager.mu.RLock()
	firstItem := manager.index.get(first.SessionID)
	manager.mu.RUnlock()

	time.Sleep(time.Millisecond)
	second := mustOpen(t, manager, scope)
	if _, err := manager.Snapshot(ctx, scope, first.SessionID, 0, 0); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("replaced session snapshot err=%v", err)
	}
	if byID, byScope, byDevice, users := manager.indexSizesForTest(); byID != 1 || byScope != 1 || byDevice != 1 || users != 1 {
		t.Fatalf("index sizes id=%d scope=%d device=%d users=%d", byID, byScope, byDevice, users)
	}

	// A stale expiry/failure path for the replaced session must not evict the new one.
	manager.removeAndClose(firstItem)
	if _, err := manager.Snapshot(ctx, scope, second.SessionID, 0, 0); err != nil {
		t.Fatalf("stale removeAndClose evicted the replacement: %v", err)
	}

	if err := manager.Close(ctx, scope, second.SessionID); err != nil {
		t.Fatal(err)
	}
	if byID, byScope, byDevice, users := manager.indexSizesForTest(); byID+byScope+byDevice+users != 0 {
		t.Fatalf("index not empty after close: %d %d %d %d", byID, byScope, byDevice, users)
	}
}

func TestPerUserLimitUsesIndexedCount(t *testing.T) {
	manager := newIndexedTestManager(t, &fakeUplinkSource{}, func(config *Config) { config.MaxSessionsPerUser = 2 })
	ctx := context.Background()
	a := mustOpen(t, manager, testScope(1, 1))
	mustOpen(t, manager, testScope(1, 2))
	if _, err := manager.Open(ctx, testScope(1, 3)); !errors.Is(err, ErrSessionCapacity) {
		t.Fatalf("third session for user err=%v, want ErrSessionCapacity", err)
	}
	mustOpen(t, manager, testScope(2, 3)) // other users are unaffected
	if err := manager.Close(ctx, testScope(1, 1), a.SessionID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond) // the rejected attempt still armed the per-scope cooldown; Windows clock ticks are coarse
	mustOpen(t, manager, testScope(1, 3))
}

func TestStopClosesEverySessionAndRejectsFurtherUse(t *testing.T) {
	manager := newIndexedTestManager(t, &fakeUplinkSource{}, nil)
	scope := testScope(1, 1)
	opened := mustOpen(t, manager, scope)
	manager.mu.RLock()
	item := manager.index.get(opened.SessionID)
	manager.mu.RUnlock()
	manager.Stop()
	item.mu.Lock()
	closed := item.closed
	item.mu.Unlock()
	if !closed {
		t.Fatal("Stop must close live sessions")
	}
	if _, err := manager.Open(context.Background(), testScope(2, 2)); !errors.Is(err, ErrRuntimeClosed) {
		t.Fatalf("open after stop err=%v", err)
	}
	manager.handleTrustedUplink(TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-1", Topic: "devices/telemetry"})
}

// Without cgo (-race unavailable on this host) this still exercises lock
// ordering under contention: a deadlock or index corruption fails the test.
func TestConcurrentFanoutAndLifecycle(t *testing.T) {
	uplink := &fakeUplinkSource{}
	manager := newIndexedTestManager(t, uplink, func(config *Config) { config.MaxCommandsPerSecond = 1 << 20 })
	ctx := context.Background()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					uplink.emit(TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-1", Topic: "devices/telemetry", Payload: []byte("x")})
				}
			}
		}()
	}
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(user int) {
			defer wg.Done()
			scope := testScope(user, 1)
			for round := 0; round < 50; round++ {
				opened, err := manager.Open(ctx, scope)
				if err != nil {
					if errors.Is(err, ErrRateLimited) {
						continue
					}
					t.Errorf("open: %v", err)
					return
				}
				_, _ = manager.Apply(ctx, scope, opened.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry"})
				_, _ = manager.Snapshot(ctx, scope, opened.SessionID, 0, 0)
				_ = manager.Close(ctx, scope, opened.SessionID)
			}
		}(worker)
	}
	done := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		close(stop)
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("deadlock: concurrent fan-out and lifecycle did not finish")
	}
	if byID, byScope, byDevice, users := manager.indexSizesForTest(); byID+byScope+byDevice+users != 0 {
		t.Fatalf("index leaked entries: %d %d %d %d", byID, byScope, byDevice, users)
	}
}

// BenchmarkHandleTrustedUplink measures the ingestion-path cost with 1000 idle
// debug sessions on other devices. legacy_scan reproduces the old O(sessions)
// scan + per-session lock + map iteration for comparison.
func BenchmarkHandleTrustedUplink(b *testing.B) {
	uplink := &fakeUplinkSource{}
	manager := newIndexedTestManager(b, uplink, nil)
	for i := 0; i < 1000; i++ {
		scope := testScope(i, 10_000+i)
		opened := mustOpen(b, manager, scope)
		mustApply(b, manager, scope, opened.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry"})
	}
	watcherScope := testScope(1, 1)
	watcher := mustOpen(b, manager, watcherScope)
	mustApply(b, manager, watcherScope, watcher.SessionID, Command{Action: ActionSubscribe, Topic: "devices/telemetry"})

	matching := TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-1", Topic: "devices/telemetry", Payload: []byte(`{"t":1}`)}
	unwatched := TrustedUplinkMessage{TenantID: "tenant-1", DeviceID: "device-unwatched", Topic: "devices/telemetry", Payload: []byte(`{"t":1}`)}

	b.Run("indexed/unwatched_device", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			manager.handleTrustedUplink(unwatched)
		}
	})
	b.Run("indexed/watched_device", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			manager.handleTrustedUplink(matching)
		}
	})
	legacyScan := func(incoming TrustedUplinkMessage) int {
		manager.mu.RLock()
		items := make([]*session, 0)
		for _, item := range manager.index.byID {
			if item.scope.DeviceID == incoming.DeviceID && item.scope.TenantID == incoming.TenantID {
				items = append(items, item)
			}
		}
		manager.mu.RUnlock()
		matched := 0
		for _, item := range items {
			item.mu.Lock()
			for _, current := range item.subscriptions {
				if current.trustedUplink && mqttTopicFilterMatches(current.topic, incoming.Topic) {
					matched++
					break
				}
			}
			item.mu.Unlock()
		}
		return matched
	}
	b.Run("legacy_scan/unwatched_device", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = legacyScan(unwatched)
		}
	})
}
