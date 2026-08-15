package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/storage"

	"github.com/spf13/viper"
)

type storageStopOrderProbe struct {
	ctx       context.Context
	input     <-chan *storage.Message
	stopCalls int
	drained   int
}

type durabilityDrainProbe struct {
	stopAcceptingCalls int
	waitCalls          int
}

func (*durabilityDrainProbe) Enqueue(context.Context, *storage.Message) error {
	return nil
}

func (*durabilityDrainProbe) PersistDurably(context.Context, *storage.Message) (storage.DurabilityReceipt, error) {
	return storage.DurabilityReceipt{MessageID: "11111111-1111-5111-8111-111111111111", Tier: storage.DurabilityTierPrimary}, nil
}

func (p *durabilityDrainProbe) StopAccepting() {
	p.stopAcceptingCalls++
}

func (p *durabilityDrainProbe) Wait(context.Context) error {
	p.waitCalls++
	if p.stopAcceptingCalls == 0 {
		return errors.New("durability wait started before producer admission stopped")
	}
	return nil
}

func (p *storageStopOrderProbe) Start(context.Context, <-chan *storage.Message) error {
	return nil
}

func (p *storageStopOrderProbe) Stop(time.Duration) error {
	p.stopCalls++
	for {
		select {
		case _, ok := <-p.input:
			if !ok {
				if err := p.ctx.Err(); err != nil {
					return fmt.Errorf("storage context canceled before buffered drain: %w", err)
				}
				return nil
			}
			p.drained++
		default:
			return fmt.Errorf("storage input channel was not closed before Stop")
		}
	}
}

func (p *storageStopOrderProbe) GetMetrics() storage.Metrics {
	return storage.Metrics{}
}

func TestStorageServiceWrapperClosesInputBeforeStopAndCancelsContextAfterward(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	input := storage.NewInputQueue(1)
	if err := input.Enqueue(context.Background(), &storage.Message{DataType: storage.DataTypeTelemetry}); err != nil {
		t.Fatalf("enqueue storage probe message: %v", err)
	}
	probe := &storageStopOrderProbe{ctx: ctx, input: input.Messages()}
	durabilityProbe := &durabilityDrainProbe{}
	wrapper := &StorageServiceWrapper{
		storage:       probe,
		input:         input,
		producerInput: durabilityProbe,
		ctx:           ctx,
		cancel:        cancel,
	}

	if err := wrapper.Stop(); err != nil {
		t.Fatalf("first Stop returned error: %v", err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("context error after Stop = %v, want context.Canceled", ctx.Err())
	}
	if err := wrapper.Stop(); err != nil {
		t.Fatalf("second Stop returned error: %v", err)
	}
	if probe.stopCalls != 1 {
		t.Fatalf("storage Stop calls = %d, want 1", probe.stopCalls)
	}
	if probe.drained != 1 {
		t.Fatalf("drained messages = %d, want 1", probe.drained)
	}
	if durabilityProbe.stopAcceptingCalls != 1 || durabilityProbe.waitCalls != 1 {
		t.Fatalf(
			"durability shutdown calls = stop:%d wait:%d, want 1/1",
			durabilityProbe.stopAcceptingCalls,
			durabilityProbe.waitCalls,
		)
	}
}

func TestApplicationGetStorageInputPreservesNilInterface(t *testing.T) {
	app := &Application{}
	if input := app.GetStorageInput(); input != nil {
		t.Fatalf("GetStorageInput() = %#v, want nil", input)
	}
}

func TestWithStorageServiceExposesDurableProducerInput(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("storage.enable_metrics", false)
	viper.Set("storage.telemetry_spool.directory", t.TempDir())
	viper.Set("storage.attribute_event_spool.directory", t.TempDir())
	viper.Set("storage.telemetry_write_ahead_spool_enabled", false)

	app, appErr := NewApplication(WithStorageService())
	if appErr != nil {
		t.Fatalf("register storage service: %v", appErr)
	}
	if len(app.ServiceManager.services) != 1 {
		t.Fatalf("registered services = %d, want 1 storage service", len(app.ServiceManager.services))
	}
	wrapper, ok := app.ServiceManager.services[0].(*StorageServiceWrapper)
	if !ok {
		t.Fatalf("registered service = %T, want *StorageServiceWrapper", app.ServiceManager.services[0])
	}
	if _, ok := app.GetStorageInput().(storage.DurableMessagePersister); !ok {
		t.Fatalf("storage input = %T, want storage.DurableMessagePersister", app.GetStorageInput())
	}
	if err := wrapper.input.Close(context.Background()); err != nil {
		t.Fatalf("close raw storage queue: %v", err)
	}

	err := app.GetStorageInput().Enqueue(context.Background(), &storage.Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		DataType:  storage.DataTypeTelemetry,
		Timestamp: 1000,
		Data: []storage.TelemetryDataPoint{
			{Key: "temperature", Value: 21.5},
		},
	})
	if !errors.Is(err, storage.ErrInputClosed) {
		t.Fatalf("producer input error = %v, want ErrInputClosed", err)
	}
	metrics := app.GetStorageService().GetMetrics()
	if metrics.TelemetrySpooled != 1 || metrics.TelemetrySpoolBacklog != 1 {
		t.Fatalf("producer fallback metrics = %#v, want one spooled point", metrics)
	}
}
