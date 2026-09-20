// 文件用途：维护 plugin\aetherlink\mqtt.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type mqttPublishRequest struct {
	topic    string
	payload  []byte
	qos      byte
	retained bool
	result   chan error
}

type mqttPublisher interface {
	SendMessage(topic string, qos byte, retained bool, data []byte) error
}

type MqttClient struct {
	Client mqtt.Client
	IsFlag bool

	mu                 sync.RWMutex
	running            bool
	closing            bool
	closed             bool
	sendCh             chan mqttPublishRequest
	abortSend          chan struct{}
	ready              chan struct{}
	workerDone         chan struct{}
	connectDone        chan struct{}
	closeDone          chan struct{}
	cancel             context.CancelFunc
	sendEnqueueTimeout time.Duration
	readyTimeout       time.Duration
	senders            sync.WaitGroup
}

var DefaultMqttClient = &MqttClient{}
var mappedMQTTPublisher mqttPublisher = DefaultMqttClient
var mappedMQTTPublisherReady = func(ctx context.Context) error {
	if mappedMQTTPublisher != DefaultMqttClient {
		return nil
	}
	return DefaultMqttClient.WaitReady(ctx)
}

var mqttSendEnqueueTimeout = time.Second
var mqttPublishTimeout = 15 * time.Second
var mqttReadyTimeout = 15 * time.Second
var mqttConnectRetryInterval = time.Second
var newMqttClient = mqtt.NewClient

func (c *MqttClient) Start() error {
	if c == nil {
		return errors.New("mqtt client is nil")
	}
	c.mu.RLock()
	active := c.running || c.closing
	c.mu.RUnlock()
	if active {
		return errors.New("mqtt client is already running")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.MqttInit()
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-errCh:
			return err
		case <-ticker.C:
			c.mu.RLock()
			running := c.running
			c.mu.RUnlock()
			if running {
				return nil
			}
		}
	}
}

func (c *MqttClient) MqttInit() error {
	if c == nil {
		return errors.New("mqtt client is nil")
	}

	opts, addr := buildInternalMqttClientOptions()
	ctx, cancel := context.WithCancel(context.Background())
	client := newMqttClient(opts)
	if err := c.beginRuntime(client, cancel); err != nil {
		cancel()
		return err
	}

	c.mu.RLock()
	sendCh := c.sendCh
	abortSend := c.abortSend
	workerDone := c.workerDone
	connectDone := c.connectDone
	c.mu.RUnlock()
	go c.sendWorker(sendCh, abortSend, workerDone)
	defer close(connectDone)

	for {
		token := client.Connect()
		select {
		case <-token.Done():
		case <-ctx.Done():
			return ctx.Err()
		}
		if token.Error() != nil {
			logInternalMqttInfo("mqtt client connect failed, retrying", zap.String("broker", addr), zap.Error(token.Error()))
			select {
			case <-time.After(mqttConnectRetryInterval):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		logInternalMqttInfo("mqtt client connected", zap.String("broker", addr))
		c.markReady()
		return nil
	}
}

func (c *MqttClient) beginRuntime(client mqtt.Client, cancel context.CancelFunc) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running || c.closing {
		return errors.New("mqtt client is already running")
	}

	c.Client = client
	c.sendCh = make(chan mqttPublishRequest, 100)
	c.abortSend = make(chan struct{})
	c.ready = make(chan struct{})
	c.workerDone = make(chan struct{})
	c.connectDone = make(chan struct{})
	c.closeDone = make(chan struct{})
	c.cancel = cancel
	c.sendEnqueueTimeout = mqttSendEnqueueTimeout
	c.readyTimeout = mqttReadyTimeout
	c.running = true
	c.closing = false
	c.closed = false
	c.IsFlag = false
	return nil
}

func (c *MqttClient) markReady() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || c.closing || c.closed || c.IsFlag {
		return
	}
	c.IsFlag = true
	close(c.ready)
}

func (c *MqttClient) startForTest(client mqtt.Client, queueSize int) error {
	_, cancel := context.WithCancel(context.Background())
	if err := c.beginRuntime(client, cancel); err != nil {
		cancel()
		return err
	}
	c.mu.Lock()
	c.sendCh = make(chan mqttPublishRequest, queueSize)
	sendCh := c.sendCh
	abortSend := c.abortSend
	workerDone := c.workerDone
	connectDone := c.connectDone
	close(connectDone)
	c.mu.Unlock()
	go c.sendWorker(sendCh, abortSend, workerDone)
	c.markReady()
	return nil
}

func (c *MqttClient) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	if !c.running {
		closeDone := c.closeDone
		c.mu.Unlock()
		if closeDone != nil {
			<-closeDone
		}
		return nil
	}
	if c.closing {
		closeDone := c.closeDone
		c.mu.Unlock()
		<-closeDone
		return nil
	}

	c.closing = true
	c.IsFlag = false
	cancel := c.cancel
	sendCh := c.sendCh
	abortSend := c.abortSend
	workerDone := c.workerDone
	connectDone := c.connectDone
	closeDone := c.closeDone
	client := c.Client
	close(abortSend)
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	c.senders.Wait()
	close(sendCh)
	<-workerDone
	<-connectDone
	if client != nil {
		client.Disconnect(250)
	}

	c.mu.Lock()
	c.Client = nil
	c.sendCh = nil
	c.abortSend = nil
	c.ready = nil
	c.workerDone = nil
	c.connectDone = nil
	c.cancel = nil
	c.sendEnqueueTimeout = 0
	c.readyTimeout = 0
	c.running = false
	c.closing = false
	c.closed = true
	c.mu.Unlock()
	close(closeDone)
	return nil
}

func (c *MqttClient) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.IsFlag = connected
}

func (c *MqttClient) isConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.IsFlag
}

func buildInternalMqttClientOptions() (*mqtt.ClientOptions, string) {
	opts := mqtt.NewClientOptions()
	opts.SetUsername("root")
	password := viper.GetString("mqtt.password")
	opts.SetPassword(password)

	addr := viper.GetString("mqtt.broker")
	if addr == "" {
		addr = "127.0.0.1:1883"
	}
	opts.AddBroker(addr)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetryInterval(1 * time.Second)
	opts.SetMaxReconnectInterval(200 * time.Second)
	opts.SetOrderMatters(true)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		logInternalMqttInfo("mqtt client connected", zap.String("broker", addr))
	})
	opts.SetClientID("aetherlink-gmqtt-client")
	return opts, addr
}

func logInternalMqttInfo(message string, fields ...zap.Field) {
	if Log == nil {
		return
	}
	Log.Info(message, fields...)
}

func (c *MqttClient) WaitReady(ctx context.Context) error {
	if c == nil {
		return errors.New("mqtt client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.RLock()
	if !c.running || c.closing || c.closed || c.ready == nil {
		c.mu.RUnlock()
		return errors.New("mqtt client is not running")
	}
	if c.IsFlag {
		c.mu.RUnlock()
		return nil
	}
	ready := c.ready
	abortSend := c.abortSend
	readyTimeout := c.readyTimeout
	c.mu.RUnlock()

	if readyTimeout <= 0 {
		readyTimeout = mqttReadyTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	select {
	case <-ready:
		return nil
	case <-abortSend:
		return errors.New("mqtt client closed before becoming ready")
	case <-waitCtx.Done():
		return waitCtx.Err()
	}
}

func (c *MqttClient) SendData(topic string, data []byte) error {
	return c.SendMessage(topic, 1, false, data)
}

func (c *MqttClient) SendMessage(topic string, qos byte, retained bool, data []byte) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("mqtt publish panic for topic %q: %v", topic, recovered)
			if Log != nil {
				Log.Warn("mqtt publish panic recovered", zap.String("topic", topic), zap.Int("payload_bytes", len(data)), zap.Error(err))
			}
		}
	}()
	if topic == "" {
		return errors.New("mqtt publish topic is required")
	}
	if qos > 2 {
		return fmt.Errorf("mqtt publish qos %d is invalid", qos)
	}

	request, sendCh, enqueueTimeout, err := c.beginSend(topic, qos, retained, data)
	if err != nil {
		return err
	}
	defer c.senders.Done()

	timer := time.NewTimer(enqueueTimeout)
	defer timer.Stop()
	select {
	case sendCh <- request:
	case <-timer.C:
		return fmt.Errorf("mqtt publish queue full for topic %q", topic)
	}

	return <-request.result
}

func (c *MqttClient) beginSend(topic string, qos byte, retained bool, data []byte) (mqttPublishRequest, chan mqttPublishRequest, time.Duration, error) {
	if c == nil {
		return mqttPublishRequest{}, nil, 0, fmt.Errorf("mqtt client is nil for topic %q", topic)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || c.Client == nil || c.sendCh == nil || c.abortSend == nil {
		return mqttPublishRequest{}, nil, 0, fmt.Errorf("mqtt client is not initialized for topic %q", topic)
	}
	if c.closing || c.closed {
		return mqttPublishRequest{}, nil, 0, fmt.Errorf("mqtt client is unavailable for topic %q", topic)
	}
	if !c.IsFlag || !c.Client.IsConnectionOpen() {
		return mqttPublishRequest{}, nil, 0, fmt.Errorf("mqtt client is not connected for topic %q", topic)
	}
	c.senders.Add(1)
	return mqttPublishRequest{
		topic:    topic,
		payload:  append([]byte(nil), data...),
		qos:      qos,
		retained: retained,
		result:   make(chan error, 1),
	}, c.sendCh, c.sendEnqueueTimeout, nil
}

func (c *MqttClient) sendWorker(sendCh <-chan mqttPublishRequest, abortSend <-chan struct{}, workerDone chan<- struct{}) {
	defer close(workerDone)
	for request := range sendCh {
		select {
		case <-abortSend:
			request.result <- fmt.Errorf("mqtt publish canceled during shutdown for topic %q", request.topic)
		default:
			request.result <- c.publish(request)
		}
	}
}

func (c *MqttClient) publish(request mqttPublishRequest) error {
	c.mu.RLock()
	client := c.Client
	c.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("mqtt client is not initialized for topic %q", request.topic)
	}

	token := client.Publish(request.topic, request.qos, request.retained, request.payload)
	if !token.WaitTimeout(mqttPublishTimeout) {
		return fmt.Errorf("mqtt publish timeout for topic %q", request.topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt publish failed for topic %q: %w", request.topic, err)
	}
	return nil
}
