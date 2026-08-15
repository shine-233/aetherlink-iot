package mqttdebug

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

type pahoTransport struct {
	client          mqtt.Client
	connectTimeout  time.Duration
	actionTimeout   time.Duration
	payloadMaxBytes int
	closed          atomic.Bool
}

func newPahoTransportFactory(logger *logrus.Logger) TransportFactory {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return func(config TransportConfig) (Transport, error) {
		if config.Broker == "" || config.ClientID == "" {
			return nil, fmt.Errorf("mqtt debug broker and client id are required")
		}
		transport := &pahoTransport{
			connectTimeout:  config.ConnectTimeout,
			actionTimeout:   config.ActionTimeout,
			payloadMaxBytes: config.PayloadMaxBytes,
		}
		if transport.actionTimeout <= 0 {
			transport.actionTimeout = 5 * time.Second
		}
		if transport.payloadMaxBytes <= 0 {
			transport.payloadMaxBytes = 4096
		}
		opts := mqtt.NewClientOptions()
		opts.AddBroker(config.Broker)
		opts.SetUsername(config.Username)
		opts.SetPassword(config.Password)
		opts.SetClientID(config.ClientID)
		opts.SetCleanSession(true)
		opts.SetResumeSubs(false)
		opts.SetAutoReconnect(true)
		opts.SetConnectRetry(false)
		// Debug handlers only append to a bounded in-memory log. Serial callback
		// dispatch prevents one goroutine per inbound message from amplifying a
		// noisy device into a process-level spike.
		opts.SetOrderMatters(true)
		opts.SetConnectTimeout(config.ConnectTimeout)
		opts.SetWriteTimeout(transport.actionTimeout)
		opts.SetMaxReconnectInterval(30 * time.Second)
		opts.SetOnConnectHandler(func(client mqtt.Client) {
			if transport.closed.Load() {
				client.Disconnect(0)
				return
			}
			if config.Hooks.OnConnect != nil {
				config.Hooks.OnConnect()
			}
		})
		opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			if transport.closed.Load() {
				return
			}
			logger.WithError(err).Warn("isolated mqtt debug connection lost")
			if config.Hooks.OnConnectionLost != nil {
				config.Hooks.OnConnectionLost(err)
			}
		})
		transport.client = mqtt.NewClient(opts)
		return transport, nil
	}
}

func (transport *pahoTransport) Connect(ctx context.Context) error {
	if transport == nil || transport.client == nil || transport.closed.Load() {
		return ErrRuntimeClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	token := transport.client.Connect()
	timer := time.NewTimer(transport.connectWaitTimeout())
	defer timer.Stop()
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("mqtt debug connect timed out")
	}
}

func (transport *pahoTransport) IsConnected() bool {
	return transport != nil && transport.client != nil && !transport.closed.Load() && transport.client.IsConnectionOpen()
}

func (transport *pahoTransport) Subscribe(topic string, qos byte, handler func(IncomingMessage)) error {
	if !transport.IsConnected() {
		return ErrNotConnected
	}
	token := transport.client.Subscribe(topic, qos, func(_ mqtt.Client, message mqtt.Message) {
		if handler == nil {
			return
		}
		payload, truncated := copyBoundedMQTTDebugPayload(message.Payload(), transport.payloadMaxBytes)
		handler(IncomingMessage{
			Topic:     message.Topic(),
			Payload:   payload,
			QoS:       message.Qos(),
			Retained:  message.Retained(),
			Duplicate: message.Duplicate(),
			Truncated: truncated,
		})
	})
	if !token.WaitTimeout(transport.timeout()) {
		transport.Close()
		return fmt.Errorf("mqtt debug subscribe timed out")
	}
	return token.Error()
}

func copyBoundedMQTTDebugPayload(payload []byte, limit int) ([]byte, bool) {
	if limit <= 0 || len(payload) <= limit {
		return append([]byte(nil), payload...), false
	}
	return append([]byte(nil), payload[:limit]...), true
}

func (transport *pahoTransport) Unsubscribe(topic string) error {
	if !transport.IsConnected() {
		return ErrNotConnected
	}
	token := transport.client.Unsubscribe(topic)
	if !token.WaitTimeout(transport.timeout()) {
		transport.Close()
		return fmt.Errorf("mqtt debug unsubscribe timed out")
	}
	return token.Error()
}

func (transport *pahoTransport) Publish(topic string, qos byte, payload []byte) error {
	if !transport.IsConnected() {
		return ErrNotConnected
	}
	token := transport.client.Publish(topic, qos, false, payload)
	if !token.WaitTimeout(transport.timeout()) {
		transport.Close()
		return fmt.Errorf("mqtt debug publish timed out")
	}
	return token.Error()
}

func (transport *pahoTransport) Close() {
	if transport == nil || !transport.closed.CompareAndSwap(false, true) {
		return
	}
	// Disconnect must also run while Paho is connecting or reconnecting. Its
	// status transition is what stops the reconnect loop; gating this call on
	// IsConnected would leave an expired debug session retrying in the
	// background until the broker happened to come back.
	if transport.client != nil {
		transport.client.Disconnect(250)
	}
}

func (transport *pahoTransport) timeout() time.Duration {
	if transport.actionTimeout <= 0 {
		return 5 * time.Second
	}
	return transport.actionTimeout
}

func (transport *pahoTransport) connectWaitTimeout() time.Duration {
	if transport.connectTimeout <= 0 {
		return 5 * time.Second
	}
	return transport.connectTimeout
}
