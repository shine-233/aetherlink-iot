// 文件用途：生成虚拟温湿度传感器的模拟 payload，并通过 MQTT 对外发布。
// 核心逻辑：构造遥测、属性和事件消息，配合 MQTT 客户端完成本地联调。
// 静态审查建议：topic、设备 ID、broker 地址和消息形状都属于联调约定，修改前要和后端协议同步确认。
package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	mqttClient   *mqtt.Client
	switchStatus int64 = 0 // 记录模拟开关状态，供遥测消息引用。
)

// TempHumSensor 启动虚拟温湿度传感器主流程，包括连接、订阅和发布。
// 静态审查重点：该函数会持续运行并启动多个发布协程，适合本地调试，不适合直接放入生产路径。
func TempHumSensor() {
	// 创建 mqtt 客户端
	createClient()

	// 订阅控制消息
	subscribeControlMessage()

	// 发布遥测消息
	go publishTelemetryMessage("devices/telemetry")
	// 发布属性消息
	go publishAttributeMessage("devices/attributes/")
	// 发布事件消息
	go publishEventMessage("devices/event/")

	select {}
}

// createClient 创建虚拟传感器使用的 MQTT 客户端。
// 静态审查重点：broker、用户名和密码为本地调试配置，不建议硬编码到共享环境。
func createClient() {
	// 初始化配置
	opts := MqttConfig{
		Broker: "127.0.0.1:1883",
		User:   "sensor1",
		Pass:   "",
	}
	mqttClient = CreateMqttClient(opts)
}

// subscribeControlMessage 订阅控制消息，并根据 switchStatus 的变化回发遥测消息。
// 静态审查重点：topic 必须和后端控制链路保持一致，否则控制消息会订阅不到。
func subscribeControlMessage() {
	topic := "devices/telemetry/control/sensor1"
	token := (*mqttClient).Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		var controlMsg map[string]interface{}
		err := json.Unmarshal(msg.Payload(), &controlMsg)
		if err != nil {
			log.Printf("解析控制消息失败: %v", err)
			return
		}

		// 检查是否包含 switchStatus
		if status, ok := controlMsg["switchStatus"].(float64); ok {
			switchStatus = int64(status)
			log.Printf("收到开关控制命令 %d", switchStatus)

			// 立即发送开关状态变化的遥测消息
			message := make(map[string]interface{})
			message["switchStatus"] = switchStatus
			payload, err := json.Marshal(message)
			if err != nil {
				log.Printf("生成遥测消息失败: %v", err)
				return
			}
			token := (*mqttClient).Publish("devices/telemetry", 0, false, payload)
			token.Wait()
			log.Printf("已发送开关状态变更遥测 %d", switchStatus)
		}
	})
	token.Wait()
	log.Printf("已订阅控制主题 %s", topic)
}

// publishTelemetryMessage 持续发布温湿度遥测消息。
// 静态审查重点：发布频率、字段类型和 topic 结构应与后端设备遥测协议一致。
func publishTelemetryMessage(topic string) {
	// 每隔一段时间发布一次消息
	for {
		message := make(map[string]interface{})
		// 生成温度值并保留两位小数
		t, err := generateRandomFloat()
		if err != nil {
			log.Println("generateRandomFloat failed:", err)
		}
		message["temperature"] = t
		message["temperature"] = float64(int(message["temperature"].(float64)*100)) / 100
		// 生成湿度值
		h, err := generateRandomFloat()
		if err != nil {
			log.Println("generateRandomFloat failed:", err)
		}
		message["humidity"] = h

		// 添加在线状态
		message["isOnline"] = true
		// 添加开关状态
		message["switchStatus"] = switchStatus

		// 添加设备状态字符串
		statuses := []string{"running", "idle", "error", "maintenance"}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(statuses))))
		if err != nil {
			log.Println("generate random number failed:", err)
			message["deviceStatus"] = "running"
		} else {
			message["deviceStatus"] = statuses[n.Int64()]
		}

		// 转换为 json 格式
		var payload []byte
		payload, err = json.Marshal(message)
		if err != nil {
			log.Println("json.Marshal failed:", err)
			return
		}
		token := (*mqttClient).Publish(topic, 0, false, payload)
		isSuccess := token.Wait()
		if !isSuccess {
			log.Println("Publish message failed", string(payload))
		} else {
			log.Println("Publish message successful:", string(payload))
		}
		// 每隔 30 秒发布一次消息
		<-time.After(30 * time.Second)
	}
}

// publishAttributeMessage 持续发布属性消息。
// 静态审查重点：属性 topic 带有消息 ID 后缀时，要确认后端是否按同样规则消费。
func publishAttributeMessage(topic string) {
	// 每隔一段时间发布一次消息
	for {
		message := make(map[string]interface{})
		message["version"] = "1.0.0"
		message["status"] = "normal"
		message["mac"] = "00:11:22:33:44:55"
		// 转换为 json 格式
		var payload []byte
		payload, err := json.Marshal(message)
		if err != nil {
			log.Println("json.Marshal failed:", err)
			return
		}
		messageId := GetMessageID()
		token := (*mqttClient).Publish(topic+messageId, 0, false, payload)
		isSuccess := token.Wait()
		if !isSuccess {
			log.Println("Publish message failed", string(payload))
		} else {
			log.Println("Publish message successful:", string(payload))
		}
		// 每隔 120 秒发布一次消息
		<-time.After(120 * time.Second)
	}
}

// publishEventMessage 持续发布事件消息。
// 静态审查重点：事件主题和 payload 中的 method/params 字段，是后端事件解析链路的关键约定。
func publishEventMessage(topic string) {
	// 每隔一段时间发布一次消息
	for {
		message := make(map[string]interface{})

		message["method"] = "alert"
		// params 使用 map 结构
		message["params"] = map[string]interface{}{
			"level":   "warning",
			"message": "temperature is too high",
		}
		// 转换为 json 格式
		var payload []byte
		payload, err := json.Marshal(message)
		if err != nil {
			log.Println("json.Marshal failed:", err)
			return
		}
		messageId := GetMessageID()
		token := (*mqttClient).Publish(topic+messageId, 0, false, payload)
		isSuccess := token.Wait()
		if !isSuccess {
			log.Println("Publish message failed", string(payload))
		} else {
			log.Println("Publish message successful:", string(payload))
		}
		// 每隔 120 秒发布一次消息
		<-time.After(120 * time.Second)
	}
}

// generateRandomFloat 生成一个带两位小数的随机数。
// 静态审查重点：返回范围由实现固定，若后端校验区间变化，需要同步调整这里的取值策略。
func generateRandomFloat() (float64, error) {
	// 生成整数部分 [10, 99]
	integer, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return 0, fmt.Errorf("生成整数部分失败: %v", err)
	}
	integer = integer.Add(integer, big.NewInt(10))

	// 生成小数部分 [0, 99]
	decimal, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return 0, fmt.Errorf("生成小数部分失败: %v", err)
	}

	// 组合整数和小数部分
	result := float64(integer.Int64()) + float64(decimal.Int64())/100.0

	return result, nil
}
