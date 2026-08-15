// 文件用途：维护 plugin\aetherlink\mqtt.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"context"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type MqttClient struct {
	Client mqtt.Client
	IsFlag bool
	mu     sync.RWMutex
	sendCh chan func()
	done   chan struct{}
	cancel context.CancelFunc
}

var DefaultMqttClient = &MqttClient{}

var mqttSendEnqueueTimeout = time.Second
var mqttConnectRetryInterval = time.Second
var newMqttClient = mqtt.NewClient

func (c *MqttClient) MqttInit() error {
	opts, addr := buildInternalMqttClientOptions()
	ctx, cancel := context.WithCancel(context.Background())
	client := newMqttClient(opts)
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.Client = client
	c.sendCh = make(chan func(), 100)
	c.done = make(chan struct{})
	c.cancel = cancel
	c.IsFlag = false
	sendCh := c.sendCh
	done := c.done
	c.mu.Unlock()
	go c.sendWorker(sendCh, done)

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
		} else {
			logInternalMqttInfo("mqtt client connected", zap.String("broker", addr))
			c.setConnected(true)
			break
		}
	}
	return nil
}

func (c *MqttClient) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	cancel := c.cancel
	done := c.done
	client := c.Client
	c.cancel = nil
	c.done = nil
	c.sendCh = nil
	c.Client = nil
	c.IsFlag = false
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		close(done)
	}
	if client != nil {
		client.Disconnect(250)
	}
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

func (c *MqttClient) snapshot() (mqtt.Client, chan func(), <-chan struct{}) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Client, c.sendCh, c.done
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

func (c *MqttClient) SendData(topic string, data []byte) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("mqtt publish panic for topic %q: %v", topic, recovered)
			Log.Warn("mqtt publish panic recovered", zap.String("topic", topic), zap.Int("payload_bytes", len(data)), zap.Error(err))
		}
	}()

	if c == nil {
		return fmt.Errorf("mqtt client is nil for topic %q", topic)
	}

	if !c.isConnected() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for i := 0; i < 10; i++ {
			if c.isConnected() {
				break
			}
			<-ticker.C
		}
	}

	client, sendCh, done := c.snapshot()
	if client == nil {
		return fmt.Errorf("mqtt client is not initialized for topic %q", topic)
	}

	if sendCh == nil {
		token := client.Publish(topic, 1, false, data)
		go func() {
			token.WaitTimeout(15 * time.Second)
		}()
		return nil
	}

	task := func() {
		token := client.Publish(topic, 1, false, data)
		if !token.WaitTimeout(15 * time.Second) {
			Log.Warn("mqtt publish timeout", zap.String("topic", topic), zap.Int("payload_bytes", len(data)))
			return
		}
		if err := token.Error(); err != nil {
			Log.Warn("mqtt publish failed", zap.String("topic", topic), zap.Int("payload_bytes", len(data)), zap.Error(err))
		}
	}

	select {
	case sendCh <- task:
		return nil
	case <-done:
		return fmt.Errorf("mqtt client is closing for topic %q", topic)
	case <-time.After(mqttSendEnqueueTimeout):
		return fmt.Errorf("mqtt publish queue full for topic %q", topic)
	}
}

func (c *MqttClient) sendWorker(sendCh <-chan func(), done <-chan struct{}) {
	for {
		select {
		case task, ok := <-sendCh:
			if !ok {
				return
			}
			task()
		case <-done:
			return
		}
	}
}
