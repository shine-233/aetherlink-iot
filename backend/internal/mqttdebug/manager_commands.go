package mqttdebug

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (manager *Manager) Apply(_ context.Context, rawScope Scope, sessionID string, command Command) (Snapshot, error) {
	_, item, err := manager.scopedSession(rawScope, sessionID)
	if err != nil {
		return Snapshot{}, err
	}
	item.commandMu.Lock()
	defer item.commandMu.Unlock()
	item.mu.Lock()
	closed := item.closed
	item.mu.Unlock()
	if closed {
		return Snapshot{}, ErrSessionNotFound
	}
	action := strings.ToLower(strings.TrimSpace(command.Action))
	if command.QoS > 1 {
		return Snapshot{}, fmt.Errorf("%w: qos must be 0 or 1", ErrInvalidCommand)
	}
	if action != ActionSubscribe && action != ActionUnsubscribe && action != ActionPublish {
		return Snapshot{}, fmt.Errorf("%w: action must be subscribe, unsubscribe or publish", ErrInvalidCommand)
	}
	if manager.exceedsCommandBudget(item, action, len(command.Payload)) {
		return Snapshot{}, fmt.Errorf("%w: too many mqtt debug commands", ErrRateLimited)
	}
	switch action {
	case ActionSubscribe:
		err = manager.subscribe(item, command.Topic, command.QoS)
	case ActionUnsubscribe:
		err = manager.unsubscribe(item, command.Topic)
	case ActionPublish:
		err = manager.publish(item, command.Topic, command.QoS, command.Payload)
	}
	if err != nil {
		return Snapshot{}, err
	}
	return manager.snapshot(item, 0, manager.config.MessageCapacity)
}

func (manager *Manager) exceedsCommandBudget(item *session, action string, payloadBytes int) bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	now := time.Now().UTC()
	if item.commandWindowStart.IsZero() || now.Sub(item.commandWindowStart) >= time.Second {
		item.commandWindowStart = now
		item.commandWindowCount = 0
		item.commandWindowPublishBytes = 0
	}
	if item.commandWindowCount >= manager.config.MaxCommandsPerSecond {
		return true
	}
	if action == ActionPublish && item.commandWindowPublishBytes+payloadBytes > manager.config.MaxPublishBytesPerSecond {
		return true
	}
	item.commandWindowCount++
	if action == ActionPublish {
		item.commandWindowPublishBytes += payloadBytes
	}
	return false
}

// subscribe runs under item.commandMu (via Apply).
func (manager *Manager) subscribe(item *session, rawTopic string, qos byte) error {
	topic, trustedUplink, err := authorizeTopic(item.scope, rawTopic, true)
	if err != nil {
		return err
	}
	item.mu.Lock()
	if item.closed {
		item.mu.Unlock()
		return ErrSessionNotFound
	}
	existing, exists := item.subscriptions[topic]
	if exists && existing.qos == qos {
		item.mu.Unlock()
		return nil
	}
	if !exists && len(item.subscriptions) >= manager.config.MaxSubscriptions {
		item.mu.Unlock()
		return fmt.Errorf("%w: at most %d subscriptions per session", ErrSessionCapacity, manager.config.MaxSubscriptions)
	}
	item.mu.Unlock()

	if trustedUplink {
		if err := manager.ensureTrustedUplinkSource(); err != nil {
			return err
		}
	} else if err := item.transport.Subscribe(topic, qos, manager.incomingHandler(item)); err != nil {
		manager.appendMessage(item, Message{Direction: "system", Topic: topic, Outcome: "subscribe_failed"})
		return fmt.Errorf("mqtt debug subscribe: %w", err)
	}

	item.mu.Lock()
	if item.closed {
		item.mu.Unlock()
		if !trustedUplink {
			_ = item.transport.Unsubscribe(topic)
		}
		return ErrSessionNotFound
	}
	item.setSubscriptionLocked(subscription{topic: topic, qos: qos, trustedUplink: trustedUplink})
	item.mu.Unlock()
	manager.appendMessage(item, Message{Direction: "system", Topic: topic, QoS: qos, Outcome: "subscribed", Source: subscriptionSource(trustedUplink)})
	return nil
}

// unsubscribe runs under item.commandMu (via Apply), so the subscription read
// here cannot change before the delete below.
func (manager *Manager) unsubscribe(item *session, rawTopic string) error {
	topic, _, err := authorizeTopic(item.scope, rawTopic, true)
	if err != nil {
		return err
	}
	item.mu.Lock()
	current, exists := item.subscriptions[topic]
	item.mu.Unlock()
	if !exists {
		return nil
	}
	if !current.trustedUplink {
		if err := item.transport.Unsubscribe(topic); err != nil {
			manager.appendMessage(item, Message{Direction: "system", Topic: topic, Outcome: "unsubscribe_failed"})
			return fmt.Errorf("mqtt debug unsubscribe: %w", err)
		}
	}
	item.mu.Lock()
	item.deleteSubscriptionLocked(topic)
	item.mu.Unlock()
	manager.appendMessage(item, Message{Direction: "system", Topic: topic, Outcome: "unsubscribed", Source: subscriptionSource(current.trustedUplink)})
	return nil
}

func (manager *Manager) publish(item *session, rawTopic string, qos byte, payload string) error {
	topic, _, err := authorizeTopic(item.scope, rawTopic, false)
	if err != nil {
		return err
	}
	if len(payload) > manager.config.PublishMaxBytes {
		return fmt.Errorf("%w: publish payload exceeds %d bytes", ErrInvalidCommand, manager.config.PublishMaxBytes)
	}
	if err := item.transport.Publish(topic, qos, []byte(payload)); err != nil {
		manager.appendMessage(item, Message{Direction: "outbound", Topic: topic, QoS: qos, Outcome: "publish_failed"})
		return fmt.Errorf("mqtt debug publish: %w", err)
	}
	manager.appendMessage(item, Message{Direction: "outbound", Topic: topic, QoS: qos, Payload: payload, Outcome: "published", Source: "broker_publish"})
	return nil
}

// incomingHandler binds broker deliveries to the session pointer directly:
// no manager lock or map lookup per message. Deliveries after close are
// dropped by appendMessage's closed check.
func (manager *Manager) incomingHandler(item *session) func(IncomingMessage) {
	return func(incoming IncomingMessage) {
		manager.appendMessage(item, Message{
			Direction: "inbound",
			Topic:     incoming.Topic,
			QoS:       incoming.QoS,
			Retained:  incoming.Retained,
			Duplicate: incoming.Duplicate,
			Truncated: incoming.Truncated,
			Payload:   string(incoming.Payload),
			Outcome:   "received",
			Source:    SubscriptionModeBroker,
		})
	}
}

func subscriptionSource(trustedUplink bool) string {
	if trustedUplink {
		return SubscriptionModeAcceptedUplink
	}
	return SubscriptionModeBroker
}
