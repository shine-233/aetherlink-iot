package aetherlink

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
)

type testMqttToken struct {
	waitOK bool
	err    error
}

func (t testMqttToken) Wait() bool                     { return t.waitOK }
func (t testMqttToken) WaitTimeout(time.Duration) bool { return t.waitOK }
func (t testMqttToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (t testMqttToken) Error() error { return t.err }

type publishedMqttMessage struct {
	topic    string
	qos      byte
	retained bool
	payload  []byte
}

type testMqttClient struct {
	mu           sync.Mutex
	connected    bool
	publish      func(string, byte, bool, interface{}) mqtt.Token
	published    []publishedMqttMessage
	disconnected chan struct{}
}

func newTestMqttClient() *testMqttClient {
	return &testMqttClient{connected: true, disconnected: make(chan struct{}, 1)}
}

func (c *testMqttClient) IsConnected() bool      { return c.connected }
func (c *testMqttClient) IsConnectionOpen() bool { return c.connected }
func (c *testMqttClient) Connect() mqtt.Token    { return testMqttToken{waitOK: true} }
func (c *testMqttClient) Disconnect(uint) {
	c.connected = false
	select {
	case c.disconnected <- struct{}{}:
	default:
	}
}
func (c *testMqttClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	if c.publish != nil {
		return c.publish(topic, qos, retained, payload)
	}
	data := append([]byte(nil), payload.([]byte)...)
	c.mu.Lock()
	c.published = append(c.published, publishedMqttMessage{topic: topic, qos: qos, retained: retained, payload: data})
	c.mu.Unlock()
	return testMqttToken{waitOK: true}
}
func (c *testMqttClient) Subscribe(string, byte, mqtt.MessageHandler) mqtt.Token {
	return testMqttToken{waitOK: true}
}
func (c *testMqttClient) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
	return testMqttToken{waitOK: true}
}
func (c *testMqttClient) Unsubscribe(...string) mqtt.Token        { return testMqttToken{waitOK: true} }
func (c *testMqttClient) AddRoute(string, mqtt.MessageHandler)    {}
func (c *testMqttClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

type stubMappedPublisher struct {
	err      error
	topic    string
	qos      byte
	retained bool
	payload  []byte
}

func (s *stubMappedPublisher) SendMessage(topic string, qos byte, retained bool, payload []byte) error {
	s.topic = topic
	s.qos = qos
	s.retained = retained
	s.payload = append([]byte(nil), payload...)
	return s.err
}

type blockingMqttToken struct {
	done <-chan struct{}
	err  error
}

func (t blockingMqttToken) Wait() bool { <-t.done; return true }
func (t blockingMqttToken) WaitTimeout(time.Duration) bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}
func (t blockingMqttToken) Done() <-chan struct{} { return t.done }
func (t blockingMqttToken) Error() error          { return t.err }

type blockingConnectMqttClient struct {
	*testMqttClient
	connectStarted chan struct{}
	connectDone    chan struct{}
}

func (c *blockingConnectMqttClient) Connect() mqtt.Token {
	select {
	case c.connectStarted <- struct{}{}:
	default:
	}
	return blockingMqttToken{done: c.connectDone}
}

func TestBuildInternalMqttClientOptionsUsesRootIdentityAndOrderedDelivery(t *testing.T) {
	viper.Set("mqtt.password", "root-pass")
	viper.Set("mqtt.broker", "127.0.0.1:1883")
	t.Cleanup(viper.Reset)

	opts, addr := buildInternalMqttClientOptions()
	if addr != "127.0.0.1:1883" || opts.Username != "root" || opts.Password != "root-pass" {
		t.Fatalf("unexpected internal mqtt options: addr=%q username=%q", addr, opts.Username)
	}
	if opts.ClientID != "aetherlink-gmqtt-client" || !opts.CleanSession || !opts.AutoReconnect || !opts.Order {
		t.Fatal("internal mqtt client identity or ordered-delivery options are invalid")
	}
}

func TestMqttClientStartsWithNoRuntime(t *testing.T) {
	client := &MqttClient{}
	if client.Client != nil || client.sendCh != nil || client.abortSend != nil || client.IsFlag || client.running {
		t.Fatal("fresh mqtt client should be zero-valued before init")
	}
}

func TestMqttClientSendMessagePreservesMetadataAndCopiesPayload(t *testing.T) {
	fake := newTestMqttClient()
	started := make(chan struct{})
	release := make(chan struct{})
	fake.publish = func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
		close(started)
		<-release
		data := append([]byte(nil), payload.([]byte)...)
		fake.mu.Lock()
		fake.published = append(fake.published, publishedMqttMessage{topic: topic, qos: qos, retained: retained, payload: data})
		fake.mu.Unlock()
		return testMqttToken{waitOK: true}
	}

	client := &MqttClient{}
	if err := client.startForTest(fake, 1); err != nil {
		t.Fatalf("startForTest: %v", err)
	}
	payload := []byte("original")
	errCh := make(chan error, 1)
	go func() { errCh <- client.SendMessage("mapped/up", 2, true, payload) }()
	<-started
	copy(payload, []byte("mutated!"))
	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	fake.mu.Lock()
	got := fake.published[0]
	fake.mu.Unlock()
	if got.topic != "mapped/up" || got.qos != 2 || !got.retained || string(got.payload) != "original" {
		t.Fatalf("published message = %#v", got)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMqttClientPropagatesPublishFailureAndTimeout(t *testing.T) {
	cases := []struct {
		name    string
		token   mqtt.Token
		wantErr string
	}{
		{name: "token error", token: testMqttToken{waitOK: true, err: errors.New("broker rejected")}, wantErr: "broker rejected"},
		{name: "token timeout", token: testMqttToken{waitOK: false}, wantErr: "timeout"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := newTestMqttClient()
			fake.publish = func(string, byte, bool, interface{}) mqtt.Token { return tt.token }
			client := &MqttClient{}
			if err := client.startForTest(fake, 1); err != nil {
				t.Fatalf("startForTest: %v", err)
			}
			err := client.SendData("mapped/up", []byte("payload"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("SendData error = %v, want containing %q", err, tt.wantErr)
			}
			if err := client.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestMqttClientSendMessageReturnsWhenQueueIsFull(t *testing.T) {
	fake := newTestMqttClient()
	block := make(chan struct{})
	started := make(chan struct{})
	fake.publish = func(string, byte, bool, interface{}) mqtt.Token {
		select {
		case <-started:
		default:
			close(started)
		}
		<-block
		return testMqttToken{waitOK: true}
	}
	client := &MqttClient{}
	if err := client.startForTest(fake, 1); err != nil {
		t.Fatalf("startForTest: %v", err)
	}
	client.mu.Lock()
	client.sendEnqueueTimeout = 10 * time.Millisecond
	client.mu.Unlock()
	first := make(chan error, 1)
	go func() { first <- client.SendData("one", []byte("1")) }()
	<-started
	second := make(chan error, 1)
	go func() { second <- client.SendData("two", []byte("2")) }()
	deadline := time.Now().Add(time.Second)
	for len(client.sendCh) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("second publish was not queued")
		}
		time.Sleep(time.Millisecond)
	}

	err := client.SendData("three", []byte("3"))
	if err == nil || !strings.Contains(err.Error(), "queue full") {
		t.Fatalf("SendData error = %v, want queue full", err)
	}
	close(block)
	if err := <-first; err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMqttClientWaitReadyTimesOutBeforeConnection(t *testing.T) {
	fake := newTestMqttClient()
	client := &MqttClient{}
	_, cancel := context.WithCancel(context.Background())
	if err := client.beginRuntime(fake, cancel); err != nil {
		t.Fatalf("beginRuntime: %v", err)
	}
	client.mu.Lock()
	client.readyTimeout = 10 * time.Millisecond
	close(client.connectDone)
	client.mu.Unlock()
	go client.sendWorker(client.sendCh, client.abortSend, client.workerDone)

	err := client.WaitReady(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitReady error = %v, want deadline exceeded", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMqttClientRejectsUninitializedAndClosedClient(t *testing.T) {
	client := &MqttClient{}
	err := client.SendData("devices/status/dev1", []byte("1"))
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("uninitialized SendData error = %v", err)
	}

	fake := newTestMqttClient()
	if err := client.startForTest(fake, 1); err != nil {
		t.Fatalf("startForTest: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	err = client.SendData("devices/status/dev1", []byte("1"))
	if err == nil {
		t.Fatal("closed client accepted publish")
	}
}

func TestMqttInitReturnsWhenCloseCancelsConnectRetry(t *testing.T) {
	fake := &blockingConnectMqttClient{
		testMqttClient: newTestMqttClient(),
		connectStarted: make(chan struct{}, 1),
		connectDone:    make(chan struct{}),
	}
	previous := newMqttClient
	newMqttClient = func(*mqtt.ClientOptions) mqtt.Client { return fake }
	defer func() { newMqttClient = previous }()

	client := &MqttClient{}
	errCh := make(chan error, 1)
	go func() { errCh <- client.MqttInit() }()
	select {
	case <-fake.connectStarted:
	case <-time.After(time.Second):
		t.Fatal("mqtt connect was not attempted")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("MqttInit error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("MqttInit did not return after Close")
	}
	select {
	case <-fake.disconnected:
	case <-time.After(time.Second):
		t.Fatal("Close did not disconnect mqtt client")
	}
}

func TestMqttClientCloseFailsPendingAndWaitsForInFlightPublish(t *testing.T) {
	fake := newTestMqttClient()
	started := make(chan struct{})
	release := make(chan struct{})
	fake.publish = func(string, byte, bool, interface{}) mqtt.Token {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		return testMqttToken{waitOK: true}
	}
	client := &MqttClient{}
	if err := client.startForTest(fake, 2); err != nil {
		t.Fatalf("startForTest: %v", err)
	}
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { first <- client.SendData("one", []byte("1")) }()
	<-started
	go func() { second <- client.SendData("two", []byte("2")) }()
	deadline := time.Now().Add(time.Second)
	for len(client.sendCh) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("pending publish was not queued")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan error, 1)
	go func() { closed <- client.Close() }()
	select {
	case <-closed:
		t.Fatal("Close returned while an accepted publish was in flight")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("in-flight publish error = %v", err)
	}
	if err := <-second; err == nil || !strings.Contains(err.Error(), "shutdown") {
		t.Fatalf("pending publish error = %v, want shutdown failure", err)
	}
	if err := <-closed; err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAetherLinkPluginUnloadClosesDefaultMqttClient(t *testing.T) {
	previousDefault := DefaultMqttClient
	client := &MqttClient{}
	if err := client.startForTest(newTestMqttClient(), 1); err != nil {
		t.Fatalf("startForTest: %v", err)
	}
	DefaultMqttClient = client
	defer func() { DefaultMqttClient = previousDefault }()

	if err := (&AetherLinkPlugin{}).Unload(); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if client.Client != nil || client.sendCh != nil || client.abortSend != nil || client.isConnected() || client.running {
		t.Fatal("Unload did not release internal mqtt client resources")
	}
}
