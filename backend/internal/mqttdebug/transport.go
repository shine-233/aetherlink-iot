package mqttdebug

import (
	"context"
	"time"
)

type IncomingMessage struct {
	Topic     string
	Payload   []byte
	QoS       byte
	Retained  bool
	Duplicate bool
	Truncated bool
}

type TransportHooks struct {
	OnConnect        func()
	OnConnectionLost func(error)
}

// Transport is an internal seam. Production uses the Paho adapter and module
// tests use an in-memory adapter; callers only see Runtime.
type Transport interface {
	Connect(context.Context) error
	IsConnected() bool
	Subscribe(topic string, qos byte, handler func(IncomingMessage)) error
	Unsubscribe(topic string) error
	Publish(topic string, qos byte, payload []byte) error
	Close()
}

type TransportConfig struct {
	Broker          string
	Username        string
	Password        string
	ClientID        string
	ConnectTimeout  time.Duration
	ActionTimeout   time.Duration
	PayloadMaxBytes int
	Hooks           TransportHooks
}

type TransportFactory func(TransportConfig) (Transport, error)

// TrustedUplinkMessage has already passed the production MQTT adapter's
// protocol parsing and device/tenant lookup. Shared uplink debug logs must use
// this identity instead of trusting a device-supplied JSON field.
type TrustedUplinkMessage struct {
	DeviceID string
	TenantID string
	Type     string
	Topic    string
	Payload  []byte
}

// UplinkSource is an internal fan-out seam over accepted production uplinks.
// Production uses the uplink Bus observer adapter and tests can use an
// in-memory source. Start must return an idempotent stop function.
type UplinkSource interface {
	Start(func(TrustedUplinkMessage)) (func(), error)
	DroppedMessages() uint64
}
