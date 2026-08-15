// 文件用途：封装 MQTT adapter 使用的客户端连接能力。
// 核心逻辑：基于 Paho MQTT client 管理连接配置、生命周期和日志输出。
// 关键注意事项：连接参数和重连行为会影响设备接入稳定性，修改需关注 broker 兼容与关闭顺序。
// 重构建议：可抽出最小 MQTT client 接口，便于用 fake client 覆盖连接失败和重试分支。

package mqttadapter

import (
	"fmt"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

const (
	mqttAdapterInitialConnectTimeout       = 10 * time.Second
	mqttAdapterInitialConnectWaitTimeout   = 12 * time.Second
	mqttAdapterInitialConnectAttempts      = 3
	mqttAdapterInitialConnectRetryInterval = 2 * time.Second
	mqttAdapterOperationTimeout            = 10 * time.Second
)

// MQTTConfig MQTT 客户端配置
type MQTTConfig struct {
	Broker            string
	Username          string
	Password          string
	ClientID          string                   // 可选，不提供则自动生成
	OnConnectCallback func(client mqtt.Client) // 连接成功回调（用于重新订阅）
}

// CreateMQTTClient 创建 MQTT 客户端（Adapter 专用）。
// 首次连接使用有限次数和单次 deadline；后续掉线仍由 Paho 自动重连。
// 这使应用启动失败能够返回 ServiceManager，而不是永久卡在尚未登记的 service 中。
func CreateMQTTClient(config MQTTConfig, logger *logrus.Logger) (mqtt.Client, error) {
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	// 初始化配置
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)

	// 客户端 ID
	clientID := config.ClientID
	if clientID == "" {
		clientID = "aetherlink-adapter-default"
	}
	opts.SetClientID(clientID)

	// 干净会话
	opts.SetCleanSession(false)
	// 恢复客户端订阅，需要 broker 支持
	opts.SetResumeSubs(true)
	// 自动重连
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(mqttAdapterInitialConnectTimeout)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(200 * time.Second)
	// 消息顺序
	opts.SetOrderMatters(false)

	configureMQTTClientCallbacks(opts, clientID, logger, config.OnConnectCallback)

	// 创建客户端
	client := mqtt.NewClient(opts)

	// 首次连接必须有明确上限。常驻重连只在首次成功后交给 Paho。
	var connectErr error
	for attempt := 1; attempt <= mqttAdapterInitialConnectAttempts; attempt++ {
		token := client.Connect()
		connectErr = waitMQTTToken(token, mqttAdapterInitialConnectWaitTimeout)
		if connectErr == nil {
			logger.WithFields(logrus.Fields{
				"client_id": clientID,
				"attempt":   attempt,
			}).Info("MQTT Adapter client created and connected")
			return client, nil
		}
		logger.WithError(connectErr).WithFields(logrus.Fields{
			"client_id": clientID,
			"attempt":   attempt,
			"attempts":  mqttAdapterInitialConnectAttempts,
		}).Error("MQTT Adapter initial connection failed")
		if attempt < mqttAdapterInitialConnectAttempts {
			time.Sleep(mqttAdapterInitialConnectRetryInterval)
		}
	}
	DisconnectMQTTClient(client, logger)
	return nil, fmt.Errorf("mqtt adapter initial connection failed after %d attempts: %w", mqttAdapterInitialConnectAttempts, connectErr)
}

// configureMQTTClientCallbacks keeps the initial subscription pass owned by
// the caller. Paho invokes OnConnect for the first successful Connect as well
// as for reconnects; running the resubscribe callback for both events races
// the subscriptions that MQTTService installs immediately after this function
// returns.
func configureMQTTClientCallbacks(
	opts *mqtt.ClientOptions,
	clientID string,
	logger *logrus.Logger,
	onReconnect func(mqtt.Client),
) {
	var initialConnection atomic.Bool
	initialConnection.Store(true)

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		logger.WithField("client_id", clientID).Info("MQTT Adapter client connected")
		if initialConnection.CompareAndSwap(true, false) {
			return
		}

		if onReconnect != nil {
			logger.Info("Executing OnConnectCallback to re-subscribe topics...")
			onReconnect(client)
		}
	})

	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		logger.WithError(err).Warn("MQTT Adapter connection lost, auto-reconnect will handle it...")
	})

	opts.SetReconnectingHandler(func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		logger.Info("MQTT Adapter reconnecting...")
	})
}

func waitMQTTToken(token mqtt.Token, timeout time.Duration) error {
	if token == nil {
		return fmt.Errorf("mqtt operation returned a nil token")
	}
	if timeout <= 0 {
		timeout = mqttAdapterOperationTimeout
	}
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("mqtt operation timed out after %s", timeout)
	}
	return token.Error()
}

// DisconnectMQTTClient 断开 MQTT 客户端连接。
// 仅在连接已建立时执行断开，避免重复关闭引发额外日志噪声。
func DisconnectMQTTClient(client mqtt.Client, logger *logrus.Logger) {
	if client != nil && client.IsConnected() {
		client.Disconnect(250)
		if logger != nil {
			logger.Info("MQTT Adapter client disconnected")
		}
	}
}
