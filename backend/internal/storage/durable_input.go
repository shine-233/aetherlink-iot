package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrInputUnavailable = errors.New("storage input is unavailable")

type DurabilityTier string

const (
	DurabilityTierPrimary            DurabilityTier = "primary"
	DurabilityTierPostgresDeadLetter DurabilityTier = "postgres_dead_letter"
	DurabilityTierFileSpool          DurabilityTier = "file_spool"
)

// DurabilityReceipt identifies the exact durable tier that accepted one
// attribute/event envelope. A final failure returns the zero receipt and an
// error; callers therefore cannot confuse queue admission with persistence.
type DurabilityReceipt struct {
	MessageID string         `json:"message_id"`
	Tier      DurabilityTier `json:"tier"`
}

// RejectedTelemetryPersister owns the durable fallback for telemetry that was
// rejected before it reached the storage consumer.
type RejectedTelemetryPersister interface {
	PersistRejectedTelemetry(context.Context, *Message, error) error
}

// TelemetryPreEnqueuer is the producer-side durability hook. Implementations
// must return an error before queue admission when every telemetry point has
// not been durably recorded.
type TelemetryPreEnqueuer interface {
	PreEnqueueTelemetry(context.Context, *Message) error
}

// DurableMessageInput owns producer admission across both the in-memory queue
// call and any synchronous durability fallback that follows a rejection. The
// split shutdown methods let the application stop new producers before it
// closes the primary queue, then wait for already-started fallbacks.
type DurableMessageInput interface {
	MessageEnqueuer
	DurableMessagePersister
	StopAccepting()
	Wait(context.Context) error
}

// DurableMessagePersister is the producer-facing synchronous durability seam.
// Nil means the complete canonical attribute/event envelope is retained by its
// primary transaction, PostgreSQL dead letter, or independent private spool.
type DurableMessagePersister interface {
	PersistDurably(context.Context, *Message) (DurabilityReceipt, error)
}

// DurableStorage is the storage lifecycle plus its pre-enqueue telemetry
// durability boundary. Keeping the fallback on the same storage instance
// ensures it reuses the writer's dead-letter database and file spool.
type DurableStorage interface {
	Storage
	RejectedTelemetryPersister
	TelemetryPreEnqueuer
	DurableMessagePersister
}

type durableMessageEnqueuer struct {
	primary          MessageEnqueuer
	persister        RejectedTelemetryPersister
	durablePersister DurableMessagePersister
	preEnqueuer      TelemetryPreEnqueuer

	mu        sync.Mutex
	accepting bool
	active    int
	idle      chan struct{}
}

// NewDurableMessageEnqueuer wraps the in-memory input queue without changing
// its admission result. Rejected telemetry is durably deferred, but callers
// still receive the original enqueue error and must not treat it as real-time
// acceptance. Attribute and event messages retain their existing behavior.
func NewDurableMessageEnqueuer(
	primary MessageEnqueuer,
	persister RejectedTelemetryPersister,
) DurableMessageInput {
	idle := make(chan struct{})
	close(idle)
	input := &durableMessageEnqueuer{
		primary:   primary,
		persister: persister,
		accepting: true,
		idle:      idle,
	}
	if durablePersister, ok := persister.(DurableMessagePersister); ok {
		input.durablePersister = durablePersister
	}
	if preEnqueuer, ok := persister.(TelemetryPreEnqueuer); ok {
		input.preEnqueuer = preEnqueuer
	}
	return input
}

func (d *durableMessageEnqueuer) Enqueue(ctx context.Context, msg *Message) error {
	if d == nil || !d.begin() {
		return ErrInputClosed
	}
	defer d.finish()

	queuedMessage := msg
	if msg != nil && (msg.DataType == DataTypeAttribute || msg.DataType == DataTypeEvent) {
		// Keep legacy async callers safe: freeze mutable interface payloads before
		// another goroutine observes the queued message.
		envelope, err := buildAttributeEventEnvelope(msg)
		if err != nil {
			return err
		}
		queuedMessage, err = messageFromAttributeEventEnvelope(envelope)
		if err != nil {
			return err
		}
	}
	if msg != nil && msg.DataType == DataTypeTelemetry && d.preEnqueuer != nil {
		if preErr := d.preEnqueuer.PreEnqueueTelemetry(ctx, msg); preErr != nil {
			if d.persister == nil {
				return preErr
			}
			if persistErr := d.persister.PersistRejectedTelemetry(ctx, msg, preErr); persistErr != nil {
				return errors.Join(preErr, fmt.Errorf("persist pre-enqueue telemetry fallback: %w", persistErr))
			}
			return preErr
		}
	}

	enqueueErr := ErrInputUnavailable
	if d.primary != nil {
		enqueueErr = d.primary.Enqueue(ctx, queuedMessage)
	}
	if enqueueErr == nil {
		return nil
	}
	if msg == nil || msg.DataType != DataTypeTelemetry {
		return enqueueErr
	}
	if d.persister == nil {
		return errors.Join(enqueueErr, fmt.Errorf("rejected telemetry persister is unavailable"))
	}
	if persistErr := d.persister.PersistRejectedTelemetry(ctx, msg, enqueueErr); persistErr != nil {
		return errors.Join(enqueueErr, fmt.Errorf("persist rejected telemetry: %w", persistErr))
	}
	return enqueueErr
}

// PersistDurably shares the same admission/drain gate as telemetry enqueue
// fallback, so application shutdown waits for already-started receipts before
// stopping the storage-owned replay and spool lifecycle.
func (d *durableMessageEnqueuer) PersistDurably(ctx context.Context, msg *Message) (DurabilityReceipt, error) {
	if d == nil || !d.begin() {
		return DurabilityReceipt{}, ErrInputClosed
	}
	defer d.finish()
	if d.durablePersister == nil {
		return DurabilityReceipt{}, fmt.Errorf("durable message persister is unavailable")
	}
	return d.durablePersister.PersistDurably(ctx, msg)
}

func (d *durableMessageEnqueuer) begin() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.accepting {
		return false
	}
	if d.active == 0 {
		d.idle = make(chan struct{})
	}
	d.active++
	return true
}

func (d *durableMessageEnqueuer) finish() {
	d.mu.Lock()
	d.active--
	if d.active == 0 {
		close(d.idle)
	}
	d.mu.Unlock()
}

func (d *durableMessageEnqueuer) StopAccepting() {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.accepting = false
	d.mu.Unlock()
}

func (d *durableMessageEnqueuer) Wait(ctx context.Context) error {
	if d == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	d.mu.Lock()
	idle := d.idle
	d.mu.Unlock()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *storage) PersistRejectedTelemetry(ctx context.Context, msg *Message, cause error) error {
	if s == nil || s.telemetryWriter == nil {
		return fmt.Errorf("telemetry writer is unavailable")
	}
	if msg == nil {
		return fmt.Errorf("telemetry message is nil")
	}
	if msg.DataType != DataTypeTelemetry {
		return fmt.Errorf("rejected message data type %q is not telemetry", msg.DataType)
	}
	parent := context.Background()
	if ctx != nil {
		parent = context.WithoutCancel(ctx)
	}
	timeout := s.config.TelemetrySpoolReplayTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	durabilityCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return s.telemetryWriter.persistRejectedTelemetry(durabilityCtx, msg, cause)
}

func (s *storage) PreEnqueueTelemetry(ctx context.Context, msg *Message) error {
	if s == nil || s.telemetryWriter == nil {
		return fmt.Errorf("telemetry writer is unavailable")
	}
	if msg == nil {
		return fmt.Errorf("telemetry message is nil")
	}
	if msg.DataType != DataTypeTelemetry {
		return fmt.Errorf("pre-enqueue message data type %q is not telemetry", msg.DataType)
	}
	return s.telemetryWriter.prepareTelemetryWriteAhead(ctx, msg)
}
