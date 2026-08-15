// Package mqttdebug owns short-lived, device-scoped MQTT debugging sessions.
//
// The module hides broker connections, subscription lifecycle, topic policy,
// bounded message capture and expiry behind a small session interface. It must
// never expose the platform MQTT credential or reuse the production adapter
// client because a debug subscription could replace a production handler.
package mqttdebug

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRuntimeClosed   = errors.New("mqtt debug runtime is closed")
	ErrSessionNotFound = errors.New("mqtt debug session not found")
	ErrSessionScope    = errors.New("mqtt debug session scope mismatch")
	ErrSessionCapacity = errors.New("mqtt debug session capacity reached")
	ErrInvalidCommand  = errors.New("invalid mqtt debug command")
	ErrInvalidTopic    = errors.New("invalid mqtt debug topic")
	ErrTopicDenied     = errors.New("mqtt debug topic is outside the device scope")
	ErrNotConnected    = errors.New("mqtt debug session is not connected")
	ErrRateLimited     = errors.New("mqtt debug rate limit exceeded")
)

const (
	ActionSubscribe   = "subscribe"
	ActionUnsubscribe = "unsubscribe"
	ActionPublish     = "publish"

	SubscriptionModeBroker         = "broker_subscription"
	SubscriptionModeAcceptedUplink = "accepted_application_uplink_observer"
)

type Scope struct {
	TenantID     string
	UserID       string
	DeviceID     string
	DeviceNumber string
}

type Command struct {
	Action  string
	Topic   string
	QoS     byte
	Payload string
}

type Message struct {
	Sequence  int64  `json:"sequence"`
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	Topic     string `json:"topic,omitempty"`
	QoS       byte   `json:"qos,omitempty"`
	Retained  bool   `json:"retained,omitempty"`
	Duplicate bool   `json:"duplicate,omitempty"`
	Payload   string `json:"payload,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	Source    string `json:"source,omitempty"`
}

type SubscriptionSnapshot struct {
	Topic string `json:"topic"`
	Mode  string `json:"mode"`
	QoS   *byte  `json:"qos,omitempty"`
}

type Snapshot struct {
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id"`
	// Connected is the isolated debug client's broker connection state.
	Connected bool `json:"connected"`
	// PlatformDeviceOnline is the latest platform-recorded device online state.
	PlatformDeviceOnline          bool                   `json:"platform_device_online"`
	CreatedAt                     time.Time              `json:"created_at"`
	ExpiresAt                     time.Time              `json:"expires_at"`
	Subscriptions                 []string               `json:"subscriptions"`
	SubscriptionDetails           []SubscriptionSnapshot `json:"subscription_details"`
	Messages                      []Message              `json:"messages"`
	LastSequence                  int64                  `json:"last_sequence"`
	DroppedMessages               int64                  `json:"dropped_messages"`
	MessageCapacity               int                    `json:"message_capacity"`
	PayloadMaxBytes               int                    `json:"payload_max_bytes"`
	SubscriptionLimit             int                    `json:"subscription_limit"`
	UplinkObserverDroppedMessages uint64                 `json:"uplink_observer_dropped_messages"`
}

// Runtime is the module interface used by the device service and its tests.
// Implementations must isolate sessions, enforce Scope on every operation and
// make Close idempotent at the transport level.
type Runtime interface {
	Open(ctx context.Context, scope Scope) (Snapshot, error)
	Apply(ctx context.Context, scope Scope, sessionID string, command Command) (Snapshot, error)
	Snapshot(ctx context.Context, scope Scope, sessionID string, afterSequence int64, limit int) (Snapshot, error)
	Close(ctx context.Context, scope Scope, sessionID string) error
	Stop()
}
