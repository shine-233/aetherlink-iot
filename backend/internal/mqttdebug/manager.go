package mqttdebug

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type subscription struct {
	topic         string
	qos           byte
	trustedUplink bool
}

type session struct {
	commandMu                 sync.Mutex
	mu                        sync.Mutex
	id                        string
	scope                     Scope
	transport                 Transport
	createdAt                 time.Time
	expiresAt                 time.Time
	connected                 bool
	closed                    bool
	subscriptions             map[string]subscription
	messages                  []Message
	lastSequence              int64
	droppedMessages           int64
	captureWindowStart        time.Time
	captureWindowMessages     int
	captureWindowBytes        int
	commandWindowStart        time.Time
	commandWindowCount        int
	commandWindowPublishBytes int
	snapshotWindowStart       time.Time
	snapshotWindowCount       int
	expiryTimer               *time.Timer
}

type Manager struct {
	mu              sync.RWMutex
	config          Config
	logger          *logrus.Logger
	sessions        map[string]*session
	lastOpenByScope map[Scope]time.Time
	closed          bool

	uplinkStartMu   sync.Mutex
	uplinkSource    UplinkSource
	uplinkStop      func()
	uplinkAvailable bool
}

func NewManager(config Config, logger *logrus.Logger) *Manager {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	config = withManagerDefaults(config)
	if config.TransportFactory == nil {
		config.TransportFactory = newPahoTransportFactory(logger)
	}
	manager := &Manager{
		config:          config,
		logger:          logger,
		sessions:        make(map[string]*session),
		lastOpenByScope: make(map[Scope]time.Time),
		uplinkSource:    config.UplinkSource,
	}
	return manager
}

func (manager *Manager) Open(ctx context.Context, rawScope Scope) (Snapshot, error) {
	scope, err := normalizeScope(rawScope)
	if err != nil {
		return Snapshot{}, err
	}

	now := time.Now().UTC()
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return Snapshot{}, ErrRuntimeClosed
	}
	if len(manager.lastOpenByScope) > manager.config.MaxSessions*4 {
		for recordedScope, openedAt := range manager.lastOpenByScope {
			if now.Sub(openedAt) >= manager.config.OpenCooldown {
				delete(manager.lastOpenByScope, recordedScope)
			}
		}
	}
	if lastOpen := manager.lastOpenByScope[scope]; !lastOpen.IsZero() && now.Sub(lastOpen) < manager.config.OpenCooldown {
		manager.mu.Unlock()
		return Snapshot{}, fmt.Errorf("%w: wait before reopening the same device session", ErrRateLimited)
	}
	manager.lastOpenByScope[scope] = now
	manager.mu.Unlock()

	sessionID := uuid.New()
	item := &session{
		id:            sessionID,
		scope:         scope,
		createdAt:     now,
		expiresAt:     now.Add(manager.config.SessionTTL),
		subscriptions: make(map[string]subscription),
		messages:      make([]Message, 0, manager.config.MessageCapacity),
	}
	transport, err := manager.config.TransportFactory(TransportConfig{
		Broker:          manager.config.Broker,
		Username:        manager.config.Username,
		Password:        manager.config.Password,
		ClientID:        debugClientID(sessionID),
		ConnectTimeout:  manager.config.ConnectTimeout,
		ActionTimeout:   manager.config.ActionTimeout,
		PayloadMaxBytes: manager.config.PayloadMaxBytes,
		Hooks: TransportHooks{
			OnConnect: func() {
				manager.handleConnected(sessionID)
			},
			OnConnectionLost: func(connectionErr error) {
				manager.handleConnectionLost(sessionID, connectionErr)
			},
		},
	})
	if err != nil {
		return Snapshot{}, err
	}
	item.transport = transport

	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		transport.Close()
		return Snapshot{}, ErrRuntimeClosed
	}
	existingSessions := make([]*session, 0, 1)
	for existingID, existing := range manager.sessions {
		if existing.scope == scope {
			existingSessions = append(existingSessions, existing)
			delete(manager.sessions, existingID)
		}
	}
	if len(manager.sessions) >= manager.config.MaxSessions || manager.userSessionCountLocked(scope.UserID) >= manager.config.MaxSessionsPerUser {
		manager.mu.Unlock()
		transport.Close()
		for _, existing := range existingSessions {
			closeSessionTransport(existing)
		}
		return Snapshot{}, ErrSessionCapacity
	}
	manager.sessions[sessionID] = item
	manager.mu.Unlock()
	for _, existing := range existingSessions {
		closeSessionTransport(existing)
	}

	if err := transport.Connect(ctx); err != nil {
		manager.removeAndClose(sessionID)
		return Snapshot{}, fmt.Errorf("open mqtt debug connection: %w", err)
	}
	item.mu.Lock()
	item.connected = transport.IsConnected()
	if !item.closed {
		item.expiryTimer = time.AfterFunc(time.Until(item.expiresAt), func() {
			manager.removeAndClose(sessionID)
		})
	}
	item.mu.Unlock()
	manager.appendMessage(sessionID, Message{Direction: "system", Outcome: "session_opened"})
	return manager.snapshot(item, 0, manager.config.MessageCapacity)
}

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

func (manager *Manager) snapshot(item *session, afterSequence int64, limit int) (Snapshot, error) {
	if limit <= 0 || limit > manager.config.MessageCapacity {
		limit = manager.config.MessageCapacity
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.closed {
		return Snapshot{}, ErrSessionNotFound
	}
	messages := selectSessionMessages(item.messages, afterSequence, limit)
	subscriptions := make([]string, 0, len(item.subscriptions))
	subscriptionDetails := make([]SubscriptionSnapshot, 0, len(item.subscriptions))
	for topic, current := range item.subscriptions {
		subscriptions = append(subscriptions, topic)
		detail := SubscriptionSnapshot{Topic: topic, Mode: SubscriptionModeAcceptedUplink}
		if !current.trustedUplink {
			qos := current.qos
			detail.Mode = SubscriptionModeBroker
			detail.QoS = &qos
		}
		subscriptionDetails = append(subscriptionDetails, detail)
	}
	sort.Strings(subscriptions)
	sort.Slice(subscriptionDetails, func(left, right int) bool {
		return subscriptionDetails[left].Topic < subscriptionDetails[right].Topic
	})
	uplinkObserverDroppedMessages := uint64(0)
	if manager.uplinkSource != nil {
		uplinkObserverDroppedMessages = manager.uplinkSource.DroppedMessages()
	}
	return Snapshot{
		SessionID:                     item.id,
		DeviceID:                      item.scope.DeviceID,
		Connected:                     item.connected && item.transport.IsConnected(),
		CreatedAt:                     item.createdAt,
		ExpiresAt:                     item.expiresAt,
		Subscriptions:                 subscriptions,
		SubscriptionDetails:           subscriptionDetails,
		Messages:                      messages,
		LastSequence:                  item.lastSequence,
		DroppedMessages:               item.droppedMessages,
		MessageCapacity:               manager.config.MessageCapacity,
		PayloadMaxBytes:               manager.config.PayloadMaxBytes,
		SubscriptionLimit:             manager.config.MaxSubscriptions,
		UplinkObserverDroppedMessages: uplinkObserverDroppedMessages,
	}, nil
}

func (manager *Manager) Close(_ context.Context, rawScope Scope, sessionID string) error {
	_, _, err := manager.scopedSession(rawScope, sessionID)
	if err != nil {
		return err
	}
	manager.removeAndClose(strings.TrimSpace(sessionID))
	return nil
}

func (manager *Manager) Stop() {
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return
	}
	manager.closed = true
	uplinkStop := manager.uplinkStop
	manager.uplinkStop = nil
	manager.uplinkAvailable = false
	sessions := make([]*session, 0, len(manager.sessions))
	for _, item := range manager.sessions {
		sessions = append(sessions, item)
	}
	manager.sessions = make(map[string]*session)
	manager.lastOpenByScope = make(map[Scope]time.Time)
	manager.mu.Unlock()
	if uplinkStop != nil {
		uplinkStop()
	}
	for _, item := range sessions {
		closeSessionTransport(item)
	}
}

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
	if existing, ok := item.subscriptions[topic]; ok && existing.qos == qos {
		item.mu.Unlock()
		return nil
	}
	if _, exists := item.subscriptions[topic]; !exists && len(item.subscriptions) >= manager.config.MaxSubscriptions {
		item.mu.Unlock()
		return fmt.Errorf("%w: at most %d subscriptions per session", ErrSessionCapacity, manager.config.MaxSubscriptions)
	}
	item.mu.Unlock()
	if trustedUplink {
		if err := manager.ensureTrustedUplinkSource(); err != nil {
			return err
		}
	}
	if !trustedUplink {
		if err := item.transport.Subscribe(topic, qos, manager.incomingHandler(item.id)); err != nil {
			manager.appendMessage(item.id, Message{Direction: "system", Topic: topic, Outcome: "subscribe_failed"})
			return fmt.Errorf("mqtt debug subscribe: %w", err)
		}
	}
	item.mu.Lock()
	if item.closed {
		item.mu.Unlock()
		if !trustedUplink {
			_ = item.transport.Unsubscribe(topic)
		}
		return ErrSessionNotFound
	}
	item.subscriptions[topic] = subscription{topic: topic, qos: qos, trustedUplink: trustedUplink}
	item.mu.Unlock()
	manager.appendMessage(item.id, Message{Direction: "system", Topic: topic, QoS: qos, Outcome: "subscribed", Source: subscriptionSource(trustedUplink)})
	return nil
}

func (manager *Manager) unsubscribe(item *session, rawTopic string) error {
	topic, _, err := authorizeTopic(item.scope, rawTopic, true)
	if err != nil {
		return err
	}
	item.mu.Lock()
	_, exists := item.subscriptions[topic]
	item.mu.Unlock()
	if !exists {
		return nil
	}
	item.mu.Lock()
	current := item.subscriptions[topic]
	item.mu.Unlock()
	if !current.trustedUplink {
		if err := item.transport.Unsubscribe(topic); err != nil {
			manager.appendMessage(item.id, Message{Direction: "system", Topic: topic, Outcome: "unsubscribe_failed"})
			return fmt.Errorf("mqtt debug unsubscribe: %w", err)
		}
	}
	item.mu.Lock()
	delete(item.subscriptions, topic)
	item.mu.Unlock()
	manager.appendMessage(item.id, Message{Direction: "system", Topic: topic, Outcome: "unsubscribed", Source: subscriptionSource(current.trustedUplink)})
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
		manager.appendMessage(item.id, Message{Direction: "outbound", Topic: topic, QoS: qos, Outcome: "publish_failed"})
		return fmt.Errorf("mqtt debug publish: %w", err)
	}
	manager.appendMessage(item.id, Message{Direction: "outbound", Topic: topic, QoS: qos, Payload: payload, Outcome: "published", Source: "broker_publish"})
	return nil
}

func (manager *Manager) incomingHandler(sessionID string) func(IncomingMessage) {
	return func(incoming IncomingMessage) {
		manager.mu.RLock()
		item := manager.sessions[sessionID]
		manager.mu.RUnlock()
		if item == nil {
			return
		}
		manager.appendMessage(sessionID, Message{
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

func (manager *Manager) handleConnected(sessionID string) {
	manager.mu.RLock()
	item := manager.sessions[sessionID]
	manager.mu.RUnlock()
	if item == nil {
		return
	}
	item.commandMu.Lock()
	defer item.commandMu.Unlock()
	item.mu.Lock()
	if item.closed {
		item.mu.Unlock()
		return
	}
	item.connected = true
	subscriptions := make([]subscription, 0, len(item.subscriptions))
	for _, current := range item.subscriptions {
		subscriptions = append(subscriptions, current)
	}
	item.mu.Unlock()
	manager.appendMessage(sessionID, Message{Direction: "system", Outcome: "connected"})
	for _, current := range subscriptions {
		if current.trustedUplink {
			continue
		}
		if err := item.transport.Subscribe(current.topic, current.qos, manager.incomingHandler(sessionID)); err != nil {
			manager.logger.WithError(err).WithField("session_id", sessionID).Warn("restore mqtt debug subscription failed")
			manager.appendMessage(sessionID, Message{Direction: "system", Topic: current.topic, Outcome: "resubscribe_failed"})
		}
	}
}

func (manager *Manager) handleTrustedUplink(incoming TrustedUplinkMessage) {
	manager.mu.RLock()
	items := make([]*session, 0)
	for _, item := range manager.sessions {
		if item.scope.DeviceID == incoming.DeviceID && item.scope.TenantID == incoming.TenantID {
			items = append(items, item)
		}
	}
	manager.mu.RUnlock()
	for _, item := range items {
		item.mu.Lock()
		matched := false
		if !item.closed {
			for _, current := range item.subscriptions {
				if current.trustedUplink && mqttTopicFilterMatches(current.topic, incoming.Topic) {
					matched = true
					break
				}
			}
		}
		item.mu.Unlock()
		if matched {
			payload, truncated := copyBoundedMQTTDebugPayload(incoming.Payload, manager.config.PayloadMaxBytes)
			manager.appendMessage(item.id, Message{
				Direction: "inbound",
				Topic:     incoming.Topic,
				Payload:   string(payload),
				Truncated: truncated,
				Outcome:   "received",
				Source:    SubscriptionModeAcceptedUplink,
			})
		}
	}
}

func subscriptionSource(trustedUplink bool) string {
	if trustedUplink {
		return SubscriptionModeAcceptedUplink
	}
	return SubscriptionModeBroker
}

func (manager *Manager) ensureTrustedUplinkSource() error {
	manager.uplinkStartMu.Lock()
	defer manager.uplinkStartMu.Unlock()
	manager.mu.RLock()
	if manager.closed {
		manager.mu.RUnlock()
		return ErrRuntimeClosed
	}
	if manager.uplinkAvailable {
		manager.mu.RUnlock()
		return nil
	}
	source := manager.uplinkSource
	manager.mu.RUnlock()
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

func (manager *Manager) handleConnectionLost(sessionID string, _ error) {
	manager.mu.RLock()
	item := manager.sessions[sessionID]
	manager.mu.RUnlock()
	if item == nil {
		return
	}
	item.mu.Lock()
	item.connected = false
	item.mu.Unlock()
	manager.appendMessage(sessionID, Message{Direction: "system", Outcome: "connection_lost"})
}

func (manager *Manager) appendMessage(sessionID string, message Message) {
	manager.mu.RLock()
	item := manager.sessions[sessionID]
	manager.mu.RUnlock()
	if item == nil {
		return
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.closed {
		return
	}
	if message.Direction == "inbound" && manager.exceedsInboundCaptureBudget(item, len(message.Payload)) {
		item.droppedMessages++
		return
	}
	item.lastSequence++
	message.Sequence = item.lastSequence
	message.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	if len(message.Payload) > manager.config.PayloadMaxBytes {
		message.Payload = message.Payload[:manager.config.PayloadMaxBytes]
		message.Truncated = true
	}
	if len(item.messages) >= manager.config.MessageCapacity {
		copy(item.messages, item.messages[1:])
		item.messages[len(item.messages)-1] = message
		item.droppedMessages++
		return
	}
	item.messages = append(item.messages, message)
}

func (manager *Manager) exceedsInboundCaptureBudget(item *session, payloadBytes int) bool {
	now := time.Now().UTC()
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

func (manager *Manager) scopedSession(rawScope Scope, rawSessionID string) (Scope, *session, error) {
	scope, err := normalizeScope(rawScope)
	if err != nil {
		return Scope{}, nil, err
	}
	sessionID := strings.TrimSpace(rawSessionID)
	if sessionID == "" {
		return Scope{}, nil, ErrSessionNotFound
	}
	manager.mu.RLock()
	if manager.closed {
		manager.mu.RUnlock()
		return Scope{}, nil, ErrRuntimeClosed
	}
	item := manager.sessions[sessionID]
	manager.mu.RUnlock()
	if item == nil {
		return Scope{}, nil, ErrSessionNotFound
	}
	if item.scope != scope {
		return Scope{}, nil, ErrSessionScope
	}
	return scope, item, nil
}

func (manager *Manager) removeAndClose(sessionID string) {
	manager.mu.Lock()
	item := manager.sessions[sessionID]
	delete(manager.sessions, sessionID)
	manager.mu.Unlock()
	if item != nil {
		closeSessionTransport(item)
	}
}

func closeSessionTransport(item *session) {
	item.commandMu.Lock()
	defer item.commandMu.Unlock()
	item.mu.Lock()
	if item.closed {
		item.mu.Unlock()
		return
	}
	item.closed = true
	timer := item.expiryTimer
	transport := item.transport
	item.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	if transport != nil {
		transport.Close()
	}
}

func (manager *Manager) userSessionCountLocked(userID string) int {
	count := 0
	for _, item := range manager.sessions {
		if item.scope.UserID == userID {
			count++
		}
	}
	return count
}
