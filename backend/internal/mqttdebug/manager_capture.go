package mqttdebug

import (
	"context"
	"fmt"
	"time"
)

// appendMessage records message in item's bounded capture ring. It never takes
// Manager.mu: callers already hold the session pointer, and a closed session
// silently drops the message.
func (manager *Manager) appendMessage(item *session, message Message) {
	if item == nil {
		return
	}
	now := time.Now().UTC()
	item.mu.Lock()
	defer item.mu.Unlock()
	manager.appendLocked(item, message, now)
}

// appendLocked is appendMessage with item.mu held and the clock pre-read.
func (manager *Manager) appendLocked(item *session, message Message, now time.Time) {
	if item.closed {
		return
	}
	if message.Direction == "inbound" && manager.exceedsInboundCaptureBudgetLocked(item, len(message.Payload), now) {
		item.droppedMessages++
		return
	}
	item.lastSequence++
	message.Sequence = item.lastSequence
	message.Timestamp = now.Format(time.RFC3339Nano)
	if len(message.Payload) > manager.config.PayloadMaxBytes {
		message.Payload = message.Payload[:manager.config.PayloadMaxBytes]
		message.Truncated = true
	}
	if item.messages.push(message) {
		item.droppedMessages++
	}
}

func (manager *Manager) exceedsInboundCaptureBudgetLocked(item *session, payloadBytes int, now time.Time) bool {
	if item.captureWindowStart.IsZero() || now.Sub(item.captureWindowStart) >= time.Second {
		item.captureWindowStart = now
		item.captureWindowMessages = 0
		item.captureWindowBytes = 0
	}
	if item.captureWindowMessages >= manager.config.MaxInboundPerSecond ||
		item.captureWindowBytes+payloadBytes > manager.config.MaxInboundBytesPerSecond {
		return true
	}
	item.captureWindowMessages++
	item.captureWindowBytes += payloadBytes
	return false
}

func (manager *Manager) Snapshot(_ context.Context, rawScope Scope, sessionID string, afterSequence int64, limit int) (Snapshot, error) {
	_, item, err := manager.scopedSession(rawScope, sessionID)
	if err != nil {
		return Snapshot{}, err
	}
	if manager.exceedsSnapshotBudget(item) {
		return Snapshot{}, fmt.Errorf("%w: too many mqtt debug snapshot requests", ErrRateLimited)
	}
	return manager.snapshot(item, afterSequence, limit)
}

func (manager *Manager) exceedsSnapshotBudget(item *session) bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	now := time.Now().UTC()
	if item.snapshotWindowStart.IsZero() || now.Sub(item.snapshotWindowStart) >= time.Second {
		item.snapshotWindowStart = now
		item.snapshotWindowCount = 0
	}
	if item.snapshotWindowCount >= manager.config.MaxSnapshotsPerSecond {
		return true
	}
	item.snapshotWindowCount++
	return false
}

// snapshot holds item.mu only for the ring copy and scalar reads. Subscription
// copies, the transport connectivity probe and the observer drop counter run
// after the lock is released so a polling UI cannot stall capture.
func (manager *Manager) snapshot(item *session, afterSequence int64, limit int) (Snapshot, error) {
	if limit <= 0 || limit > manager.config.MessageCapacity {
		limit = manager.config.MessageCapacity
	}
	item.mu.Lock()
	if item.closed {
		item.mu.Unlock()
		return Snapshot{}, ErrSessionNotFound
	}
	messages := item.messages.selectAfter(afterSequence, limit)
	view := item.view.Load()
	connected := item.connected
	lastSequence := item.lastSequence
	droppedMessages := item.droppedMessages
	item.mu.Unlock()

	subscriptions, subscriptionDetails := view.snapshotCopies()
	uplinkObserverDroppedMessages := uint64(0)
	if manager.uplinkSource != nil {
		uplinkObserverDroppedMessages = manager.uplinkSource.DroppedMessages()
	}
	return Snapshot{
		SessionID:                     item.id,
		DeviceID:                      item.scope.DeviceID,
		Connected:                     connected && item.transport.IsConnected(),
		CreatedAt:                     item.createdAt,
		ExpiresAt:                     item.expiresAt,
		Subscriptions:                 subscriptions,
		SubscriptionDetails:           subscriptionDetails,
		Messages:                      messages,
		LastSequence:                  lastSequence,
		DroppedMessages:               droppedMessages,
		MessageCapacity:               manager.config.MessageCapacity,
		PayloadMaxBytes:               manager.config.PayloadMaxBytes,
		SubscriptionLimit:             manager.config.MaxSubscriptions,
		UplinkObserverDroppedMessages: uplinkObserverDroppedMessages,
	}, nil
}
