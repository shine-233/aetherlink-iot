package uplink

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/isolatedqueue"
)

// The isolated queues are a monitoring view with no consumer; recording into
// them must never block the ingest path once their capacity is exceeded.
func TestBusPublishDoesNotStallAfterIsolatedQueueCapacity(t *testing.T) {
	bus := newTestBus(64)
	t.Cleanup(bus.Close)

	before, _ := isolatedqueue.GetDefaultManager().GetQueueStats(isolatedqueue.QueueTypeMain)
	const total = 12000 // Main isolated queue capacity is 10000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < total; i++ {
			<-bus.SubscribeTelemetry()
		}
	}()

	published := make(chan error, 1)
	go func() {
		for i := 0; i < total; i++ {
			if err := bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-stall"}); err != nil {
				published <- err
				return
			}
		}
		published <- nil
	}()

	select {
	case err := <-published:
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("bus.Publish stalled after isolated queue capacity was exceeded")
	}
	<-done

	stats, err := isolatedqueue.GetDefaultManager().GetQueueStats(isolatedqueue.QueueTypeMain)
	if err != nil {
		t.Fatalf("main queue stats: %v", err)
	}
	if stats.TotalSubmitted-before.TotalSubmitted < total {
		t.Fatalf("main queue should account every accepted telemetry message, got %d", stats.TotalSubmitted)
	}
	if stats.Size != 0 {
		t.Fatalf("observed messages must not be retained in memory, size=%d", stats.Size)
	}
}
