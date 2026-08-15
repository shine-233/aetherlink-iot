// 文件用途：在设备解绑后撤销该设备仍存活的 MQTT 在线会话。
// 核心逻辑：筛出并终止目标设备的旧连接，在本机处理返回后发布带 broker 身份的 processing ACK。
// 关键注意事项：IterateClient 持有 broker 全局锁，回调内不能访问 Redis 或调用会再次获取该锁的终止接口。
package aetherlink

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
)

type mqttDeviceSessionTerminator interface {
	TerminateDeviceSessions(deviceID string) int
}

type mqttLegacyDeviceSessionTerminator interface {
	TerminateLegacyDeviceSessions(deviceID string) int
}

type mqttSessionRevocationSubscription interface {
	Messages() <-chan string
	Close() error
}

type mqttSessionRevocationSubscribe func() (mqttSessionRevocationSubscription, error)

type mqttSessionRevocationAcknowledge func(mqttSessionRevocationAck) error

const mqttSessionRevocationAckStatusProcessed = "processed"

type mqttSessionRevocationMonitor struct {
	mu           sync.Mutex
	revoker      mqttDeviceSessionTerminator
	subscribe    mqttSessionRevocationSubscribe
	acknowledge  mqttSessionRevocationAcknowledge
	brokerID     string
	subscription mqttSessionRevocationSubscription
	stop         chan struct{}
	done         chan struct{}
	started      bool
}

type mqttDeviceSessionRevoker struct {
	clients           server.ClientService
	deviceIDForClient func(client server.Client) (string, bool)
	forgetClient      func(client server.Client)
}

type mqttOnlineClientSnapshot struct {
	client             server.Client
	deviceStateVersion time.Time
}

type mqttSessionRevocationMessage struct {
	Version   int       `json:"version"`
	EventID   string    `json:"event_id"`
	DeviceID  string    `json:"device_id"`
	RevokedAt time.Time `json:"revoked_at"`
}

type mqttSessionRevocationAck struct {
	Version            int       `json:"version"`
	EventID            string    `json:"event_id"`
	DeviceID           string    `json:"device_id"`
	RevokedAt          time.Time `json:"revoked_at"`
	BrokerID           string    `json:"broker_id"`
	Status             string    `json:"status"`
	ProcessedAt        time.Time `json:"processed_at"`
	TerminatedSessions int       `json:"terminated_sessions"`
}

func newMQTTDeviceSessionRevoker(clients server.ClientService) mqttDeviceSessionRevoker {
	return mqttDeviceSessionRevoker{
		clients:           clients,
		deviceIDForClient: mqttAuthenticatedDeviceForClient,
		forgetClient:      forgetMQTTAuthenticatedClientBinding,
	}
}

func newMQTTSessionRevocationMonitor(
	revoker mqttDeviceSessionTerminator,
	subscribe mqttSessionRevocationSubscribe,
	brokerID string,
	acknowledge mqttSessionRevocationAcknowledge,
) *mqttSessionRevocationMonitor {
	return &mqttSessionRevocationMonitor{
		revoker:     revoker,
		subscribe:   subscribe,
		brokerID:    brokerID,
		acknowledge: acknowledge,
	}
}

func (m *mqttSessionRevocationMonitor) Start() error {
	if m == nil {
		return fmt.Errorf("mqtt session revocation monitor is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	if m.revoker == nil || m.subscribe == nil || m.acknowledge == nil {
		return fmt.Errorf("mqtt session revocation monitor is not configured")
	}
	brokerID, err := normalizeMQTTSessionRevocationBrokerID(m.brokerID)
	if err != nil {
		return err
	}
	m.brokerID = brokerID

	subscription, err := m.subscribe()
	if err != nil {
		return err
	}
	if subscription == nil {
		return fmt.Errorf("mqtt session revocation subscription is nil")
	}

	m.subscription = subscription
	m.stop = make(chan struct{})
	m.done = make(chan struct{})
	m.started = true
	go m.run(subscription.Messages(), m.stop, m.done)
	return nil
}

func (m *mqttSessionRevocationMonitor) run(messages <-chan string, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-stop:
			return
		case payload, ok := <-messages:
			if !ok {
				return
			}
			event, valid := parseMQTTSessionRevocationMessage(payload)
			if !valid {
				continue
			}
			if event.Version == 0 {
				if legacyRevoker, ok := m.revoker.(mqttLegacyDeviceSessionTerminator); ok {
					// Plain IDs are retained for rolling-reload compatibility, but
					// may only clear legacy zero-version bindings. Versioned
					// sessions must wait for a revoked_at cutoff.
					legacyRevoker.TerminateLegacyDeviceSessions(event.DeviceID)
				}
				continue
			}
			terminatedSessions := 0
			if versionedRevoker, ok := m.revoker.(interface {
				TerminateDeviceSessionsBefore(string, time.Time) int
			}); ok && !event.RevokedAt.IsZero() {
				terminatedSessions = versionedRevoker.TerminateDeviceSessionsBefore(event.DeviceID, event.RevokedAt)
			} else {
				terminatedSessions = m.revoker.TerminateDeviceSessions(event.DeviceID)
			}
			ack := mqttSessionRevocationAck{
				Version:            1,
				EventID:            event.EventID,
				DeviceID:           event.DeviceID,
				RevokedAt:          event.RevokedAt.UTC(),
				BrokerID:           m.brokerID,
				Status:             mqttSessionRevocationAckStatusProcessed,
				ProcessedAt:        time.Now().UTC(),
				TerminatedSessions: terminatedSessions,
			}
			if err := m.acknowledge(ack); err != nil && Log != nil {
				Log.Warn(
					"mqtt session revocation acknowledgement failed; waiting for redelivery",
					zap.String("event_id", event.EventID),
					zap.String("device_id", event.DeviceID),
					zap.String("broker_id", m.brokerID),
					zap.Error(err),
				)
			}
		}
	}
}

func parseMQTTSessionRevocationMessage(payload string) (mqttSessionRevocationMessage, bool) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return mqttSessionRevocationMessage{}, false
	}
	if !strings.HasPrefix(payload, "{") {
		return mqttSessionRevocationMessage{Version: 0, DeviceID: payload}, true
	}
	var event mqttSessionRevocationMessage
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return mqttSessionRevocationMessage{}, false
	}
	event.DeviceID = strings.TrimSpace(event.DeviceID)
	event.EventID = strings.TrimSpace(event.EventID)
	if event.Version != 1 || event.EventID == "" || event.DeviceID == "" || event.RevokedAt.IsZero() {
		return mqttSessionRevocationMessage{}, false
	}
	return event, true
}

func (m *mqttSessionRevocationMonitor) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	subscription := m.subscription
	stop := m.stop
	done := m.done
	m.subscription = nil
	m.stop = nil
	m.done = nil
	m.started = false
	close(stop)
	m.mu.Unlock()

	err := subscription.Close()
	<-done
	return err
}

// TerminateDeviceSessions closes every online MQTT client currently mapped to deviceID.
// It returns the number of sessions for which termination was requested.
func (r mqttDeviceSessionRevoker) TerminateDeviceSessions(deviceID string) int {
	return r.terminateDeviceSessions(deviceID, time.Time{}, false)
}

// TerminateLegacyDeviceSessions is a compatibility-only path for old plain
// device-ID Redis messages. It can clear only zero-version bindings created by
// an older plugin instance; it must never terminate a newer authenticated
// generation just because the legacy payload has no cutoff timestamp.
func (r mqttDeviceSessionRevoker) TerminateLegacyDeviceSessions(deviceID string) int {
	return r.terminateDeviceSessions(deviceID, time.Time{}, true)
}

// TerminateDeviceSessionsBefore revokes only clients authenticated from the
// device state at or before revokedAt. Durable outbox redelivery therefore
// cannot disconnect a client authenticated after the device was re-bound.
func (r mqttDeviceSessionRevoker) TerminateDeviceSessionsBefore(deviceID string, revokedAt time.Time) int {
	return r.terminateDeviceSessions(deviceID, revokedAt, false)
}

func (r mqttDeviceSessionRevoker) terminateDeviceSessions(deviceID string, revokedAt time.Time, legacyOnly bool) int {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || r.clients == nil || r.deviceIDForClient == nil {
		return 0
	}

	clients := make([]mqttOnlineClientSnapshot, 0, 1)
	r.clients.IterateClient(func(client server.Client) bool {
		if client == nil {
			return true
		}
		options := client.ClientOptions()
		if options == nil {
			return true
		}
		clientID := strings.TrimSpace(options.ClientID)
		if clientID == "" {
			return true
		}
		binding, ok := mqttAuthenticatedBindingForClient(client)
		if !ok && r.deviceIDForClient != nil {
			mappedDeviceID, mapped := r.deviceIDForClient(client)
			binding = mqttAuthenticatedClientBinding{deviceID: strings.TrimSpace(mappedDeviceID)}
			ok = mapped
		}
		if ok && binding.deviceID == deviceID &&
			(!legacyOnly || binding.deviceStateVersion.IsZero()) &&
			(revokedAt.IsZero() || binding.deviceStateVersion.IsZero() || !binding.deviceStateVersion.After(revokedAt)) {
			clients = append(clients, mqttOnlineClientSnapshot{
				client:             client,
				deviceStateVersion: binding.deviceStateVersion,
			})
		}
		return true
	})

	terminated := 0
	for _, onlineClient := range clients {
		if !r.clients.TerminateClientIfCurrent(onlineClient.client) {
			continue
		}
		if r.forgetClient != nil {
			r.forgetClient(onlineClient.client)
		}
		terminated++
	}
	return terminated
}
