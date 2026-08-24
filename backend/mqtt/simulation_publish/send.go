// 文件用途：提供模拟设备向 MQTT Broker 发布单条消息的辅助入口。
// 核心逻辑：根据调用参数临时创建 MQTT 客户端，连接 Broker 后将字符串 payload 发布到指定 topic。
// 关键注意事项：该模块面向模拟和调试场景，生产路径不要依赖调用者传入的明文账号和任意 topic。
// 重构建议：建议后续抽出参数校验和上下文超时，避免 Broker 不可达时阻塞业务验证流程。
package simulationpublish

import (
	"net"

	"aetherlink-iot/backend/pkg/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

// 发布一次消息
func PublishMessage(host string, port string, topic string, payload string, username string, password string, clientId string) error {
	// 拨号前统一归一化回环地址：仅空/回环 host 触发 env 回退与 localhost→127.0.0.1，非回环原样放行以兼容宿主机场景。
	// 助手返回值带端口（来自 env）时一并采纳；仅归一化 host 时保留调用方传入的端口。
	if resolved := utils.ResolveMQTTBrokerDialAddress(host); resolved != "" {
		if resolvedHost, resolvedPort, splitErr := net.SplitHostPort(resolved); splitErr == nil && resolvedPort != "" {
			host, port = resolvedHost, resolvedPort
		} else {
			host = resolved
		}
	}
	opts := buildSimulationClientOptions(host, port, username, password, clientId)
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		logrus.Println("simulation mqtt connect success")
	})
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		logrus.Error("simulation MQTT Broker connection failed")
		return token.Error()
	}
	defer c.Disconnect(250)
	logrus.Debug("simulation MQTT publish request prepared")
	token := c.Publish(topic, 0, false, simulationPayload(payload))
	if token.Wait() && token.Error() != nil {
		logrus.Error("simulation MQTT Broker publish failed")
		return token.Error()
	}
	return nil
}

func buildSimulationClientOptions(host string, port string, username string, password string, clientID string) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(net.JoinHostPort(host, port))
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetClientID(clientID)
	opts.SetCleanSession(true)
	opts.SetOrderMatters(false)
	opts.SetResumeSubs(false)
	return opts
}

func simulationPayload(payload string) []byte {
	return []byte(payload)
}
