package mqttdebug

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// Manager is the production Runtime. Responsibilities are split by file:
//
//   - manager.go          lifecycle: Open, Close, Stop, expiry, connection hooks
//   - manager_index.go    O(1) session lookups (id, scope, device, per-user count)
//   - manager_commands.go Apply: subscribe / unsubscribe / publish + command budget
//   - manager_capture.go  bounded capture, inbound budget, snapshots
//   - manager_uplink.go   trusted accepted-uplink observer fan-out
//
// Lock order: Manager.uplinkStartMu -> Manager.mu; session.commandMu ->
// session.mu. Manager.mu and session.mu are never held together except where
// Manager.mu is taken first and released before any session lock.
type Manager struct {
	config Config
	logger *logrus.Logger
	// uplinkSource is set once in NewManager and never reassigned, so it may be
	// read without mu. The mutable observer state lives in uplinkStop and
	// uplinkAvailable below, which are guarded by mu.
	uplinkSource UplinkSource

	mu              sync.RWMutex
	index           sessionIndex
	lastOpenByScope map[Scope]time.Time
	closed          bool
	uplinkStop      func()
	uplinkAvailable bool

	// uplinkStartMu serialises observer start so concurrent first trusted
	// subscriptions do not start two Bus observers.
	uplinkStartMu sync.Mutex
}

func NewManager(config Config, logger *logrus.Logger) *Manager {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	config = withManagerDefaults(config)
	if config.TransportFactory == nil {
		config.TransportFactory = newPahoTransportFactory(logger)
	}
	return &Manager{
		config:          config,
		logger:          logger,
		uplinkSource:    config.UplinkSource,
		index:           newSessionIndex(),
		lastOpenByScope: make(map[Scope]time.Time),
	}
}

func (manager *Manager) Open(ctx context.Context, rawScope Scope) (Snapshot, error) {
	scope, err := normalizeScope(rawScope)
	if err != nil {
		return Snapshot{}, err
	}

	now := time.Now().UTC()
	if err := manager.reserveOpen(scope, now); err != nil {
		return Snapshot{}, err
	}

	sessionID := uuid.New()
	item := newSession(sessionID, scope, now, manager.config.SessionTTL, manager.config.MessageCapacity)
	transport, err := manager.config.TransportFactory(TransportConfig{
		Broker:          manager.config.Broker,
		Username:        manager.config.Username,
		Password:        manager.config.Password,
		ClientID:        debugClientID(sessionID),
		ConnectTimeout:  manager.config.ConnectTimeout,
		ActionTimeout:   manager.config.ActionTimeout,
		PayloadMaxBytes: manager.config.PayloadMaxBytes,
		Hooks: TransportHooks{
			OnConnect:        func() { manager.handleConnected(item) },
			OnConnectionLost: func(error) { manager.handleConnectionLost(item) },
		},
	})
	if err != nil {
		return Snapshot{}, err
	}
	item.transport = transport

	if err := manager.register(item); err != nil {
		transport.Close()
		return Snapshot{}, err
	}

	if err := transport.Connect(ctx); err != nil {
		manager.removeAndClose(item)
		return Snapshot{}, fmt.Errorf("open mqtt debug connection: %w", err)
	}
	item.mu.Lock()
	item.connected = transport.IsConnected()
	if !item.closed {
		item.expiryTimer = time.AfterFunc(time.Until(item.expiresAt), func() {
			manager.removeAndClose(item)
		})
	}
	item.mu.Unlock()
	manager.appendMessage(item, Message{Direction: "system", Outcome: "session_opened"})
	return manager.snapshot(item, 0, manager.config.MessageCapacity)
}

// reserveOpen enforces the per-scope reopen cooldown and records the attempt.
func (manager *Manager) reserveOpen(scope Scope, now time.Time) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.closed {
		return ErrRuntimeClosed
	}
	if len(manager.lastOpenByScope) > manager.config.MaxSessions*4 {
		for recordedScope, openedAt := range manager.lastOpenByScope {
			if now.Sub(openedAt) >= manager.config.OpenCooldown {
				delete(manager.lastOpenByScope, recordedScope)
			}
		}
	}
	if lastOpen := manager.lastOpenByScope[scope]; !lastOpen.IsZero() && now.Sub(lastOpen) < manager.config.OpenCooldown {
		return fmt.Errorf("%w: wait before reopening the same device session", ErrRateLimited)
	}
	manager.lastOpenByScope[scope] = now
	return nil
}

// register installs item, replacing (and closing) any previous session for the
// same scope. As before, the replaced session is closed even when the new one
// is rejected for capacity: reopening always ends the old debug connection.
func (manager *Manager) register(item *session) error {
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return ErrRuntimeClosed
	}
	previous := manager.index.forScope(item.scope)
	manager.index.remove(previous)
	err := error(nil)
	if manager.index.len() >= manager.config.MaxSessions ||
		manager.index.users(item.scope.UserID) >= manager.config.MaxSessionsPerUser {
		err = ErrSessionCapacity
	} else {
		manager.index.add(item)
	}
	manager.mu.Unlock()
	if previous != nil {
		closeSessionTransport(previous)
	}
	return err
}

func (manager *Manager) Close(_ context.Context, rawScope Scope, sessionID string) error {
	_, item, err := manager.scopedSession(rawScope, sessionID)
	if err != nil {
		return err
	}
	manager.removeAndClose(item)
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
	sessions := manager.index.drain()
	manager.lastOpenByScope = make(map[Scope]time.Time)
	manager.mu.Unlock()
	if uplinkStop != nil {
		uplinkStop()
	}
	for _, item := range sessions {
		closeSessionTransport(item)
	}
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
	item := manager.index.get(sessionID)
	manager.mu.RUnlock()
	if item == nil {
		return Scope{}, nil, ErrSessionNotFound
	}
	if item.scope != scope {
		return Scope{}, nil, ErrSessionScope
	}
	return scope, item, nil
}

// removeAndClose unregisters item (if still registered) and closes it. It is
// keyed by pointer so a stale expiry timer or a late failure path can never
// remove a newer session that reused the same scope.
func (manager *Manager) removeAndClose(item *session) {
	if item == nil {
		return
	}
	manager.mu.Lock()
	manager.index.remove(item)
	manager.mu.Unlock()
	closeSessionTransport(item)
}

// isRegistered reports whether item is still the live session for its id.
// Transport callbacks use it so a replaced session's late callbacks are inert.
func (manager *Manager) isRegistered(item *session) bool {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.index.get(item.id) == item
}

func (manager *Manager) handleConnected(item *session) {
	if !manager.isRegistered(item) {
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
	restore := item.brokerSubscriptionsLocked()
	item.mu.Unlock()
	manager.appendMessage(item, Message{Direction: "system", Outcome: "connected"})
	for _, current := range restore {
		if err := item.transport.Subscribe(current.topic, current.qos, manager.incomingHandler(item)); err != nil {
			manager.logger.WithError(err).WithField("session_id", item.id).Warn("restore mqtt debug subscription failed")
			manager.appendMessage(item, Message{Direction: "system", Topic: current.topic, Outcome: "resubscribe_failed"})
		}
	}
}

func (manager *Manager) handleConnectionLost(item *session) {
	if !manager.isRegistered(item) {
		return
	}
	item.mu.Lock()
	item.connected = false
	item.mu.Unlock()
	manager.appendMessage(item, Message{Direction: "system", Outcome: "connection_lost"})
}
