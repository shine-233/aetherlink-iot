package storage

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type messageEnqueuerFunc func(context.Context, *Message) error

func (f messageEnqueuerFunc) Enqueue(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

type rejectedTelemetryPersisterFunc func(context.Context, *Message, error) error

func (f rejectedTelemetryPersisterFunc) PersistRejectedTelemetry(ctx context.Context, msg *Message, cause error) error {
	return f(ctx, msg, cause)
}

type telemetryPreEnqueuerFunc func(context.Context, *Message) error

func (f telemetryPreEnqueuerFunc) PreEnqueueTelemetry(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

type preEnqueuePersister struct {
	pre      telemetryPreEnqueuerFunc
	fallback rejectedTelemetryPersisterFunc
}

func (p preEnqueuePersister) PreEnqueueTelemetry(ctx context.Context, msg *Message) error {
	return p.pre(ctx, msg)
}

func (p preEnqueuePersister) PersistRejectedTelemetry(ctx context.Context, msg *Message, cause error) error {
	return p.fallback(ctx, msg, cause)
}

func TestDurableMessageEnqueuerFailsClosedBeforePrimaryQueueOnPreEnqueueFailure(t *testing.T) {
	preErr := errors.New("telemetry write-ahead spool is full")
	primaryCalls := 0
	fallbackCalls := 0
	input := NewDurableMessageEnqueuer(
		messageEnqueuerFunc(func(context.Context, *Message) error {
			primaryCalls++
			return nil
		}),
		preEnqueuePersister{
			pre: telemetryPreEnqueuerFunc(func(context.Context, *Message) error { return preErr }),
			fallback: rejectedTelemetryPersisterFunc(func(_ context.Context, _ *Message, cause error) error {
				fallbackCalls++
				if !errors.Is(cause, preErr) {
					t.Fatalf("fallback cause = %v, want pre-enqueue error", cause)
				}
				return nil
			}),
		},
	)

	err := input.Enqueue(context.Background(), &Message{
		DataType: DataTypeTelemetry,
		Data:     []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	})
	if !errors.Is(err, preErr) {
		t.Fatalf("enqueue error = %v, want pre-enqueue error", err)
	}
	if primaryCalls != 0 {
		t.Fatalf("primary queue calls = %d, want 0 after fail-closed pre-enqueue", primaryCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallbackCalls)
	}
}

func TestDurableMessageEnqueuerPersistsTelemetryBeforePrimaryQueueAdmission(t *testing.T) {
	config := DefaultConfig()
	config.EnableMetrics = false
	config.TelemetrySpoolDirectory = t.TempDir()
	service := New(nil, nil, config)
	queue := NewInputQueue(1)
	input := NewDurableMessageEnqueuer(queue, service)
	message := &Message{
		DeviceID:  "device-pre-enqueue",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 1000,
		Data:      []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	}

	if err := input.Enqueue(context.Background(), message); err != nil {
		t.Fatalf("enqueue telemetry: %v", err)
	}
	if usage := service.(*storage).telemetryWriter.spool.usage(); usage.Records != 1 {
		t.Fatalf("spool records before consumer admission = %d, want 1", usage.Records)
	}
	queued := <-queue.Messages()
	if !queued.telemetryWriteAheadPrepared {
		t.Fatal("queued telemetry is missing the prepared write-ahead marker")
	}
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("close input queue: %v", err)
	}
}

func TestDurableMessageEnqueuerReturnsOriginalErrorAfterDurableFallback(t *testing.T) {
	enqueueErr := errors.New("queue rejected message")
	message := &Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 1000,
		Data:      []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	}
	var capturedMessage *Message
	var capturedCause error
	input := NewDurableMessageEnqueuer(
		messageEnqueuerFunc(func(context.Context, *Message) error { return enqueueErr }),
		rejectedTelemetryPersisterFunc(func(_ context.Context, msg *Message, cause error) error {
			capturedMessage = msg
			capturedCause = cause
			return nil
		}),
	)

	err := input.Enqueue(context.Background(), message)
	if !errors.Is(err, enqueueErr) {
		t.Fatalf("enqueue error = %v, want original queue error", err)
	}
	if capturedMessage != message || !errors.Is(capturedCause, enqueueErr) {
		t.Fatalf("fallback captured message=%p cause=%v, want message=%p cause=%v", capturedMessage, capturedCause, message, enqueueErr)
	}
}

func TestDurableMessageEnqueuerDoesNotPutNonTelemetryIntoTelemetryFallback(t *testing.T) {
	enqueueErr := errors.New("queue rejected message")
	fallbackCalls := 0
	input := NewDurableMessageEnqueuer(
		messageEnqueuerFunc(func(context.Context, *Message) error { return enqueueErr }),
		rejectedTelemetryPersisterFunc(func(context.Context, *Message, error) error {
			fallbackCalls++
			return nil
		}),
	)

	err := input.Enqueue(context.Background(), testAttributeMessage([]AttributeDataPoint{{Key: "firmware", Value: "1.0.0"}}))
	if !errors.Is(err, enqueueErr) {
		t.Fatalf("enqueue error = %v, want original queue error", err)
	}
	if fallbackCalls != 0 {
		t.Fatalf("telemetry fallback calls = %d, want 0 for attribute message", fallbackCalls)
	}
}

func TestDurableMessageEnqueuerJoinsEnqueueAndFallbackFailures(t *testing.T) {
	enqueueErr := errors.New("queue rejected message")
	fallbackErr := errors.New("durability fallback failed")
	input := NewDurableMessageEnqueuer(
		messageEnqueuerFunc(func(context.Context, *Message) error { return enqueueErr }),
		rejectedTelemetryPersisterFunc(func(context.Context, *Message, error) error { return fallbackErr }),
	)

	err := input.Enqueue(context.Background(), &Message{
		DataType: DataTypeTelemetry,
		Data:     []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	})
	if !errors.Is(err, enqueueErr) || !errors.Is(err, fallbackErr) {
		t.Fatalf("joined error = %v, want enqueue and fallback causes", err)
	}
}

func TestDurableMessageEnqueuerPersistsTelemetryWhenPrimaryInputIsUnavailable(t *testing.T) {
	fallbackCalls := 0
	input := NewDurableMessageEnqueuer(
		nil,
		rejectedTelemetryPersisterFunc(func(_ context.Context, _ *Message, cause error) error {
			fallbackCalls++
			if !errors.Is(cause, ErrInputUnavailable) {
				t.Fatalf("fallback cause = %v, want ErrInputUnavailable", cause)
			}
			return nil
		}),
	)

	err := input.Enqueue(context.Background(), &Message{
		DataType: DataTypeTelemetry,
		Data:     []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	})
	if !errors.Is(err, ErrInputUnavailable) {
		t.Fatalf("enqueue error = %v, want ErrInputUnavailable", err)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallbackCalls)
	}
}

func TestDurableMessageEnqueuerPersistsContextCanceledRejection(t *testing.T) {
	queue := NewInputQueue(0)
	t.Cleanup(func() { _ = queue.Close(context.Background()) })
	fallbackCalls := 0
	input := NewDurableMessageEnqueuer(
		queue,
		rejectedTelemetryPersisterFunc(func(_ context.Context, _ *Message, cause error) error {
			fallbackCalls++
			if !errors.Is(cause, context.Canceled) {
				t.Fatalf("fallback cause = %v, want context.Canceled", cause)
			}
			return nil
		}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := input.Enqueue(ctx, &Message{
		DataType: DataTypeTelemetry,
		Data:     []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("enqueue error = %v, want context.Canceled", err)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallbackCalls)
	}
}

func TestStorageFallbackUsesIndependentDurabilityContextAfterCallerCancel(t *testing.T) {
	config := DefaultConfig()
	config.EnableMetrics = false
	config.TelemetryWriteAheadSpoolEnabled = false
	config.TelemetrySpoolDirectory = t.TempDir()
	service := New(nil, nil, config)
	queue := NewInputQueue(0)
	input := NewDurableMessageEnqueuer(queue, service)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := input.Enqueue(ctx, &Message{
		DeviceID:  "device-cancelled",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 1000,
		Data:      []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Enqueue() error = %v, want original context cancellation", err)
	}
	serviceImpl := service.(*storage)
	if usage := serviceImpl.telemetryWriter.spool.usage(); usage.Records != 1 {
		t.Fatalf("spool records after caller cancel = %d, want 1", usage.Records)
	}
	input.StopAccepting()
	if closeErr := queue.Close(context.Background()); closeErr != nil {
		t.Fatalf("close input queue: %v", closeErr)
	}
}

func TestDurableMessageEnqueuerReusesStorageWriterSpoolForEveryRejectedPoint(t *testing.T) {
	config := DefaultConfig()
	config.EnableMetrics = false
	config.TelemetryWriteAheadSpoolEnabled = false
	config.TelemetrySpoolDirectory = t.TempDir()
	service := New(nil, nil, config)
	queue := NewInputQueue(1)
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("close input queue: %v", err)
	}
	input := NewDurableMessageEnqueuer(queue, service)
	message := &Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		DataType:  DataTypeTelemetry,
		Timestamp: 1000,
		Data: []TelemetryDataPoint{
			{Key: "temperature", Value: 21.5},
			{Key: "humidity", Value: 50},
			{Key: "temperature", Value: 21.5},
		},
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := input.Enqueue(context.Background(), message); !errors.Is(err, ErrInputClosed) {
			t.Fatalf("attempt %d enqueue error = %v, want ErrInputClosed", attempt+1, err)
		}
	}

	serviceImpl := service.(*storage)
	usage := serviceImpl.telemetryWriter.spool.usage()
	if usage.Records != 2 {
		t.Fatalf("spool records = %d, want two unique telemetry points after duplicate rejection", usage.Records)
	}
	metrics := service.GetMetrics()
	if metrics.TelemetrySpooled != 2 || metrics.TelemetrySpoolBacklog != 2 {
		t.Fatalf("spool metrics = %#v, want two newly persisted points", metrics)
	}

	var replayed []TelemetryData
	result, err := serviceImpl.telemetryWriter.spool.replay(
		context.Background(),
		10,
		func(_ context.Context, history TelemetryData) error {
			replayed = append(replayed, history)
			return nil
		},
	)
	if err != nil || result.Replayed != 2 || len(replayed) != 2 {
		t.Fatalf("replay result=%#v rows=%#v err=%v, want two rows", result, replayed, err)
	}
	keys := map[string]bool{}
	for _, history := range replayed {
		keys[history.Key] = true
		if history.DeviceID != message.DeviceID || history.TenantID != message.TenantID || history.TS != message.Timestamp {
			t.Fatalf("replayed identity = %#v, want message identity", history)
		}
	}
	if !keys["temperature"] || !keys["humidity"] {
		t.Fatalf("replayed keys = %#v, want temperature and humidity", keys)
	}
}

func TestDurableMessageEnqueuerReportsInvalidRejectedTelemetryWithoutHidingQueueError(t *testing.T) {
	queue := NewInputQueue(1)
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("close input queue: %v", err)
	}
	config := DefaultConfig()
	config.EnableMetrics = false
	config.TelemetryWriteAheadSpoolEnabled = false
	config.TelemetrySpoolDirectory = t.TempDir()
	input := NewDurableMessageEnqueuer(queue, New(nil, nil, config))

	err := input.Enqueue(context.Background(), &Message{
		DataType: DataTypeTelemetry,
		Data:     "not telemetry points",
	})
	if !errors.Is(err, ErrInputClosed) || !strings.Contains(err.Error(), "invalid telemetry data format") {
		t.Fatalf("enqueue error = %v, want queue and invalid-payload causes", err)
	}
}

func TestDurableMessageInputShutdownWaitsForStartedFallback(t *testing.T) {
	enqueueErr := errors.New("queue rejected message")
	fallbackStarted := make(chan struct{})
	releaseFallback := make(chan struct{})
	var startedOnce sync.Once
	input := NewDurableMessageEnqueuer(
		messageEnqueuerFunc(func(context.Context, *Message) error { return enqueueErr }),
		rejectedTelemetryPersisterFunc(func(context.Context, *Message, error) error {
			startedOnce.Do(func() { close(fallbackStarted) })
			<-releaseFallback
			return nil
		}),
	)

	enqueueDone := make(chan error, 1)
	go func() {
		enqueueDone <- input.Enqueue(context.Background(), &Message{
			DataType: DataTypeTelemetry,
			Data:     []TelemetryDataPoint{{Key: "temperature", Value: 21.5}},
		})
	}()
	<-fallbackStarted
	input.StopAccepting()

	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := input.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() error = %v, want deadline while fallback is active", err)
	}
	close(releaseFallback)
	if err := input.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() after fallback completion = %v", err)
	}
	if err := <-enqueueDone; !errors.Is(err, enqueueErr) {
		t.Fatalf("Enqueue() error = %v, want original rejection", err)
	}
	if err := input.Enqueue(context.Background(), &Message{DataType: DataTypeTelemetry}); !errors.Is(err, ErrInputClosed) {
		t.Fatalf("Enqueue() after StopAccepting = %v, want ErrInputClosed", err)
	}
}
