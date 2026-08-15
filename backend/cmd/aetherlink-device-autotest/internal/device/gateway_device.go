// 文件用途：实现网关设备在自动测试中的 MQTT 行为。
// 核心逻辑：依据网关配置建立 MQTT 连接，发布嵌套遥测、属性、事件和响应，并订阅网关下行控制主题。
// 关键注意事项：网关拓扑来自配置文件，测试数据需与 gateway_data、sub_device_data 和 sub_gateway_data 契约一致。
// 重构建议：可复用直连设备的连接、订阅、消息缓存和发布模板，仅保留网关 topic 与报文差异。

/*
Purpose: 实现网关设备在自动测试中的 MQTT 行为。
Core logic: 依据网关配置建立 MQTT 连接，使用网关报文构建器发布嵌套遥测、属性、事件和响应，并订阅网关下行控制主题。
Important notes: 网关拓扑来自配置文件，测试数据需要与 gateway_data、sub_device_data、sub_gateway_data 的平台契约保持一致。
Refactor suggestion: 可复用直连设备的连接、订阅、消息缓存和发布模板，仅保留网关 topic 与报文差异，降低双实现漂移风险。
*/
package device

import (
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
	"aetherlink-iot/aetherlink-device-autotest/internal/protocol"
	"aetherlink-iot/aetherlink-device-autotest/internal/utils"
)

// GatewayDevice 网关设备实现
type GatewayDevice struct {
	config   *config.Config
	client   mqtt.Client
	topics   *utils.MQTTTopics
	builder  protocol.MessageBuilder
	topology *config.GatewayConfig
	logger   *zap.Logger

	// 与直连实现一样按实际 topic 缓存消息，方便多层网关场景做模式匹配。
	receivedMessages map[string][]ReceivedMessage
	mu               sync.RWMutex
}

// NewGatewayDevice 创建网关设备
func NewGatewayDevice(cfg *config.Config, logger *zap.Logger) *GatewayDevice {
	return &GatewayDevice{
		config:           cfg,
		topics:           utils.NewGatewayMQTTTopics(cfg.Device.DeviceNumber),
		builder:          protocol.NewGatewayMessageBuilder(nil),
		topology:         &cfg.Gateway, // 从配置中获取拓扑结构
		logger:           logger,
		receivedMessages: make(map[string][]ReceivedMessage),
	}
}

// Connect 连接到MQTT Broker
func (d *GatewayDevice) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", d.config.MQTT.Broker))
	opts.SetClientID(d.config.MQTT.ClientID)
	opts.SetUsername(d.config.MQTT.Username)
	opts.SetPassword(d.config.MQTT.Password)
	opts.SetCleanSession(d.config.MQTT.CleanSession)
	opts.SetKeepAlive(time.Duration(d.config.MQTT.KeepAlive) * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetMaxReconnectInterval(10 * time.Second)

	// 真实环境下的 broker 抖动会先体现在这里，日志是定位多层链路问题的重要线索。
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		d.logger.Error("MQTT connection lost", zap.Error(err))
	})

	// 当前只记录连接状态，不在回调里自动恢复订阅，避免隐藏测试流程。
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		d.logger.Info("MQTT connected successfully")
	})

	d.client = mqtt.NewClient(opts)

	token := d.client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("connection timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("connection failed: %w", token.Error())
	}

	d.logger.Info("Gateway device connected",
		zap.String("broker", d.config.MQTT.Broker),
		zap.String("client_id", d.config.MQTT.ClientID),
		zap.String("device_number", d.config.Device.DeviceNumber),
		zap.Int("sub_devices", len(d.topology.SubDevices)),
		zap.Int("sub_gateways", len(d.topology.SubGateways)))

	return nil
}

// Disconnect 断开连接
func (d *GatewayDevice) Disconnect() {
	if d.client != nil && d.client.IsConnected() {
		d.client.Disconnect(250)
		d.logger.Info("Gateway device disconnected")
	}
}

// IsConnected 检查连接状态
func (d *GatewayDevice) IsConnected() bool {
	return d.client != nil && d.client.IsConnected()
}

// PublishTelemetry 上报遥测数据
func (d *GatewayDevice) PublishTelemetry(data interface{}) error {
	payload, err := d.builder.BuildTelemetry(data)
	if err != nil {
		return err
	}

	topic := d.topics.Telemetry()
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	d.logger.Info("Gateway telemetry published",
		zap.String("topic", topic),
		zap.String("payload", string(payload)))

	return nil
}

// PublishAttribute 上报属性数据
func (d *GatewayDevice) PublishAttribute(data interface{}, messageID string) error {
	payload, err := d.builder.BuildAttribute(data)
	if err != nil {
		return err
	}

	topic := d.topics.Attributes(messageID)
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	d.logger.Info("Gateway attribute published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("payload", string(payload)))

	return nil
}

// PublishEvent 上报事件数据
func (d *GatewayDevice) PublishEvent(method string, params interface{}, messageID string) error {
	payload, err := d.builder.BuildEvent(method, params)
	if err != nil {
		return err
	}

	topic := d.topics.Event(messageID)
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	d.logger.Info("Gateway event published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("method", method),
		zap.String("payload", string(payload)))

	return nil
}

// PublishCommandResponse 发送命令响应
func (d *GatewayDevice) PublishCommandResponse(messageID string, success bool, method string) error {
	payload, err := d.builder.BuildResponse(success, method)
	if err != nil {
		return err
	}

	topic := d.topics.CommandResponse(messageID)
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	d.logger.Info("Gateway command response published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("payload", string(payload)))

	return nil
}

// PublishAttributeSetResponse 发送属性设置响应
func (d *GatewayDevice) PublishAttributeSetResponse(messageID string, success bool) error {
	payload, err := d.builder.BuildResponse(success, "")
	if err != nil {
		return err
	}

	topic := d.topics.AttributeSetResponse(messageID)
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	d.logger.Info("Gateway attribute set response published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("payload", string(payload)))

	return nil
}

// messageHandler 通用消息处理器
func (d *GatewayDevice) messageHandler(client mqtt.Client, msg mqtt.Message) {
	d.mu.Lock()
	defer d.mu.Unlock()

	topic := msg.Topic()
	payload := msg.Payload()

	// 保留原始网关 topic 与 payload，后续测试根据不同层级协议自行解析。
	d.receivedMessages[topic] = append(d.receivedMessages[topic], ReceivedMessage{
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
	})

	d.logger.Info("Gateway message received",
		zap.String("topic", topic),
		zap.String("payload", string(payload)))
}

// subscribe 订阅主题
func (d *GatewayDevice) subscribe(topic string) error {
	token := d.client.Subscribe(topic, d.config.MQTT.QoS, d.messageHandler)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("subscribe failed: %w", token.Error())
	}

	d.logger.Info("Gateway subscribed to topic", zap.String("topic", topic))
	return nil
}

// SubscribeAll 订阅所有需要的主题
func (d *GatewayDevice) SubscribeAll() error {
	// 网关模式下订阅集合与直连接近，但 topic 前缀和 message 路径遵循网关契约。
	topics := []string{
		d.topics.TelemetryControl(),
		d.topics.AttributeSet(),
		d.topics.AttributeGet(),
		d.topics.Command(),
		d.topics.AttributeResponse(),
		d.topics.EventResponse(),
	}

	for _, topic := range topics {
		if err := d.subscribe(topic); err != nil {
			return fmt.Errorf("failed to subscribe %s: %w", topic, err)
		}
	}

	return nil
}

// GetReceivedMessages 获取接收到的消息
func (d *GatewayDevice) GetReceivedMessages(topicPattern string, timeout time.Duration) []ReceivedMessage {
	deadline := time.Now().Add(timeout)

	// 网关测试依赖异步消息回流，这里保持与直连实现一致的轮询等待策略。
	for time.Now().Before(deadline) {
		d.mu.RLock()
		for topic, messages := range d.receivedMessages {
			if matchTopic(topic, topicPattern) && len(messages) > 0 {
				result := make([]ReceivedMessage, len(messages))
				copy(result, messages)
				d.mu.RUnlock()

				d.logger.Debug("Found matching gateway messages",
					zap.String("pattern", topicPattern),
					zap.String("actual_topic", topic),
					zap.Int("count", len(result)))

				return result
			}
		}
		d.mu.RUnlock()
		time.Sleep(100 * time.Millisecond)
	}

	// 超时后打印调试信息
	d.mu.RLock()
	d.logger.Warn("No matching gateway messages found",
		zap.String("pattern", topicPattern),
		zap.Int("total_topics", len(d.receivedMessages)))
	for topic, msgs := range d.receivedMessages {
		d.logger.Debug("Available gateway topic",
			zap.String("topic", topic),
			zap.Int("message_count", len(msgs)))
	}
	d.mu.RUnlock()

	return nil
}

// ClearReceivedMessages 清空接收到的消息
func (d *GatewayDevice) ClearReceivedMessages(topicPattern string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if topicPattern == "" {
		d.receivedMessages = make(map[string][]ReceivedMessage)
		return
	}

	for topic := range d.receivedMessages {
		if matchTopic(topic, topicPattern) {
			delete(d.receivedMessages, topic)
		}
	}
}
