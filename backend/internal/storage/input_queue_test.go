package storage

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

type enqueueObservedContext struct {
	done     chan struct{}
	observed chan struct{}
	once     sync.Once
}

func (c *enqueueObservedContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *enqueueObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.done
}
func (c *enqueueObservedContext) Err() error    { return nil }
func (c *enqueueObservedContext) Value(any) any { return nil }

func TestInputQueueCloseDrainsAcceptedMessagesAndRejectsNewWork(t *testing.T) {
	queue := NewInputQueue(1)
	want := &Message{DeviceID: "device-1", DataType: DataTypeTelemetry}
	if err := queue.Enqueue(context.Background(), want); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if got, ok := <-queue.Messages(); !ok || got != want {
		t.Fatalf("first receive = (%#v, %v), want accepted message", got, ok)
	}
	if got, ok := <-queue.Messages(); ok || got != nil {
		t.Fatalf("second receive = (%#v, %v), want closed channel", got, ok)
	}
	if err := queue.Enqueue(context.Background(), want); !errors.Is(err, ErrInputClosed) {
		t.Fatalf("Enqueue after Close error = %v, want ErrInputClosed", err)
	}
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestInputQueueAcceptsNilContextsWithoutPanicking(t *testing.T) {
	queue := NewInputQueue(1)
	message := &Message{DeviceID: "device-nil-context"}
	if err := queue.Enqueue(nil, message); err != nil {
		t.Fatalf("Enqueue with nil context returned error: %v", err)
	}
	if err := queue.Close(nil); err != nil {
		t.Fatalf("Close with nil context returned error: %v", err)
	}
	if got := <-queue.Messages(); got != message {
		t.Fatalf("received message = %#v, want %#v", got, message)
	}
}

func TestInputQueueEnqueueHonorsContextWhenFull(t *testing.T) {
	queue := NewInputQueue(1)
	if err := queue.Enqueue(context.Background(), &Message{DeviceID: "device-1"}); err != nil {
		t.Fatalf("first Enqueue returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := queue.Enqueue(ctx, &Message{DeviceID: "device-2"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("full queue Enqueue error = %v, want context.Canceled", err)
	}
}

func TestInputQueueCloseWaitsForAnAdmittedSend(t *testing.T) {
	queue := NewInputQueue(1)
	first := &Message{DeviceID: "device-1"}
	second := &Message{DeviceID: "device-2"}
	if err := queue.Enqueue(context.Background(), first); err != nil {
		t.Fatalf("first Enqueue returned error: %v", err)
	}

	ctx := &enqueueObservedContext{
		done:     make(chan struct{}),
		observed: make(chan struct{}),
	}
	enqueueDone := make(chan error, 1)
	go func() { enqueueDone <- queue.Enqueue(ctx, second) }()
	<-ctx.observed

	closeDone := make(chan error, 1)
	go func() { closeDone <- queue.Close(context.Background()) }()
	for {
		queue.mu.Lock()
		accepting := queue.accepting
		queue.mu.Unlock()
		if !accepting {
			break
		}
		runtime.Gosched()
	}
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before admitted send completed: %v", err)
	default:
	}

	if got := <-queue.Messages(); got != first {
		t.Fatalf("first receive = %#v, want first message", got)
	}
	if err := <-enqueueDone; err != nil {
		t.Fatalf("second Enqueue returned error: %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if got, ok := <-queue.Messages(); !ok || got != second {
		t.Fatalf("second receive = (%#v, %v), want admitted message", got, ok)
	}
	if got, ok := <-queue.Messages(); ok || got != nil {
		t.Fatalf("final receive = (%#v, %v), want closed channel", got, ok)
	}
}

func TestInputQueueCloseTimeoutAbortsBlockedAdmittedSendBeforeClosingChannel(t *testing.T) {
	queue := NewInputQueue(1)
	first := &Message{DeviceID: "device-1"}
	second := &Message{DeviceID: "device-2"}
	if err := queue.Enqueue(context.Background(), first); err != nil {
		t.Fatalf("first Enqueue returned error: %v", err)
	}

	ctx := &enqueueObservedContext{
		done:     make(chan struct{}),
		observed: make(chan struct{}),
	}
	enqueueDone := make(chan error, 1)
	go func() { enqueueDone <- queue.Enqueue(ctx, second) }()
	<-ctx.observed

	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := queue.Close(closeCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close error = %v, want context deadline exceeded", err)
	}
	if err := <-enqueueDone; !errors.Is(err, ErrInputClosed) {
		t.Fatalf("blocked Enqueue error = %v, want ErrInputClosed", err)
	}
	if got, ok := <-queue.Messages(); !ok || got != first {
		t.Fatalf("first receive = (%#v, %v), want accepted first message", got, ok)
	}
	if got, ok := <-queue.Messages(); ok || got != nil {
		t.Fatalf("second receive = (%#v, %v), want closed channel", got, ok)
	}
}
