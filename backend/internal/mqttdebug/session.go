package mqttdebug

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type subscription struct {
	topic         string
	qos           byte
	trustedUplink bool
}

// subscriptionView is an immutable, pre-sorted projection of a session's
// subscriptions. It is rebuilt (under session.mu) only when subscriptions
// change and published through an atomic pointer, so:
//
//   - the trusted-uplink fan-out path matches filters without taking any
//     session lock or iterating a map;
//   - snapshots stop re-sorting and re-allocating the subscription list on
//     every poll.
//
// Never mutate a published view; build a new one instead.
type subscriptionView struct {
	topics         []string               // sorted
	details        []SubscriptionSnapshot // sorted by Topic, QoS pointers owned by the view
	trustedFilters []compiledTopicFilter  // trusted-uplink filters only, sorted
}

// compiledTopicFilter is a pre-split MQTT filter. Matching walks the topic
// with strings.Cut, so the per-uplink hot path allocates nothing (the generic
// mqttTopicFilterMatches splits both strings on every call). Semantics are
// identical: same segment count, "+" matches one non-empty segment, everything
// else matches literally.
type compiledTopicFilter struct {
	raw      string
	segments []string
}

func compileTopicFilter(filter string) compiledTopicFilter {
	return compiledTopicFilter{raw: filter, segments: strings.Split(filter, "/")}
}

func (filter compiledTopicFilter) matches(topic string) bool {
	rest := topic
	for index, expected := range filter.segments {
		segment, tail, found := strings.Cut(rest, "/")
		last := index == len(filter.segments)-1
		if found == last { // topic has fewer or more segments than the filter
			return false
		}
		if expected == "+" {
			if segment == "" {
				return false
			}
		} else if expected != segment {
			return false
		}
		rest = tail
	}
	return true
}

var emptySubscriptionView = &subscriptionView{}

func buildSubscriptionView(subscriptions map[string]subscription) *subscriptionView {
	if len(subscriptions) == 0 {
		return emptySubscriptionView
	}
	view := &subscriptionView{
		topics:  make([]string, 0, len(subscriptions)),
		details: make([]SubscriptionSnapshot, 0, len(subscriptions)),
	}
	for topic := range subscriptions {
		view.topics = append(view.topics, topic)
	}
	sort.Strings(view.topics)
	for _, topic := range view.topics {
		current := subscriptions[topic]
		detail := SubscriptionSnapshot{Topic: topic, Mode: SubscriptionModeAcceptedUplink}
		if current.trustedUplink {
			view.trustedFilters = append(view.trustedFilters, compileTopicFilter(topic))
		} else {
			qos := current.qos
			detail.Mode = SubscriptionModeBroker
			detail.QoS = &qos
		}
		view.details = append(view.details, detail)
	}
	return view
}

// matchesTrusted reports whether any trusted-uplink filter matches topic.
func (view *subscriptionView) matchesTrusted(topic string) bool {
	for _, filter := range view.trustedFilters {
		if filter.matches(topic) {
			return true
		}
	}
	return false
}

// snapshotCopies returns caller-owned copies so JSON encoders or callers that
// mutate the Snapshot cannot corrupt the shared view.
func (view *subscriptionView) snapshotCopies() ([]string, []SubscriptionSnapshot) {
	topics := append(make([]string, 0, len(view.topics)), view.topics...)
	details := make([]SubscriptionSnapshot, len(view.details))
	for i, detail := range view.details {
		if detail.QoS != nil {
			qos := *detail.QoS
			detail.QoS = &qos
		}
		details[i] = detail
	}
	return topics, details
}

// session is one isolated debug connection.
//
// Lock order: commandMu (serialises user commands and reconnect restores)
// before mu (guards every mutable field below). Manager.mu is never acquired
// while holding session.mu.
type session struct {
	commandMu sync.Mutex
	mu        sync.Mutex

	id        string
	scope     Scope
	transport Transport
	createdAt time.Time
	expiresAt time.Time

	connected     bool
	closed        bool
	subscriptions map[string]subscription
	// view mirrors subscriptions; written under mu, read lock-free.
	view atomic.Pointer[subscriptionView]

	messages        messageRing
	lastSequence    int64
	droppedMessages int64

	captureWindowStart    time.Time
	captureWindowMessages int
	captureWindowBytes    int

	commandWindowStart        time.Time
	commandWindowCount        int
	commandWindowPublishBytes int

	snapshotWindowStart time.Time
	snapshotWindowCount int

	expiryTimer *time.Timer
}

func newSession(id string, scope Scope, now time.Time, ttl time.Duration, messageCapacity int) *session {
	item := &session{
		id:            id,
		scope:         scope,
		createdAt:     now,
		expiresAt:     now.Add(ttl),
		subscriptions: make(map[string]subscription),
		messages:      newMessageRing(messageCapacity),
	}
	item.view.Store(emptySubscriptionView)
	return item
}

// setSubscriptionLocked records or replaces a subscription. Caller holds mu.
func (item *session) setSubscriptionLocked(current subscription) {
	item.subscriptions[current.topic] = current
	item.view.Store(buildSubscriptionView(item.subscriptions))
}

// deleteSubscriptionLocked removes a subscription. Caller holds mu.
func (item *session) deleteSubscriptionLocked(topic string) {
	if _, ok := item.subscriptions[topic]; !ok {
		return
	}
	delete(item.subscriptions, topic)
	item.view.Store(buildSubscriptionView(item.subscriptions))
}

// brokerSubscriptionsLocked lists subscriptions that live on the debug
// client's own broker connection (needed for reconnect restore). Caller holds mu.
func (item *session) brokerSubscriptionsLocked() []subscription {
	out := make([]subscription, 0, len(item.subscriptions))
	for _, current := range item.subscriptions {
		if !current.trustedUplink {
			out = append(out, current)
		}
	}
	return out
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
	item.messages.release()
	item.view.Store(emptySubscriptionView)
	item.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	if transport != nil {
		transport.Close()
	}
}
