// 文件用途：平台上报通道（ROADMAP B1）。
// 核心逻辑：每台设备一条持久 MQTT 连接（以设备自身凭证接入），发布遥测快照；
//   订阅 devices/command/{number}/+ 接收平台命令下发，映射到可写寄存器写入。
// 关键注意事项：payload 为 {key: value} JSON，与平台遥测上行契约一致；
//   命令负载沿用 PutMessageForCommand 形状 {identify, value}；连接断开由 paho 自动重连。
package reporter

import (
	"encoding/json"
	"fmt"
	"net"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
)

// CommandWriter 单条命令的执行回调（由 poller/客户端侧提供）。
type CommandWriter func(key string, value float64) error

// DeviceReporter 单设备上报与命令订阅。
type DeviceReporter struct {
	cfg    config.DeviceConfig
	global config.MQTTConfig
	client mqtt.Client
	logger *logrus.Logger
}

// NewDeviceReporter 创建并连接设备级 MQTT 客户端。
func NewDeviceReporter(globalCfg config.MQTTConfig, device config.DeviceConfig, logger *logrus.Logger) (*DeviceReporter, error) {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	broker := net.JoinHostPort(globalCfg.Host, fmt.Sprint(globalCfg.Port))
	clientID := device.ClientID
	if clientID == "" {
		clientID = "modbus-plugin-" + device.DeviceNumber
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetUsername(device.Username)
	opts.SetPassword(device.Password)
	opts.SetClientID(clientID)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetOrderMatters(false)
	r := &DeviceReporter{cfg: device, global: globalCfg, logger: logger}
	r.client = mqtt.NewClient(opts)
	if token := r.client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect %s as %s: %w", broker, device.Username, token.Error())
	}
	return r, nil
}

// PublishTelemetry 发布一轮采集快照。
func (r *DeviceReporter) PublishTelemetry(deviceNumber string, values map[string]any) error {
	payload, err := json.Marshal(values)
	if err != nil {
		return err
	}
	token := r.client.Publish(r.global.TelemetryTopic, 0, false, payload)
	if !token.WaitTimeout(5*1000*1000*1000) || token.Error() != nil {
		if token.Error() != nil {
			return token.Error()
		}
		return fmt.Errorf("telemetry publish timeout")
	}
	return nil
}

// SubscribeCommands 订阅平台命令下发并把 {identify,value} 写入可写寄存器。
func (r *DeviceReporter) SubscribeCommands(write CommandWriter) error {
	topic := fmt.Sprintf("%s/%s/+", r.global.CommandTopicPrefix, r.cfg.DeviceNumber)
	handler := func(_ mqtt.Client, msg mqtt.Message) {
		var payload struct {
			Identify string          `json:"identify"`
			Value    json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
			r.logger.WithError(err).WithField("topic", msg.Topic()).Warn("command payload is not valid json")
			return
		}
		register, ok := r.cfg.FindWritable(payload.Identify)
		if !ok || register == nil {
			r.logger.WithField("identify", payload.Identify).Warn("no writable register for command identify")
			return
		}
		value, err := coerceNumber(payload.Value)
		if err != nil {
			r.logger.WithError(err).WithField("identify", payload.Identify).Warn("command value must be a number or bool")
			return
		}
		if err := write(register.Key, value); err != nil {
			r.logger.WithError(err).WithField("identify", payload.Identify).Warn("modbus write failed")
		} else {
			r.logger.WithField("identify", payload.Identify).Info("modbus write executed")
		}
	}
	token := r.client.Subscribe(topic, 0, handler)
	if !token.WaitTimeout(5*1000*1000*1000) || token.Error() != nil {
		if token.Error() != nil {
			return token.Error()
		}
		return fmt.Errorf("command subscribe timeout")
	}
	return nil
}

// Close 断开连接。
func (r *DeviceReporter) Close() {
	r.client.Disconnect(250)
}

func coerceNumber(raw json.RawMessage) (float64, error) {
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return num, nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if b {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("value is neither number nor bool")
}
