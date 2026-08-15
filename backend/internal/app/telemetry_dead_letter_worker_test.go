package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
)

func TestTelemetryDeadLetterWorkerConfigDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	if !telemetryDeadLetterWorkerEnabled() {
		t.Fatal("worker should be enabled when the config key is absent")
	}
	if got := telemetryDeadLetterWorkerInterval(); got != defaultDeadLetterWorkerInterval {
		t.Fatalf("interval = %s, want %s", got, defaultDeadLetterWorkerInterval)
	}
	if got := telemetryDeadLetterWorkerLimit(); got != defaultDeadLetterWorkerLimit {
		t.Fatalf("limit = %d, want %d", got, defaultDeadLetterWorkerLimit)
	}
	if got := telemetryDeadLetterWorkerShutdownTimeout(); got != defaultDeadLetterWorkerShutdownTimeout {
		t.Fatalf("shutdown timeout = %s, want %s", got, defaultDeadLetterWorkerShutdownTimeout)
	}
}

func TestTelemetryDeadLetterWorkerConfigOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("telemetry_dead_letters.worker.enabled", false)
	viper.Set("telemetry_dead_letters.worker.interval", "45s")
	viper.Set("telemetry_dead_letters.worker.limit", 12)
	viper.Set("telemetry_dead_letters.worker.shutdown_timeout", "7s")

	if telemetryDeadLetterWorkerEnabled() {
		t.Fatal("worker should respect an explicit disabled flag")
	}
	if got := telemetryDeadLetterWorkerInterval(); got != 45*time.Second {
		t.Fatalf("interval = %s, want 45s", got)
	}
	if got := telemetryDeadLetterWorkerLimit(); got != 12 {
		t.Fatalf("limit = %d, want 12", got)
	}
	if got := telemetryDeadLetterWorkerShutdownTimeout(); got != 7*time.Second {
		t.Fatalf("shutdown timeout = %s, want 7s", got)
	}
}

func TestTelemetryDeadLetterWorkerLimitBounds(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("telemetry_dead_letters.worker.limit", -1)
	if got := telemetryDeadLetterWorkerLimit(); got != defaultDeadLetterWorkerLimit {
		t.Fatalf("negative limit = %d, want default %d", got, defaultDeadLetterWorkerLimit)
	}

	viper.Set("telemetry_dead_letters.worker.limit", 500)
	if got := telemetryDeadLetterWorkerLimit(); got != 100 {
		t.Fatalf("large limit = %d, want 100", got)
	}
}

func TestTelemetryDeadLetterWorkerStartStopDrainsImmediatelyAndOnTick(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	var calls atomic.Int32
	drained := make(chan int, 4)
	worker := &TelemetryDeadLetterWorker{
		interval:        10 * time.Millisecond,
		limit:           7,
		shutdownTimeout: 100 * time.Millisecond,
		drainReady: func(_ context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
			calls.Add(1)
			select {
			case drained <- limit:
			default:
			}
			return &model.DrainTelemetryDeadLetterRsp{TotalReady: 1, Attempted: 1, Replayed: 1}, nil
		},
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = worker.Stop() })

	select {
	case got := <-drained:
		if got != 7 {
			t.Fatalf("drain limit = %d, want 7", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not drain immediately")
	}

	select {
	case <-drained:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not drain on ticker")
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("drain calls = %d, want at least 2", calls.Load())
	}
	if err := worker.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestTelemetryDeadLetterWorkerStopRunsOneFinalDrain(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	drained := make(chan int, 3)
	worker := &TelemetryDeadLetterWorker{
		interval:        time.Hour,
		limit:           7,
		shutdownTimeout: 200 * time.Millisecond,
		drainReady: func(_ context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
			drained <- limit
			return &model.DrainTelemetryDeadLetterRsp{}, nil
		},
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	select {
	case limit := <-drained:
		if limit != 7 {
			t.Fatalf("initial drain limit = %d, want 7", limit)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not run the initial drain")
	}

	if err := worker.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	select {
	case limit := <-drained:
		if limit != 7 {
			t.Fatalf("shutdown drain limit = %d, want 7", limit)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not run the shutdown final drain")
	}

	select {
	case <-drained:
		t.Fatal("worker ran more than one shutdown final drain")
	default:
	}
}

func TestTelemetryDeadLetterWorkerStopCancelsBlockedFinalDrain(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	initialDrainDone := make(chan struct{}, 1)
	finalDrainStarted := make(chan struct{}, 1)
	finalDrainCanceled := make(chan struct{}, 1)

	var calls atomic.Int32
	worker := &TelemetryDeadLetterWorker{
		interval:        time.Hour,
		limit:           5,
		shutdownTimeout: 30 * time.Millisecond,
		drainReady: func(ctx context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
			switch calls.Add(1) {
			case 1:
				initialDrainDone <- struct{}{}
			case 2:
				finalDrainStarted <- struct{}{}
				<-ctx.Done()
				finalDrainCanceled <- struct{}{}
				return nil, ctx.Err()
			}
			return &model.DrainTelemetryDeadLetterRsp{}, nil
		},
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	select {
	case <-initialDrainDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not run the initial drain")
	}

	stopStarted := time.Now()
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- worker.Stop()
	}()

	select {
	case <-finalDrainStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not start the shutdown final drain")
	}

	select {
	case err := <-stopDone:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
		if elapsed := time.Since(stopStarted); elapsed >= 200*time.Millisecond {
			t.Fatalf("Stop took %s, want a bounded return", elapsed)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop blocked past the shutdown timeout")
	}

	select {
	case <-finalDrainCanceled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("shutdown final drain did not observe context cancellation")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("drain calls = %d, want initial plus one shutdown final drain", got)
	}
}

func TestTelemetryDeadLetterWorkerDisabledDoesNotDrain(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("telemetry_dead_letters.worker.enabled", false)

	var calls atomic.Int32
	worker := &TelemetryDeadLetterWorker{
		interval:        time.Millisecond,
		limit:           1,
		shutdownTimeout: 10 * time.Millisecond,
		drainReady: func(_ context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
			calls.Add(1)
			return &model.DrainTelemetryDeadLetterRsp{}, nil
		},
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Fatalf("drain calls = %d, want 0", got)
	}
	if err := worker.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestTelemetryDeadLetterWorkerDrainErrorDoesNotStopTicker(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	var calls atomic.Int32
	drained := make(chan struct{}, 4)
	worker := &TelemetryDeadLetterWorker{
		interval:        10 * time.Millisecond,
		limit:           1,
		shutdownTimeout: 100 * time.Millisecond,
		drainReady: func(_ context.Context, limit int) (*model.DrainTelemetryDeadLetterRsp, error) {
			call := calls.Add(1)
			select {
			case drained <- struct{}{}:
			default:
			}
			if call == 1 {
				return nil, errors.New("temporary drain failure")
			}
			return &model.DrainTelemetryDeadLetterRsp{}, nil
		},
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = worker.Stop() })

	select {
	case <-drained:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not run first drain")
	}
	select {
	case <-drained:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker stopped after drain error")
	}
}
