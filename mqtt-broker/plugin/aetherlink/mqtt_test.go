// 文件用途：维护 plugin\aetherlink\mqtt_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"context"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
)

type testMqttToken struct{}

func (testMqttToken) Wait() bool                     { return true }
func (testMqttToken) WaitTimeout(time.Duration) bool { return true }
func (testMqttToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (testMqttToken) Error() error { return nil }

type testMqttClient struct{}

func (testMqttClient) IsConnected() bool                                      { return true }
func (testMqttClient) IsConnectionOpen() bool                                 { return true }
func (testMqttClient) Connect() mqtt.Token                                    { return testMqttToken{} }
func (testMqttClient) Disconnect(uint)                                        {}
func (testMqttClient) Publish(string, byte, bool, interface{}) mqtt.Token     { return testMqttToken{} }
func (testMqttClient) Subscribe(string, byte, mqtt.MessageHandler) mqtt.Token { return testMqttToken{} }
func (testMqttClient) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
	return testMqttToken{}
}
func (testMqttClient) Unsubscribe(...string) mqtt.Token        { return testMqttToken{} }
func (testMqttClient) AddRoute(string, mqtt.MessageHandler)    {}
func (testMqttClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

type blockingMqttToken struct {
	done <-chan struct{}
}

func (t blockingMqttToken) Wait() bool {
	<-t.done
	return true
}
func (t blockingMqttToken) WaitTimeout(time.Duration) bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}
func (t blockingMqttToken) Done() <-chan struct{} { return t.done }
func (t blockingMqttToken) Error() error          { return nil }

type blockingConnectMqttClient struct {
	testMqttClient
	connectStarted   chan struct{}
	disconnectCalled chan struct{}
}

func (c *blockingConnectMqttClient) Connect() mqtt.Token {
	select {
	case c.connectStarted <- struct{}{}:
	default:
	}
	return blockingMqttToken{done: make(chan struct{})}
}

func (c *blockingConnectMqttClient) Disconnect(uint) {
	select {
	case c.disconnectCalled <- struct{}{}:
	default:
	}
}

func TestBuildInternalMqttClientOptionsUsesRootIdentityAndOrderedDelivery(t *testing.T) {
	viper.Set("mqtt.password", "root-pass")
	viper.Set("mqtt.broker", "127.0.0.1:1883")
	t.Cleanup(viper.Reset)

	opts, addr := buildInternalMqttClientOptions()
	if addr != "127.0.0.1:1883" {
		t.Fatalf("addr = %q", addr)
	}
	if opts.Username != "root" {
		t.Fatalf("username = %q", opts.Username)
	}
	if opts.Password != "root-pass" {
		t.Fatalf("password = %q", opts.Password)
	}
	if opts.ClientID != "aetherlink-gmqtt-client" {
		t.Fatalf("client id = %q", opts.ClientID)
	}
	if !opts.CleanSession || !opts.AutoReconnect || !opts.Order {
		t.Fatal("internal mqtt client should use clean session, auto reconnect, and ordered delivery")
	}
}

func TestMqttClientStartsWithNoChannelsOrConnectedClient(t *testing.T) {
	client := &MqttClient{}
	if client.Client != nil || client.sendCh != nil || client.done != nil || client.IsFlag {
		t.Fatal("fresh mqtt client should be zero-valued before init")
	}
}

func TestMqttClientSendDataReturnsWhenQueueIsFull(t *testing.T) {
	client := &MqttClient{
		Client: testMqttClient{},
		sendCh: make(chan func(), 1),
	}
	client.setConnected(true)
	client.sendCh <- func() {}

	prevTimeout := mqttSendEnqueueTimeout
	mqttSendEnqueueTimeout = 10 * time.Millisecond
	t.Cleanup(func() { mqttSendEnqueueTimeout = prevTimeout })

	err := client.SendData("devices/status/dev1", []byte("1"))
	if err == nil {
		t.Fatal("expected queue-full error")
	}
	if !strings.Contains(err.Error(), "queue full") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMqttClientSendDataRejectsUninitializedClient(t *testing.T) {
	client := &MqttClient{}
	client.setConnected(true)

	err := client.SendData("devices/status/dev1", []byte("1"))
	if err == nil {
		t.Fatal("expected uninitialized-client error")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMqttInitReturnsWhenCloseCancelsConnectRetry(t *testing.T) {
	fake := &blockingConnectMqttClient{
		connectStarted:   make(chan struct{}, 1),
		disconnectCalled: make(chan struct{}, 1),
	}
	prevNewClient := newMqttClient
	newMqttClient = func(*mqtt.ClientOptions) mqtt.Client { return fake }
	t.Cleanup(func() { newMqttClient = prevNewClient })

	client := &MqttClient{}
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.MqttInit()
	}()

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
		if err != context.Canceled {
			t.Fatalf("MqttInit error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("MqttInit did not return after Close")
	}
	select {
	case <-fake.disconnectCalled:
	case <-time.After(time.Second):
		t.Fatal("Close did not disconnect mqtt client")
	}
}

func TestMqttClientCloseStopsWorkerAfterInFlightTask(t *testing.T) {
	client := &MqttClient{
		sendCh: make(chan func(), 1),
		done:   make(chan struct{}),
	}
	taskStarted := make(chan struct{})
	releaseTask := make(chan struct{})
	workerStopped := make(chan struct{})

	go func() {
		client.sendWorker(client.sendCh, client.done)
		close(workerStopped)
	}()
	client.sendCh <- func() {
		close(taskStarted)
		<-releaseTask
	}

	select {
	case <-taskStarted:
	case <-time.After(time.Second):
		t.Fatal("send worker did not start queued task")
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	close(releaseTask)

	select {
	case <-workerStopped:
	case <-time.After(time.Second):
		t.Fatal("send worker did not stop after Close released an in-flight task")
	}
}

func TestAetherLinkPluginUnloadClosesDefaultMqttClient(t *testing.T) {
	previousDefault := DefaultMqttClient
	ctx, cancel := context.WithCancel(context.Background())
	client := &MqttClient{
		Client: testMqttClient{},
		sendCh: make(chan func(), 1),
		done:   make(chan struct{}),
		cancel: cancel,
	}
	client.setConnected(true)
	DefaultMqttClient = client
	t.Cleanup(func() {
		DefaultMqttClient = previousDefault
	})

	if err := (&AetherLinkPlugin{}).Unload(); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Unload did not cancel the internal mqtt client")
	}
	if client.Client != nil || client.sendCh != nil || client.done != nil || client.isConnected() {
		t.Fatal("Unload did not release internal mqtt client resources")
	}
}
