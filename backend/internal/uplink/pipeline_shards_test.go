package uplink

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestShardedBase(t *testing.T, shards int) *uplinkBase {
	t.Helper()
	b := newUplinkBase(telemetryKindSpec, nil, nil, quietLogger())
	b.SetShards(shards)
	t.Cleanup(b.Stop)
	return &b
}

// TestShardedRunKeepsPerDeviceOrder: 1000 messages over 3 devices must be seen
// by the handler in publish order per device, and all of them must be
// processed before Done closes (drain-on-close contract).
func TestShardedRunKeepsPerDeviceOrder(t *testing.T) {
	b := newTestShardedBase(t, 4)
	in := make(chan *DeviceMessage, 16)

	var mu sync.Mutex
	seen := map[string][]int{}
	b.run(in, func(msg *DeviceMessage) {
		seq := msg.Metadata["seq"].(int)
		mu.Lock()
		seen[msg.DeviceID] = append(seen[msg.DeviceID], seq)
		mu.Unlock()
	})

	devices := []string{"dev-a", "dev-b", "dev-c"}
	const total = 1000
	want := map[string][]int{}
	for i := 0; i < total; i++ {
		id := devices[i%len(devices)]
		want[id] = append(want[id], i)
		in <- &DeviceMessage{DeviceID: id, Metadata: map[string]interface{}{"seq": i}}
	}
	close(in)

	select {
	case <-b.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("sharded run did not drain and exit after channel close")
	}

	got := 0
	for _, id := range devices {
		if fmt.Sprint(seen[id]) != fmt.Sprint(want[id]) {
			t.Fatalf("device %s order broken:\n got %v\nwant %v", id, seen[id], want[id])
		}
		got += len(seen[id])
	}
	if got != total {
		t.Fatalf("processed %d messages, want %d", got, total)
	}
}

// TestShardedRunProcessesDevicesInParallel: a slow device must not block a
// device on another shard (the legacy loop processed everything serially).
func TestShardedRunProcessesDevicesInParallel(t *testing.T) {
	const shards = 8
	b := newTestShardedBase(t, shards)
	// Pick two ids that land on different shards.
	slow, fast := "dev-slow", ""
	for i := 0; fast == ""; i++ {
		id := fmt.Sprintf("dev-fast-%d", i)
		if shardIndex(id, shards) != shardIndex(slow, shards) {
			fast = id
		}
	}

	in := make(chan *DeviceMessage, 4)
	release := make(chan struct{})
	fastDone := make(chan struct{})
	b.run(in, func(msg *DeviceMessage) {
		switch msg.DeviceID {
		case slow:
			<-release
		case fast:
			close(fastDone)
		}
	})
	in <- &DeviceMessage{DeviceID: slow}
	in <- &DeviceMessage{DeviceID: fast}

	select {
	case <-fastDone:
	case <-time.After(2 * time.Second):
		t.Fatal("fast device was blocked behind slow device on another shard")
	}
	close(release)
	close(in)
	<-b.Done()
}

// TestShardedRunStopExitsWithBlockedShard: Stop must not hang when the reader
// is blocked on a full shard queue.
func TestShardedRunStopExitsWithBlockedShard(t *testing.T) {
	b := newTestShardedBase(t, 2)
	b.shardQueueSize = 1
	in := make(chan *DeviceMessage, 64)
	block := make(chan struct{})
	b.run(in, func(*DeviceMessage) { <-block })
	for i := 0; i < 64; i++ {
		in <- &DeviceMessage{DeviceID: "same-device"}
	}
	time.Sleep(20 * time.Millisecond)

	b.Stop()
	// Release the worker stuck inside process(); with ctx cancelled the reader
	// and workers must all return even though 60+ messages are still queued.
	close(block)
	select {
	case <-b.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done did not close after Stop")
	}
}

func TestSingleShardKeepsLegacyLoop(t *testing.T) {
	b := newTestShardedBase(t, 1)
	if b.Shards() != 1 {
		t.Fatalf("Shards() = %d, want 1", b.Shards())
	}
	in := make(chan *DeviceMessage, 3)
	var n atomic.Int32
	b.run(in, func(*DeviceMessage) { n.Add(1) })
	for i := 0; i < 3; i++ {
		in <- &DeviceMessage{DeviceID: "d"}
	}
	close(in)
	<-b.Done()
	if n.Load() != 3 {
		t.Fatalf("processed %d, want 3", n.Load())
	}
}

func TestShardKeyFallsBackToMetadata(t *testing.T) {
	if got := shardKey(&DeviceMessage{Metadata: map[string]interface{}{"device_id": "m1"}}); got != "m1" {
		t.Fatalf("shardKey = %q, want m1", got)
	}
	if got := shardKey(&DeviceMessage{DeviceID: "d1", Metadata: map[string]interface{}{"device_id": "m1"}}); got != "d1" {
		t.Fatalf("shardKey = %q, want d1", got)
	}
	if shardKey(nil) != "" || shardIndex("anything", 1) != 0 {
		t.Fatal("nil message / single shard must route to shard 0")
	}
	for i := 0; i < 100; i++ {
		if idx := shardIndex(fmt.Sprint(i), 7); idx < 0 || idx >= 7 {
			t.Fatalf("shardIndex out of range: %d", idx)
		}
	}
}

func TestUplinkManagerShardOverride(t *testing.T) {
	tel := NewTelemetryUplink(TelemetryUplinkConfig{Logger: quietLogger(), Shards: 3})
	attr := NewAttributeUplink(AttributeUplinkConfig{Logger: quietLogger()})
	ev := NewEventUplink(EventUplinkConfig{Logger: quietLogger(), Shards: 5})
	if tel.Shards() != 3 || ev.Shards() != 5 || attr.Shards() != defaultUplinkShards() {
		t.Fatalf("config shards not applied: tel=%d attr=%d ev=%d", tel.Shards(), attr.Shards(), ev.Shards())
	}
	NewUplinkManager(UplinkManagerConfig{TelemetryUplink: tel, AttributeUplink: attr, EventUplink: ev, UplinkShards: 1, Logger: quietLogger()})
	if tel.Shards() != 1 || attr.Shards() != 1 || ev.Shards() != 1 {
		t.Fatalf("manager override not applied: tel=%d attr=%d ev=%d", tel.Shards(), attr.Shards(), ev.Shards())
	}
}
