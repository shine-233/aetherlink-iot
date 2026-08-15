package uplink

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestUplinkManagerStopDrainsQueuedMessageBeforeHandlerExit(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	processed := make(chan struct{})
	logger.AddHook(&busTestLogHook{
		needle: "Device ID not found in message metadata",
		fired:  processed,
	})

	bus := NewBus(BusConfig{BufferSize: 1}, logger)
	attribute := NewAttributeUplink(AttributeUplinkConfig{Logger: logger})
	manager := NewUplinkManager(UplinkManagerConfig{
		Bus:             bus,
		AttributeUplink: attribute,
		Logger:          logger,
	})
	t.Cleanup(func() {
		bus.Close()
		attribute.Stop()
	})

	if err := manager.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := bus.Publish(&DeviceMessage{
		Type:     MessageTypeAttribute,
		DeviceID: "dev-drain-contract",
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if err := manager.Stop(time.Second); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	select {
	case <-processed:
	default:
		t.Fatal("Stop returned before the queued attribute message was processed")
	}
	select {
	case <-attribute.Done():
	default:
		t.Fatal("attribute handler was still running after Stop")
	}
}

func TestUplinkManagerStopReturnsDeadlineWhenBusPublisherDoesNotExit(t *testing.T) {
	logger := logrus.New()
	fullQueue := make(chan struct{})
	logger.AddHook(&busTestLogHook{
		needle: "Telemetry channel full, blocking publish",
		fired:  fullQueue,
	})
	bus := NewBus(BusConfig{BufferSize: 1}, logger)
	manager := NewUplinkManager(UplinkManagerConfig{Bus: bus, Logger: logger})
	publishDone := make(chan error, 1)

	t.Cleanup(bus.Close)
	if err := bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-buffered"}); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}

	go func() {
		publishDone <- bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-blocked"})
	}()

	select {
	case <-fullQueue:
	case <-time.After(time.Second):
		t.Fatal("publisher did not block on the full telemetry queue")
	}

	if err := manager.Stop(20 * time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop() error = %v, want context deadline exceeded", err)
	}

	select {
	case err := <-publishDone:
		if !errors.Is(err, ErrBusClosed) {
			t.Fatalf("Publish() error after Stop = %v, want ErrBusClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("publisher did not exit after Stop aborted pending publishes")
	}

	bus.Close()
}
