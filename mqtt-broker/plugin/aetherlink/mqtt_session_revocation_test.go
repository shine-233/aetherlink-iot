// 文件用途：验证设备解绑后的 MQTT 在线会话撤销行为。
// 核心逻辑：通过 broker ClientService seam 证明只终止属于目标设备的客户端。
// 关键注意事项：IterateClient 持有 broker 锁，测试必须锁定“先收集、后终止”的调用顺序。
package aetherlink

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"github.com/alicebob/miniredis/v2"
	"github.com/golang/mock/gomock"
	"gopkg.in/redis.v5"
)

type recordingMQTTDeviceSessionRevoker struct {
	deviceIDs          chan string
	terminatedSessions int
}

func (r *recordingMQTTDeviceSessionRevoker) TerminateDeviceSessions(deviceID string) int {
	r.deviceIDs <- deviceID
	return r.terminatedSessions
}

func (r *recordingMQTTDeviceSessionRevoker) TerminateLegacyDeviceSessions(deviceID string) int {
	r.deviceIDs <- deviceID
	return r.terminatedSessions
}

func (r *recordingMQTTDeviceSessionRevoker) TerminateDeviceSessionsBefore(deviceID string, _ time.Time) int {
	r.deviceIDs <- deviceID
	return r.terminatedSessions
}

type fakeMQTTSessionRevocationSubscription struct {
	messages chan string
	mu       sync.Mutex
	closed   bool
}

func (s *fakeMQTTSessionRevocationSubscription) Messages() <-chan string {
	return s.messages
}

func (s *fakeMQTTSessionRevocationSubscription) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *fakeMQTTSessionRevocationSubscription) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func TestMQTTDeviceSessionRevokerTerminatesOnlyMatchingDeviceClients(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientService := server.NewMockClientService(ctrl)
	matchingClient := server.NewMockClient(ctrl)
	otherClient := server.NewMockClient(ctrl)
	unknownClient := server.NewMockClient(ctrl)

	matchingClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-matching"})
	otherClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-other"})
	unknownClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-unknown"})

	clientService.EXPECT().IterateClient(gomock.Any()).Do(func(iterate server.ClientIterateFn) {
		iterate(matchingClient)
		iterate(otherClient)
		iterate(unknownClient)
	})
	clientService.EXPECT().TerminateClientIfCurrent(matchingClient).Return(true)

	forgotten := make([]string, 0, 1)
	revoker := mqttDeviceSessionRevoker{
		clients: clientService,
		deviceIDForClient: func(client server.Client) (string, bool) {
			switch client {
			case matchingClient:
				return "device-1", true
			case otherClient:
				return "device-2", true
			default:
				return "", false
			}
		},
		forgetClient: func(client server.Client) {
			if client != matchingClient {
				t.Fatalf("forgotten client = %v, want matching client", client)
			}
			forgotten = append(forgotten, "client-matching")
		},
	}

	if got := revoker.TerminateDeviceSessions(" device-1 "); got != 1 {
		t.Fatalf("TerminateDeviceSessions() = %d, want 1", got)
	}
	if len(forgotten) != 1 || forgotten[0] != "client-matching" {
		t.Fatalf("forgotten clients = %#v, want [client-matching]", forgotten)
	}
}

func TestMQTTDeviceSessionRevokerDoesNotTerminateReusedClientID(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientService := server.NewMockClientService(ctrl)
	originalClient := server.NewMockClient(ctrl)

	originalClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-reused"})
	clientService.EXPECT().IterateClient(gomock.Any()).Do(func(iterate server.ClientIterateFn) {
		iterate(originalClient)
	})
	clientService.EXPECT().TerminateClientIfCurrent(originalClient).Return(false)

	revoker := mqttDeviceSessionRevoker{
		clients: clientService,
		deviceIDForClient: func(client server.Client) (string, bool) {
			return "device-1", client == originalClient
		},
		forgetClient: func(server.Client) {
			t.Fatal("reused client ID mapping must not be forgotten")
		},
	}

	if got := revoker.TerminateDeviceSessions("device-1"); got != 0 {
		t.Fatalf("TerminateDeviceSessions() = %d, want 0", got)
	}
}

func TestMQTTDeviceSessionRevokerKeepsSessionsAuthenticatedAfterRevocationCutoff(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientService := server.NewMockClientService(ctrl)
	oldClient := server.NewMockClient(ctrl)
	newClient := server.NewMockClient(ctrl)
	cutoff := time.Now().UTC()

	oldClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-old-generation"})
	newClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-new-generation"})
	clientService.EXPECT().IterateClient(gomock.Any()).Do(func(iterate server.ClientIterateFn) {
		iterate(oldClient)
		iterate(newClient)
	})
	clientService.EXPECT().TerminateClientIfCurrent(oldClient).Return(true)

	mqttAuthenticatedClientBindings.Store(oldClient, mqttAuthenticatedClientBinding{
		deviceID:           "device-1",
		deviceStateVersion: cutoff.Add(-time.Second),
	})
	mqttAuthenticatedClientBindings.Store(newClient, mqttAuthenticatedClientBinding{
		deviceID:           "device-1",
		deviceStateVersion: cutoff.Add(time.Second),
	})
	t.Cleanup(func() {
		mqttAuthenticatedClientBindings.Delete(oldClient)
		mqttAuthenticatedClientBindings.Delete(newClient)
	})

	revoker := mqttDeviceSessionRevoker{
		clients:           clientService,
		deviceIDForClient: mqttAuthenticatedDeviceForClient,
		forgetClient:      forgetMQTTAuthenticatedClientBinding,
	}
	if got := revoker.TerminateDeviceSessionsBefore("device-1", cutoff); got != 1 {
		t.Fatalf("TerminateDeviceSessionsBefore() = %d, want only the old session", got)
	}
	if _, ok := mqttAuthenticatedBindingForClient(oldClient); ok {
		t.Fatal("old session binding should be forgotten after termination")
	}
	if binding, ok := mqttAuthenticatedBindingForClient(newClient); !ok || binding.deviceStateVersion.Before(cutoff) {
		t.Fatalf("new session binding was removed or regressed: %#v, ok=%v", binding, ok)
	}
}

func TestMQTTDeviceSessionRevokerLegacyMessageKeepsVersionedSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	clientService := server.NewMockClientService(ctrl)
	legacyClient := server.NewMockClient(ctrl)
	versionedClient := server.NewMockClient(ctrl)

	legacyClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-legacy-binding"})
	versionedClient.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "client-versioned-binding"})
	clientService.EXPECT().IterateClient(gomock.Any()).Do(func(iterate server.ClientIterateFn) {
		iterate(legacyClient)
		iterate(versionedClient)
	})
	clientService.EXPECT().TerminateClientIfCurrent(legacyClient).Return(true)

	mqttAuthenticatedClientBindings.Store(legacyClient, "device-1")
	mqttAuthenticatedClientBindings.Store(versionedClient, mqttAuthenticatedClientBinding{
		deviceID:           "device-1",
		deviceStateVersion: time.Now().UTC(),
	})
	t.Cleanup(func() {
		mqttAuthenticatedClientBindings.Delete(legacyClient)
		mqttAuthenticatedClientBindings.Delete(versionedClient)
	})

	revoker := mqttDeviceSessionRevoker{
		clients:           clientService,
		deviceIDForClient: mqttAuthenticatedDeviceForClient,
		forgetClient:      forgetMQTTAuthenticatedClientBinding,
	}
	if got := revoker.TerminateLegacyDeviceSessions("device-1"); got != 1 {
		t.Fatalf("TerminateLegacyDeviceSessions() = %d, want only the zero-version session", got)
	}
	if _, ok := mqttAuthenticatedBindingForClient(legacyClient); ok {
		t.Fatal("legacy session binding should be forgotten after termination")
	}
	if binding, ok := mqttAuthenticatedBindingForClient(versionedClient); !ok || binding.deviceStateVersion.IsZero() {
		t.Fatalf("versioned session binding was removed or regressed: %#v, ok=%v", binding, ok)
	}
}

func TestMQTTSessionRevocationMonitorConsumesDeviceEventsAndClosesSubscription(t *testing.T) {
	subscription := &fakeMQTTSessionRevocationSubscription{messages: make(chan string, 1)}
	revoker := &recordingMQTTDeviceSessionRevoker{deviceIDs: make(chan string, 1), terminatedSessions: 1}
	acknowledgements := make(chan mqttSessionRevocationAck, 1)
	monitor := newMQTTSessionRevocationMonitor(revoker, func() (mqttSessionRevocationSubscription, error) {
		return subscription, nil
	}, "broker-a", func(ack mqttSessionRevocationAck) error {
		acknowledgements <- ack
		return nil
	})

	if err := monitor.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	subscription.messages <- " device-1 "

	select {
	case deviceID := <-revoker.deviceIDs:
		if deviceID != "device-1" {
			t.Fatalf("revoked device = %q, want device-1", deviceID)
		}
	case <-time.After(time.Second):
		t.Fatal("revocation event was not consumed")
	}
	select {
	case ack := <-acknowledgements:
		t.Fatalf("legacy revocation unexpectedly produced acknowledgement: %#v", ack)
	default:
	}

	if err := monitor.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !subscription.isClosed() {
		t.Fatal("Close() did not close the revocation subscription")
	}
}

func TestMQTTSessionRevocationMonitorAcknowledgesV1AfterProcessingZeroSessions(t *testing.T) {
	subscription := &fakeMQTTSessionRevocationSubscription{messages: make(chan string, 1)}
	revoker := &recordingMQTTDeviceSessionRevoker{deviceIDs: make(chan string, 1), terminatedSessions: 0}
	acknowledgements := make(chan mqttSessionRevocationAck, 1)
	revokedAt := time.Date(2026, time.July, 19, 8, 30, 0, 0, time.UTC)
	monitor := newMQTTSessionRevocationMonitor(revoker, func() (mqttSessionRevocationSubscription, error) {
		return subscription, nil
	}, " broker-a ", func(ack mqttSessionRevocationAck) error {
		acknowledgements <- ack
		return nil
	})

	if err := monitor.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = monitor.Close() })
	payload, err := json.Marshal(mqttSessionRevocationMessage{
		Version:   1,
		EventID:   "event-1",
		DeviceID:  "device-1",
		RevokedAt: revokedAt,
	})
	if err != nil {
		t.Fatalf("marshal revocation message: %v", err)
	}
	subscription.messages <- string(payload)

	select {
	case ack := <-acknowledgements:
		select {
		case deviceID := <-revoker.deviceIDs:
			if deviceID != "device-1" {
				t.Fatalf("revoked device = %q, want device-1", deviceID)
			}
		default:
			t.Fatal("acknowledgement was emitted before session termination returned")
		}
		if ack.Version != 1 || ack.EventID != "event-1" || ack.DeviceID != "device-1" {
			t.Fatalf("acknowledgement identity = %#v", ack)
		}
		if ack.BrokerID != "broker-a" || ack.Status != mqttSessionRevocationAckStatusProcessed {
			t.Fatalf("acknowledgement broker/status = %#v", ack)
		}
		if !ack.RevokedAt.Equal(revokedAt) || ack.ProcessedAt.IsZero() || ack.TerminatedSessions != 0 {
			t.Fatalf("acknowledgement processing fields = %#v", ack)
		}
	case <-time.After(time.Second):
		t.Fatal("v1 revocation acknowledgement was not emitted")
	}
}

func TestMQTTSessionRevocationMonitorDoesNotRetryFailedAcknowledgementLocally(t *testing.T) {
	subscription := &fakeMQTTSessionRevocationSubscription{messages: make(chan string, 1)}
	revoker := &recordingMQTTDeviceSessionRevoker{deviceIDs: make(chan string, 1), terminatedSessions: 2}
	ackAttempts := make(chan mqttSessionRevocationAck, 2)
	monitor := newMQTTSessionRevocationMonitor(revoker, func() (mqttSessionRevocationSubscription, error) {
		return subscription, nil
	}, "broker-a", func(ack mqttSessionRevocationAck) error {
		ackAttempts <- ack
		return errors.New("ack unavailable")
	})

	if err := monitor.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = monitor.Close() })
	payload, err := json.Marshal(mqttSessionRevocationMessage{
		Version:   1,
		EventID:   "event-retry",
		DeviceID:  "device-retry",
		RevokedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("marshal revocation message: %v", err)
	}
	subscription.messages <- string(payload)

	select {
	case <-ackAttempts:
	case <-time.After(time.Second):
		t.Fatal("failed acknowledgement was not attempted")
	}
	select {
	case ack := <-ackAttempts:
		t.Fatalf("failed acknowledgement retried locally instead of waiting for redelivery: %#v", ack)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestParseMQTTSessionRevocationMessageRequiresCompleteV1Envelope(t *testing.T) {
	valid := `{"version":1,"event_id":"event-1","device_id":"device-1","revoked_at":"2026-07-19T08:30:00Z"}`
	if event, ok := parseMQTTSessionRevocationMessage(valid); !ok || event.EventID != "event-1" {
		t.Fatalf("valid v1 envelope = %#v, ok=%v", event, ok)
	}

	invalid := []string{
		`{"version":1,"device_id":"device-1","revoked_at":"2026-07-19T08:30:00Z"}`,
		`{"version":1,"event_id":"event-1","revoked_at":"2026-07-19T08:30:00Z"}`,
		`{"version":1,"event_id":"event-1","device_id":"device-1"}`,
	}
	for _, payload := range invalid {
		if event, ok := parseMQTTSessionRevocationMessage(payload); ok {
			t.Fatalf("incomplete v1 envelope %s parsed as %#v", payload, event)
		}
	}
}

func TestNormalizeMQTTSessionRevocationBrokerID(t *testing.T) {
	brokerID, err := normalizeMQTTSessionRevocationBrokerID(" broker-01.eu:primary ")
	if err != nil || brokerID != "broker-01.eu:primary" {
		t.Fatalf("normalized broker ID = %q, err=%v", brokerID, err)
	}

	for _, raw := range []string{"", "   ", "broker/id", "broker id", strings.Repeat("a", 129)} {
		if brokerID, err := normalizeMQTTSessionRevocationBrokerID(raw); err == nil {
			t.Fatalf("invalid broker ID %q normalized to %q", raw, brokerID)
		}
	}
}

func TestRedisMQTTSessionRevocationSubscriptionForwardsPublishedDeviceID(t *testing.T) {
	server := miniredis.RunT(t)
	previousRedisCache := redisCache
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	redisCache = client
	t.Cleanup(func() {
		_ = client.Close()
		redisCache = previousRedisCache
	})

	subscription, err := subscribeRedisMQTTSessionRevocations()
	if err != nil {
		t.Fatalf("subscribeRedisMQTTSessionRevocations() error = %v", err)
	}
	t.Cleanup(func() { _ = subscription.Close() })

	if err := client.Publish(mqttDeviceSessionRevocationChannel, " device-1 ").Err(); err != nil {
		t.Fatalf("publish revocation: %v", err)
	}

	select {
	case deviceID := <-subscription.Messages():
		if deviceID != " device-1 " {
			t.Fatalf("forwarded device ID = %q, want raw published payload", deviceID)
		}
	case <-time.After(time.Second):
		t.Fatal("published revocation was not forwarded")
	}
}

func TestPublishRedisMQTTSessionRevocationAckUsesDedicatedChannelAndPayload(t *testing.T) {
	server := miniredis.RunT(t)
	previousRedisCache := redisCache
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	redisCache = client
	t.Cleanup(func() {
		_ = client.Close()
		redisCache = previousRedisCache
	})

	pubsub, err := client.Subscribe(mqttDeviceSessionRevocationAckChannel)
	if err != nil {
		t.Fatalf("subscribe acknowledgement channel: %v", err)
	}
	t.Cleanup(func() { _ = pubsub.Close() })
	if _, err := pubsub.ReceiveTimeout(time.Second); err != nil {
		t.Fatalf("confirm acknowledgement subscription: %v", err)
	}

	revokedAt := time.Date(2026, time.July, 19, 8, 30, 0, 0, time.UTC)
	processedAt := revokedAt.Add(time.Second)
	want := mqttSessionRevocationAck{
		Version:            1,
		EventID:            "event-ack",
		DeviceID:           "device-ack",
		RevokedAt:          revokedAt,
		BrokerID:           "broker-a",
		Status:             mqttSessionRevocationAckStatusProcessed,
		ProcessedAt:        processedAt,
		TerminatedSessions: 0,
	}
	if err := publishRedisMQTTSessionRevocationAck(want); err != nil {
		t.Fatalf("publish acknowledgement: %v", err)
	}

	message, err := pubsub.ReceiveMessage()
	if err != nil {
		t.Fatalf("receive acknowledgement: %v", err)
	}
	var got mqttSessionRevocationAck
	if err := json.Unmarshal([]byte(message.Payload), &got); err != nil {
		t.Fatalf("decode acknowledgement: %v", err)
	}
	if got != want {
		t.Fatalf("acknowledgement = %#v, want %#v", got, want)
	}
}
