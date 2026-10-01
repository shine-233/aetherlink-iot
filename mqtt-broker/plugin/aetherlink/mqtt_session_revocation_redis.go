// 文件用途：把 Redis Pub/Sub 的设备会话撤销命令与处理 ACK 适配到 broker 内部 monitor。
// 核心逻辑：订阅固定控制 channel，并把结构化 processing ACK 发布到专用 :ack channel。
// 关键注意事项：go-redis v5 的 ReceiveMessage 会自动处理网络重连/重订阅；其它错误会记录并退避重试，channel 修改必须两侧同步。
package aetherlink

import (
	"encoding/json"
	"fmt"
)

const (
	mqttDeviceSessionRevocationChannel    = "aetherlink:mqtt:device-session:terminate"
	mqttDeviceSessionRevocationAckChannel = mqttDeviceSessionRevocationChannel + ":ack"
)

func publishRedisMQTTSessionRevocationAck(ack mqttSessionRevocationAck) error {
	if redisCache == nil {
		return fmt.Errorf("redis is not initialized for mqtt session revocation acknowledgement")
	}
	payload, err := json.Marshal(ack)
	if err != nil {
		return fmt.Errorf("encode mqtt session revocation acknowledgement: %w", err)
	}
	subscriberCount, err := redisCache.Publish(mqttDeviceSessionRevocationAckChannel, string(payload)).Result()
	if err != nil {
		return fmt.Errorf("publish mqtt session revocation acknowledgement: %w", err)
	}
	if subscriberCount == 0 {
		return fmt.Errorf("publish mqtt session revocation acknowledgement: no backend subscriber")
	}
	return nil
}

func subscribeRedisMQTTSessionRevocations() (mqttSessionRevocationSubscription, error) {
	subscription, err := subscribeRedisChannel(mqttDeviceSessionRevocationChannel, "mqtt session revocation")
	if err != nil {
		return nil, err
	}
	return subscription, nil
}
