// 文件用途：维护 script\main.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package main

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// mqtt服务器地址
var BROKEN string = "127.0.0.1:1883"

// 每隔多久发送一次消息，单位s
var LOOP_TIME int = 5

// 模拟设备的数量
var DEVICE_NUM int = 1000

// 发送主题
var TOPIC string = "device/attributes"

// 发送的数据
var PAYLOAD string = `{"temperature": 20, "humidity": 60}`

// 消息质量
var QOS int = 0

func main() {
	fmt.Println("开始执行脚本...")
	// 循环创建mqtt客户端并每个客户端循环发送消息
	MqttPublishLoopClient(TOPIC, PAYLOAD, QOS)
}

// 新增mqtt客户端连接
func MqttClient(clientId string) (mqtt.Client, error) {
	// 掉线重连
	var connectLostHandler mqtt.ConnectionLostHandler = func(c mqtt.Client, err error) {
		fmt.Printf("（"+clientId+"）Mqtt Connect lost: %v", err)
		i := 0
		for {
			time.Sleep(5 * time.Second)
			if !c.IsConnectionOpen() {
				i++
				fmt.Println("（"+clientId+"）Mqtt客户端掉线重连...", i)
				if token := c.Connect(); token.Wait() && token.Error() != nil {
					fmt.Println("（" + clientId + "）Mqtt客户端连接失败...")
				} else {
					break
				}
			} else {
				//subscribe(msgProc1, gatewayMsgProc)
				break
			}
		}
	}
	opts := mqtt.NewClientOptions()
	opts.SetClientID(clientId)
	opts.AddBroker(BROKEN)
	opts.SetAutoReconnect(true)
	opts.SetOrderMatters(false)
	opts.OnConnectionLost = connectLostHandler
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		fmt.Println("Mqtt客户端已连接（" + clientId + "）")
	})
	reconnec_number := 0
	c := mqtt.NewClient(opts)
	// 异步建立连接，失败重连
	for {

		if token := c.Connect(); token.Wait() && token.Error() != nil {
			reconnec_number++
			fmt.Println("链接错误错误说明：", token.Error().Error())
			fmt.Println("Mqtt客户端连接失败（"+clientId+"）...重试", reconnec_number)
		} else {
			MqttPublishLoop(TOPIC, PAYLOAD, QOS, c)
			fmt.Println("Mqtt客户端连接成功（" + clientId + "）")
			break
		}
		time.Sleep(5 * time.Second)
	}
	return c, nil
	// 1.连接mqtt服务器
	// 2.发布消息
	// 3.断开连接
}

// 发送mqtt消息
func MqttPublish(topic string, payload string, qos int, c mqtt.Client) {
	cc := c.OptionsReader()
	token := c.Publish(topic, byte(qos), false, payload)
	token.Wait()
	fmt.Printf("%s发送消息成功，topic:%s, payload:%s\n", cc.ClientID(), topic, payload)
}

// 循环发送mqtt消息
func MqttPublishLoop(topic string, payload string, qos int, c mqtt.Client) {
	for {
		MqttPublish(topic, payload, qos, c)
		time.Sleep(time.Duration(LOOP_TIME) * time.Second)
	}
}

// 循环创建mqtt客户端并循环发送消息
func MqttPublishLoopClient(topic string, payload string, qos int) {
	//循环生成100个clientId
	for i := 0; i < DEVICE_NUM; i++ {
		clientId := fmt.Sprintf("client_%d", i)
		go MqttClient(clientId)
	}
	time.Sleep(100 * time.Second)
}
