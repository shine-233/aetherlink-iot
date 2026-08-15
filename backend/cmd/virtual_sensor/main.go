// 文件用途：提供本地虚拟传感器启动入口，用于联调 MQTT 设备数据流。
// 核心逻辑：启动温湿度模拟器并保持进程常驻，让虚拟设备持续对外发布消息。
// 静态审查建议：该命令适合本地调试，不应把真实 broker 地址、凭证和设备标识直接提交到公开配置。
package main

import (
	"strconv"
	"time"
)

// main 负责启动虚拟温湿度传感器并阻塞进程，保证模拟发布持续运行。
// 静态审查重点：这里不应加入额外业务逻辑，避免把调试入口变成生产控制路径。
func main() {
	go TempHumSensor()
	select {}
}

// GetMessageID 生成消息 ID，用于属性和事件类消息的 topic 后缀。
// 静态审查重点：该实现依赖当前时间戳截断，适合本地调试，不保证跨进程全局唯一。
func GetMessageID() string {
	// 获取当前 Unix 时间戳
	timestamp := time.Now().Unix()
	// 将时间戳转换为字符串
	timestampStr := strconv.FormatInt(timestamp, 10)
	// 截取后 7 位作为消息 ID
	messageID := timestampStr[len(timestampStr)-7:]

	return messageID
}
