// 文件用途：提供虚拟传感器使用的 MQTT 客户端创建逻辑。
// 核心逻辑：构造 Paho MQTT 客户端，配置自动重连和连接回调，支撑模拟设备持续发布消息。
// 静态审查建议：broker 地址、用户名和密码应来自本地环境配置，不建议写死到公共仓库。
package main

import (
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-basic/uuid"
)

type MqttConfig struct {
	Broker string
	User   string
	Pass   string
}

// CreateMqttClient 创建并连接 MQTT 客户端，供虚拟传感器模拟设备流量使用。
// 静态审查重点：该函数包含重连逻辑，若 broker 不可达会持续等待，调用方要接受阻塞行为。
func CreateMqttClient(config MqttConfig) *mqtt.Client {
	// 初始化配置
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetUsername(config.User)
	if config.Pass != "" {
		opts.SetPassword(config.Pass)
	}
	opts.SetClientID(uuid.New())
	// 使用干净会话
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	// 恢复订阅依赖 broker 支持
	opts.SetResumeSubs(false)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(20 * time.Second)
	// 保持消息顺序宽松，减少模拟压测时的阻塞
	opts.SetOrderMatters(false)
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		log.Println("mqtt connect success")
	})
	// 断线重连
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Println("mqtt connect  lost: ", err)
		// 等待连接成功，失败则重新连接
		for {
			token := client.Connect()
			if token.Wait() && token.Error() == nil {
				log.Println("Reconnected to MQTT broker")
				break
			}
			log.Printf("Reconnect failed: %v\n", token.Error())
			time.Sleep(5 * time.Second)
		}
	})
	mqttClient := mqtt.NewClient(opts)
	for {
		if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.Println(token.Error())
			time.Sleep(15 * time.Second)
		} else {
			break
		}
	}
	return &mqttClient
}
