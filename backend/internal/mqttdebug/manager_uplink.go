package mqttdebug

import (
	"fmt"
	"time"
)

// trustedFanoutStackTargets sizes the on-stack candidate buffer. A device is
// normally watched by at most a handful of debug sessions (MaxSessionsPerUser
// per user), so the common case allocates nothing for the target list.
const trustedFanoutStackTargets = 8

// handleTrustedUplink runs on the accepted-uplink observer goroutine for every
// bus-accepted message while the observer is active, so it is the hot path:
//
//  1. one RLock + one map lookup by (tenant, device) instead of scanning all
//     sessions;
//  2. lock-free filter matching against each candidate's immutable
//     subscriptionView, so sessions without a matching trusted subscription
//     are never locked;
//  3. the payload is bounded and stringified once and shared by every matching
//     session (Go strings are immutable).
func (manager *Manager) handleTrustedUplink(incoming TrustedUplinkMessage) {
	var stack [trustedFanoutStackTargets]*session
	targets := manager.trustedUplinkTargets(incoming, stack[:0])
	if len(targets) == 0 {
		return
	}
	payload := incoming.Payload
	truncated := false
	if limit := manager.config.PayloadMaxBytes; limit > 0 && len(payload) > limit {
		payload = payload[:limit]
		truncated = true
	}
	message := Message{
		Direction: "inbound",
		Topic:     incoming.Topic,
		Payload:   string(payload), // the one copy; the observer may reuse its buffer afterwards
		Truncated: truncated,
		Outcome:   "received",
		Source:    SubscriptionModeAcceptedUplink,
	}
	now := time.Now().UTC()
	for _, item := range targets {
		item.mu.Lock()
		manager.appendLocked(item, message, now)
		item.mu.Unlock()
	}
}

// trustedUplinkTargets appends to dst the sessions bound to the message's
// trusted (tenant, device) identity that hold a trusted-uplink filter matching
// its topic.
func (manager *Manager) trustedUplinkTargets(incoming TrustedUplinkMessage, dst []*session) []*session {
	key := deviceKey{tenantID: incoming.TenantID, deviceID: incoming.DeviceID}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	for _, item := range manager.index.forDevice(key) {
		if item.view.Load().matchesTrusted(incoming.Topic) {
			dst = append(dst, item)
		}
	}
	return dst
}

func (manager *Manager) ensureTrustedUplinkSource() error {
	manager.uplinkStartMu.Lock()
	defer manager.uplinkStartMu.Unlock()
	manager.mu.RLock()
	closed, available := manager.closed, manager.uplinkAvailable
	manager.mu.RUnlock()
	if closed {
		return ErrRuntimeClosed
	}
	if available {
		return nil
	}
	source := manager.uplinkSource
	if source == nil {
		return fmt.Errorf("%w: trusted uplink observer is unavailable", ErrRuntimeClosed)
	}
	stop, err := source.Start(manager.handleTrustedUplink)
	if err != nil {
		manager.logger.WithError(err).Warn("mqtt debug trusted uplink observer unavailable")
		return fmt.Errorf("%w: trusted uplink observer is unavailable", ErrRuntimeClosed)
	}
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		stop()
		return ErrRuntimeClosed
	}
	manager.uplinkStop = stop
	manager.uplinkAvailable = true
	manager.mu.Unlock()
	return nil
}
