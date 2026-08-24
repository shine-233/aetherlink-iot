// 文件用途：收敛后端进程内部拨号 MQTT Broker 地址的解析与归一化逻辑。
// 核心逻辑：剥掉 tcp:// 前缀后拆出 host/port；host 为空或回环地址时依次回退 GOTP_MQTT_BROKER、AETHERLINK_MQTT_INNER_BROKER；最终把 localhost 归一化为 127.0.0.1，规避 net.Dial 按 RFC6724 偏好 ::1 而 broker 只监听 IPv4 导致的拒连。
// 关键注意事项：只对回环/空地址做改写，非回环地址（如 mqtt-broker:1883）原样返回，宿主机直连场景不受影响；env 值自带端口时覆盖原端口，否则保留原端口。
// 重构建议：新增后端进程内的 broker 拨号点必须复用本助手，不要再内联 viper/env 回退判断。
package utils

import (
	"net"
	"os"
	"strings"
)

const (
	mqttBrokerEnvKeyGOTP  = "GOTP_MQTT_BROKER"
	mqttBrokerEnvKeyInner = "AETHERLINK_MQTT_INNER_BROKER"
)

// mqttBrokerEnvLookup 抽象进程环境读取，便于单元测试注入假 lookup。
var mqttBrokerEnvLookup = os.Getenv

// ResolveMQTTBrokerDialAddress 返回后端进程内部拨号 broker 应使用的 host:port。
func ResolveMQTTBrokerDialAddress(configured string) string {
	return resolveMQTTBrokerDialAddress(configured, mqttBrokerEnvLookup)
}

// resolveMQTTBrokerDialAddress 是纯函数核心：输入配置地址与 env lookup，输出归一化拨号地址。
func resolveMQTTBrokerDialAddress(configured string, lookup func(string) string) string {
	host, port := splitMQTTBrokerHostPort(configured)
	if isLoopbackMQTTBrokerHost(host) {
		for _, envKey := range []string{mqttBrokerEnvKeyGOTP, mqttBrokerEnvKeyInner} {
			envValue := strings.TrimSpace(lookup(envKey))
			if envValue == "" {
				continue
			}
			envHost, envPort := splitMQTTBrokerHostPort(envValue)
			if envHost != "" {
				host = envHost
			}
			if envPort != "" {
				port = envPort
			}
			break
		}
	}
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		host = "127.0.0.1"
	}
	if port == "" {
		return host
	}
	return net.JoinHostPort(host, port)
}

// splitMQTTBrokerHostPort 拆出 broker 地址中的 host 与 port；无端口（含裸 IPv6 字面量）时 port 为空串。
func splitMQTTBrokerHostPort(address string) (string, string) {
	address = strings.TrimSpace(address)
	if len(address) >= len("tcp://") && strings.EqualFold(address[:len("tcp://")], "tcp://") {
		address = strings.TrimSpace(address[len("tcp://"):])
	}
	if host, port, err := net.SplitHostPort(address); err == nil {
		return strings.Trim(host, "[]"), port
	}
	return strings.Trim(address, "[]"), ""
}

// isLoopbackMQTTBrokerHost 判断 host 是否为空或指向本机（localhost/127.0.0.1/::1）。
func isLoopbackMQTTBrokerHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
