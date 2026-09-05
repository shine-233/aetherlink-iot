// 文件用途：实现直连设备在自动测试中的 MQTT 行为。
// 核心逻辑：依据配置建立 MQTT 连接，发布遥测、属性、事件和响应，并缓存平台下行消息。
// 关键注意事项：该实现依赖真实 broker，消息缓存需保持并发安全，topic 匹配要与平台协议一致。
// 重构建议：可抽出 MQTT client 适配层和消息缓存组件，让连接重试、发布超时和 topic 匹配独立可测。

/*
Purpose: 实现直连设备在自动测试中的 MQTT 行为。
Core logic: 依据配置建立 Paho MQTT 连接，使用直连报文构建器发布遥测、属性、事件和响应，并缓存订阅收到的平台下行消息。
Important notes: 该实现依赖真实 MQTT broker，消息缓存由互斥锁保护；测试读取消息时使用 topic 通配符匹配和超时轮询。
Refactor suggestion: 可抽出 MQTT client 适配层和消息缓存组件，让连接重试、发布超时和 topic 匹配能被独立单测覆盖。
*/
package device

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
	"aetherlink-iot/aetherlink-device-autotest/internal/protocol"
	"aetherlink-iot/aetherlink-device-autotest/internal/utils"
)

// DirectDevice 直连设备实现
type DirectDevice struct {
	config  *config.Config
	client  mqtt.Client
	topics  *utils.MQTTTopics
	builder protocol.MessageBuilder
	logger  *zap.Logger

	// 以实际 topic 为 key 缓存收到的下行消息，便于测试按通配符检索。
	receivedMessages map[string][]ReceivedMessage
	mu               sync.RWMutex
}

// NewDirectDevice 创建直连设备
func NewDirectDevice(cfg *config.Config, logger *zap.Logger) *DirectDevice {
	return &DirectDevice{
		config:           cfg,
		topics:           utils.NewMQTTTopics(cfg.Device.DeviceNumber),
		builder:          protocol.NewDirectMessageBuilder(),
		logger:           logger,
		receivedMessages: make(map[string][]ReceivedMessage),
	}
}

// Connect 连接到MQTT Broker
func (d *DirectDevice) Connect() error {
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

	// Paho 会尝试自动重连，这里主要负责把链路异常暴露到日志中。
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		d.logger.Error("MQTT connection lost", zap.Error(err))
	})

	// 当前不在重连回调里自动补订阅，而是由调用方显式控制订阅时机。
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

	d.logger.Info("Direct device connected",
		zap.String("broker", d.config.MQTT.Broker),
		zap.String("client_id", d.config.MQTT.ClientID),
		zap.String("device_number", d.config.Device.DeviceNumber))

	return nil
}

// Disconnect 断开连接
func (d *DirectDevice) Disconnect() {
	if d.client != nil && d.client.IsConnected() {
		d.client.Disconnect(250)
		d.logger.Info("Direct device disconnected")
	}
}

// IsConnected 检查连接状态
func (d *DirectDevice) IsConnected() bool {
	return d.client != nil && d.client.IsConnected()
}

// PublishTelemetry 上报遥测数据
func (d *DirectDevice) PublishTelemetry(data interface{}) error {
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

	d.logger.Info("Telemetry published",
		zap.String("topic", topic),
		zap.String("payload", string(payload)))

	return nil
}

// PublishAttribute 上报属性数据
func (d *DirectDevice) PublishAttribute(data interface{}, messageID string) error {
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

	d.logger.Info("Attribute published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("payload", string(payload)))

	return nil
}

// PublishEvent 上报事件数据
func (d *DirectDevice) PublishEvent(method string, params interface{}, messageID string) error {
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

	d.logger.Info("Event published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("method", method),
		zap.String("payload", string(payload)))

	return nil
}

// PublishStatus reports the device's online state using the status topic
// consumed by backend/internal/adapter/mqttadapter. This is separate from
// telemetry: a device without a device-config heartbeat still needs an
// explicit status transition before command preview can consider it eligible.
func (d *DirectDevice) PublishStatus(online bool) error {
	if d.client == nil || !d.client.IsConnected() {
		return fmt.Errorf("device must be connected before publishing status")
	}
	payload := []byte("0")
	if online {
		payload = []byte("1")
	}
	topic := d.topics.Status(d.config.Device.DeviceID)
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish status timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish status failed: %w", token.Error())
	}
	d.logger.Info("Device status published",
		zap.String("topic", topic),
		zap.String("payload", string(payload)),
		zap.Bool("online", online))
	return nil
}

// Topics 返回该设备绑定的 MQTT 主题构造器
func (d *DirectDevice) Topics() *utils.MQTTTopics {
	return d.topics
}

// PublishRaw 发布原始字节流到指定主题
func (d *DirectDevice) PublishRaw(topic string, payload []byte) error {
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}
	d.logger.Info("Raw message published",
		zap.String("topic", topic),
		zap.Int("payload_len", len(payload)))
	return nil
}

// PublishCommandResponse 发送命令响应
func (d *DirectDevice) PublishCommandResponse(messageID string, success bool, method string) error {
	innerPayload, err := d.builder.BuildResponse(success, method)
	if err != nil {
		return err
	}

	topic := d.topics.CommandResponse(messageID)
	// The AetherLink broker adds the authenticated device_id/values envelope
	// around every standard device uplink. Publishing an envelope here would
	// make the broker wrap it a second time and the backend would parse the
	// outer object as a successful response because it has no result field.
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, innerPayload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	d.logger.Info("Command response published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("payload", string(innerPayload)))

	return nil
}

// PublishAttributeSetResponse 发送属性设置响应
func (d *DirectDevice) PublishAttributeSetResponse(messageID string, success bool) error {
	innerPayload, err := d.builder.BuildResponse(success, "")
	if err != nil {
		return err
	}

	topic := d.topics.AttributeSetResponse(messageID)
	// As with command responses, the broker supplies the standard uplink
	// envelope; keep the device publication at the protocol's inner payload.
	token := d.client.Publish(topic, d.config.MQTT.QoS, false, innerPayload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	d.logger.Info("Attribute set response published",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("payload", string(innerPayload)))

	return nil
}

// RunCommandEmulator keeps a connected direct device alive and acknowledges
// every platform command through the same MQTT topic and response envelope as
// the integration device. It is intentionally opt-in (CLI mode) so ordinary
// telemetry/attribute/event smoke runs do not mutate command state.
// RunCommandEmulator keeps a connected direct device alive and acknowledges
// every platform command through the same MQTT topic and response envelope as
// the integration device. It is intentionally opt-in (CLI mode) so ordinary
// telemetry/attribute/event smoke runs do not mutate command state.
func (d *DirectDevice) RunCommandEmulator(stop <-chan struct{}, success bool, receiptPath string) error {
	if d.client == nil || !d.client.IsConnected() {
		return fmt.Errorf("device must be connected before starting command emulator")
	}

	topic := d.topics.Command()
	token := d.client.Subscribe(topic, d.config.MQTT.QoS, func(_ mqtt.Client, msg mqtt.Message) {
		messageID, method, params, ok := commandMessageFullDetails(msg.Topic(), msg.Payload())
		if !ok {
			d.logger.Warn("Ignoring malformed command in emulator", zap.String("topic", msg.Topic()))
			return
		}
		responseSuccess := commandResponseSuccess(success, method, d.config.Test.CommandFailureIdentify)
		if err := d.PublishCommandResponse(messageID, responseSuccess, method); err != nil {
			d.logger.Error("Failed to publish emulated command response", zap.Error(err), zap.String("message_id", messageID))
			return
		}
		if strings.TrimSpace(receiptPath) != "" {
			ackResult := 0
			if !responseSuccess {
				ackResult = 1
			}
			receipt := map[string]interface{}{
				"message_id": messageID,
				"method":     method,
				"params":     params,
				"topic":      msg.Topic(),
				"ack_topic":  d.topics.CommandResponse(messageID),
				"ack_payload": map[string]interface{}{
					"result": ackResult,
					"method": method,
				},
			}
			if err := appendReceiptLine(receiptPath, receipt); err != nil {
				d.logger.Error("Failed to append command receipt", zap.Error(err), zap.String("receipt_path", receiptPath))
			}
		}
		d.logger.Info("Emulated command response published",
			zap.String("message_id", messageID),
			zap.String("method", method),
			zap.Bool("success", responseSuccess))
	})
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("command emulator subscription timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("command emulator subscription failed: %w", token.Error())
	}

	if err := d.PublishStatus(true); err != nil {
		return fmt.Errorf("publish emulator online status: %w", err)
	}

	// Keep the status observable while the emulator is running. The seeded
	// command fixture has no heartbeat device-config, so this explicit status
	// refresh is the real contract that keeps the row eligible for dispatch.
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()
	for {
		select {
		case <-stop:
			if err := d.PublishStatus(false); err != nil {
				return fmt.Errorf("publish emulator offline status: %w", err)
			}
			goto unsubscribe
		case <-statusTicker.C:
			if err := d.PublishStatus(true); err != nil {
				return fmt.Errorf("refresh emulator online status: %w", err)
			}
		}
	}

unsubscribe:
	unsubscribe := d.client.Unsubscribe(topic)
	if !unsubscribe.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("command emulator unsubscribe timeout")
	}
	return unsubscribe.Error()
}

// RunOTAEmulator keeps a connected direct device alive, listens for OTA informs,
// reports progressive status updates on ota/devices/progress, and writes receipts.
func (d *DirectDevice) RunOTAEmulator(stop <-chan struct{}, receiptPath, progressValuesStr, otaVersion string, otaFailure bool) error {
	if d.client == nil || !d.client.IsConnected() {
		return fmt.Errorf("device must be connected before starting ota emulator")
	}

	informTopic := d.topics.OTAInform()
	token := d.client.Subscribe(informTopic, d.config.MQTT.QoS, func(_ mqtt.Client, msg mqtt.Message) {
		d.logger.Info("OTA inform message received", zap.String("topic", msg.Topic()))
		if strings.TrimSpace(receiptPath) != "" {
			var rawPayload interface{}
			if err := json.Unmarshal(msg.Payload(), &rawPayload); err != nil {
				rawPayload = string(msg.Payload())
			}
			receipt := map[string]interface{}{
				"kind":    "inform",
				"topic":   msg.Topic(),
				"payload": rawPayload,
			}
			if err := appendReceiptLine(receiptPath, receipt); err != nil {
				d.logger.Error("Failed to write ota inform receipt", zap.Error(err))
			}
		}

		progressValues := parseProgressValues(progressValuesStr)
		for i, p := range progressValues {
			time.Sleep(250 * time.Millisecond)
			status := int16(3) // upgrading
			isLast := i == len(progressValues)-1
			desc := fmt.Sprintf("OTA upgrading, progress %d%%", p)
			if otaFailure && isLast {
				status = 5 // failure
				desc = "OTA upgrade failed"
			} else if p >= 100 {
				status = 4 // success
				desc = "OTA upgrade succeeded"
			}

			params := map[string]interface{}{
				"progress":    p,
				"status":      status,
				"description": desc,
			}
			if otaVersion != "" {
				params["version"] = otaVersion
			}
			eventPayload := map[string]interface{}{
				"method": "ota_progress",
				"params": params,
			}
			payloadBytes, err := json.Marshal(eventPayload)
			if err != nil {
				d.logger.Error("Failed to marshal ota_progress event", zap.Error(err))
				continue
			}

			progressTopic := "ota/devices/progress"
			pubToken := d.client.Publish(progressTopic, d.config.MQTT.QoS, false, payloadBytes)
			if !pubToken.WaitTimeout(5 * time.Second) {
				d.logger.Error("Failed to publish ota_progress: timeout")
			} else if pubToken.Error() != nil {
				d.logger.Error("Failed to publish ota_progress", zap.Error(pubToken.Error()))
			} else {
				d.logger.Info("OTA progress published", zap.Int("progress", p), zap.Int16("status", status))
			}

			if strings.TrimSpace(receiptPath) != "" {
				receipt := map[string]interface{}{
					"kind":     "progress",
					"progress": p,
					"topic":    progressTopic,
					"payload":  eventPayload,
				}
				if err := appendReceiptLine(receiptPath, receipt); err != nil {
					d.logger.Error("Failed to write ota progress receipt", zap.Error(err))
				}
			}
		}
	})
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("ota emulator subscription timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("ota emulator subscription failed: %w", token.Error())
	}

	if err := d.PublishStatus(true); err != nil {
		return fmt.Errorf("publish ota emulator online status: %w", err)
	}

	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()
	for {
		select {
		case <-stop:
			if err := d.PublishStatus(false); err != nil {
				return fmt.Errorf("publish ota emulator offline status: %w", err)
			}
			goto unsubscribe
		case <-statusTicker.C:
			if err := d.PublishStatus(true); err != nil {
				return fmt.Errorf("refresh ota emulator online status: %w", err)
			}
		}
	}

unsubscribe:
	unsubscribe := d.client.Unsubscribe(informTopic)
	if !unsubscribe.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("ota emulator unsubscribe timeout")
	}
	return unsubscribe.Error()
}

func parseProgressValues(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" {
		return []int{0, 10, 50, 100}
	}
	parts := strings.Split(s, ",")
	var result []int
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if v, err := strconv.Atoi(part); err == nil {
			result = append(result, v)
		}
	}
	if len(result) == 0 {
		return []int{0, 10, 50, 100}
	}
	return result
}

func appendReceiptLine(receiptPath string, obj interface{}) error {
	dir := filepath.Dir(receiptPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(receiptPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// commandResponseSuccess keeps the emulator successful by default while
// allowing one explicitly named command contract to exercise a real failed
// acknowledgement path. This avoids racing a test-injected failure with the
// default success response for the same submitted command.
func commandResponseSuccess(defaultSuccess bool, method, failureIdentify string) bool {
	if defaultSuccess && strings.TrimSpace(failureIdentify) != "" && strings.TrimSpace(method) == strings.TrimSpace(failureIdentify) {
		return false
	}
	return defaultSuccess
}

func commandMessageDetails(topic string, payload []byte) (string, string, bool) {
	id, method, _, ok := commandMessageFullDetails(topic, payload)
	return id, method, ok
}

func commandMessageFullDetails(topic string, payload []byte) (string, string, interface{}, bool) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) != 4 || parts[0] != "devices" || parts[1] != "command" || parts[2] == "" || parts[3] == "" {
		return "", "", nil, false
	}
	var command struct {
		Method string      `json:"method"`
		Params interface{} `json:"params"`
	}
	if err := json.Unmarshal(payload, &command); err != nil || strings.TrimSpace(command.Method) == "" {
		return "", "", nil, false
	}
	return parts[3], command.Method, command.Params, true
}

// messageHandler 通用消息处理器
func (d *DirectDevice) messageHandler(client mqtt.Client, msg mqtt.Message) {
	d.mu.Lock()
	defer d.mu.Unlock()

	topic := msg.Topic()
	payload := msg.Payload()

	// 保留原始 topic 与 payload，后续由测试按协议语义解析并断言。
	d.receivedMessages[topic] = append(d.receivedMessages[topic], ReceivedMessage{
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
	})

	d.logger.Info("Message received",
		zap.String("topic", topic),
		zap.String("payload", string(payload)))
}

// subscribe 订阅主题
func (d *DirectDevice) subscribe(topic string) error {
	token := d.client.Subscribe(topic, d.config.MQTT.QoS, d.messageHandler)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("subscribe failed: %w", token.Error())
	}

	d.logger.Info("Subscribed to topic", zap.String("topic", topic))
	return nil
}

// SubscribeAll 订阅所有需要的主题
func (d *DirectDevice) SubscribeAll() error {
	// 这里只订阅平台下行与平台回执主题，不包含设备主动上报主题。
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
func (d *DirectDevice) GetReceivedMessages(topicPattern string, timeout time.Duration) []ReceivedMessage {
	deadline := time.Now().Add(timeout)

	// 通过短轮询等待异步 MQTT 消息到达，减少测试用例对内部锁的感知。
	for time.Now().Before(deadline) {
		d.mu.RLock()
		for topic, messages := range d.receivedMessages {
			if matchTopic(topic, topicPattern) && len(messages) > 0 {
				result := make([]ReceivedMessage, len(messages))
				copy(result, messages)
				d.mu.RUnlock()

				d.logger.Debug("Found matching messages",
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
	d.logger.Warn("No matching messages found",
		zap.String("pattern", topicPattern),
		zap.Int("total_topics", len(d.receivedMessages)))
	for topic, msgs := range d.receivedMessages {
		d.logger.Debug("Available topic",
			zap.String("topic", topic),
			zap.Int("message_count", len(msgs)))
	}
	d.mu.RUnlock()

	return nil
}

// ClearReceivedMessages 清空接收到的消息
func (d *DirectDevice) ClearReceivedMessages(topicPattern string) {
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

// matchTopic MQTT主题匹配(支持+和#通配符)
func matchTopic(topic, pattern string) bool {
	// 这里实现的是测试当前需要的最小 MQTT 通配符语义，而不是完整 broker 规范。
	// 完全匹配
	if pattern == topic {
		return true
	}

	// 分割主题和模式
	topicParts := splitTopic(topic)
	patternParts := splitTopic(pattern)

	// 处理 # 通配符
	if len(patternParts) > 0 && patternParts[len(patternParts)-1] == "#" {
		// # 必须是最后一个且匹配所有剩余层级
		patternParts = patternParts[:len(patternParts)-1]
		if len(topicParts) < len(patternParts) {
			return false
		}
		// 只比较 # 之前的部分
		topicParts = topicParts[:len(patternParts)]
	} else {
		// 没有 #,长度必须相等
		if len(topicParts) != len(patternParts) {
			return false
		}
	}

	// 逐层匹配
	for i := 0; i < len(patternParts); i++ {
		if patternParts[i] == "+" {
			// + 匹配任意单层
			continue
		}
		if patternParts[i] != topicParts[i] {
			return false
		}
	}

	return true
}

// splitTopic 分割主题
func splitTopic(topic string) []string {
	if topic == "" {
		return []string{}
	}

	var parts []string
	start := 0
	for i, c := range topic {
		if c == '/' {
			if i > start {
				parts = append(parts, topic[start:i])
			}
			start = i + 1
		}
	}
	if start < len(topic) {
		parts = append(parts, topic[start:])
	}

	return parts
}
