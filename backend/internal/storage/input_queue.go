package storage

import (
	"context"
	"errors"
	"sync"
)

var ErrInputClosed = errors.New("storage input is closed")

// MessageEnqueuer is the storage input surface used by message producers.
type MessageEnqueuer interface {
	Enqueue(context.Context, *Message) error
}

// InputQueue owns the channel shared by storage and its producers. Closing the
// queue rejects new work and waits for admitted producers before closing Messages.
type InputQueue struct {
	messages chan *Message
	closed   chan struct{}
	abort    chan struct{}

	mu        sync.Mutex
	accepting bool
	inFlight  sync.WaitGroup
	closeOnce sync.Once
	abortOnce sync.Once
}

func NewInputQueue(bufferSize int) *InputQueue {
	if bufferSize < 0 {
		bufferSize = 0
	}
	return &InputQueue{
		messages:  make(chan *Message, bufferSize),
		closed:    make(chan struct{}),
		abort:     make(chan struct{}),
		accepting: true,
	}
}

func (q *InputQueue) Messages() <-chan *Message {
	return q.messages
}

func (q *InputQueue) Enqueue(ctx context.Context, msg *Message) error {
	if ctx == nil {
		ctx = context.Background()
	}
	q.mu.Lock()
	if !q.accepting {
		q.mu.Unlock()
		return ErrInputClosed
	}
	q.inFlight.Add(1)
	q.mu.Unlock()
	defer q.inFlight.Done()

	select {
	case q.messages <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-q.abort:
		return ErrInputClosed
	}
}

func (q *InputQueue) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	q.closeOnce.Do(func() {
		q.mu.Lock()
		q.accepting = false
		q.mu.Unlock()

		go func() {
			q.inFlight.Wait()
			close(q.messages)
			close(q.closed)
		}()
	})

	select {
	case <-q.closed:
		return nil
	case <-ctx.Done():
		// Stop waiting producers only after the caller's drain budget expires.
		// They return an explicit error instead of racing a send against channel
		// close. Wait for all admitted calls to leave before returning so the
		// consumer sees a stable, closed channel even on the timeout path.
		q.abortOnce.Do(func() { close(q.abort) })
		<-q.closed
		return ctx.Err()
	}
}
