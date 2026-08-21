// 文件用途：定义设备 MQTT 标准下行订阅主题及其安全校验。
// 核心逻辑：区分标准主题候选、合法主题形状，并将设备身份槽绑定到认证设备编号。
// 安全职责：标准模板结构即使包含非法或空身份，也必须被识别为保留主题并拒绝 mapping 回退。

package util

import (
	"strings"
)

// subList 是允许设备订阅（下行）的主题模式列表。
// '{device_number}' 为设备编号占位符，'+' 为 MQTT 单层通配符。
var subList = []string{
	"devices/telemetry/control/{device_number}",   // 订阅平台下发的控制
	"devices/telemetry/control/{device_number}/+", // 订阅平台下发的控制
	"devices/attributes/set/{device_number}/+",    // 订阅平台下发的属性设置
	"devices/attributes/get/{device_number}",      // 订阅平台对属性的请求
	"devices/command/{device_number}/+",           // 订阅命令

	"ota/devices/inform/{device_number}", // 接收升级任务（固件升级相关）

	"devices/attributes/response/{device_number}/+", // 订阅平台收到属性的响应
	"devices/event/response/{device_number}/+",      // 接收平台收到事件的响应

	"gateway/telemetry/control/{device_number}", // 订阅平台下发的控制（网关）
	"gateway/attributes/set/{device_number}/+",  // 订阅平台下发的属性设置（网关）
	"gateway/attributes/get/{device_number}",    // 订阅平台对属性的请求（网关）
	"gateway/command/{device_number}/+",         // 订阅命令（网关）

	"gateway/attributes/response/{device_number}/+", // 订阅平台收到属性的响应（网关）
	"gateway/event/response/{device_number}/+",      // 接收平台收到事件的响应（网关）

	"{device_number}/down", // 兼容遗留接入方案的单层通配下行主题（历史设备固件使用）

	"devices/register/response/+",    // 网关子设备注册平台回复
	"devices/config/down/response/+", // 设备配置下载平台回复
}

// IsStandardSubTopicCandidate 检查主题是否占用标准模板结构。
// 占位槽内容即使为空或为通配符也仍是标准候选，由后续严格校验拒绝。
func IsStandardSubTopicCandidate(topic string) bool {
	for _, pattern := range subList {
		if matchesPatternSubStructure(topic, pattern) {
			return true
		}
	}
	return false
}

// ValidateSubTopic 检查主题是否符合 subList 中的任一模式。
func ValidateSubTopic(topic string) bool {
	for _, pattern := range subList {
		if matchesPatternSub(topic, pattern) {
			return true
		}
	}
	return false
}

func matchesPatternSubStructure(topic, pattern string) bool {
	topicParts := strings.Split(topic, "/")
	patternParts := strings.Split(pattern, "/")
	if len(topicParts) != len(patternParts) {
		return false
	}
	for i := range topicParts {
		if patternParts[i] != "{device_number}" && patternParts[i] != "+" && topicParts[i] != patternParts[i] {
			return false
		}
	}
	return true
}

// ValidateSubTopicForDevice 检查标准主题中的设备编号是否属于当前认证设备。
// 不含 {device_number} 身份槽的宽泛主题无法安全绑定设备，因此拒绝。
func ValidateSubTopicForDevice(topic, deviceNumber string) bool {
	if deviceNumber == "" {
		return false
	}
	for _, pattern := range subList {
		if matchesPatternSubForDevice(topic, pattern, deviceNumber) {
			return true
		}
	}
	return false
}

func matchesPatternSubForDevice(topic, pattern, deviceNumber string) bool {
	if !strings.Contains(pattern, "{device_number}") || !matchesPatternSub(topic, pattern) {
		return false
	}

	topicParts := strings.Split(topic, "/")
	patternParts := strings.Split(pattern, "/")
	for i := range patternParts {
		if patternParts[i] == "{device_number}" && topicParts[i] != deviceNumber {
			return false
		}
	}
	return true
}

// matchesPatternSub 检查主题是否符合给定模式。
// '{device_number}' 占位符不能匹配 '+' 或 '#'；'+' 不能匹配 '#'；其余需完全相等。
func matchesPatternSub(topic, pattern string) bool {
	topicParts := strings.Split(topic, "/")
	patternParts := strings.Split(pattern, "/")

	// 主题和模式层数不一致则不匹配
	if len(topicParts) != len(patternParts) {
		return false
	}

	// 逐层匹配
	for i := range topicParts {
		switch patternParts[i] {
		case "{device_number}":
			// {device_number} 部分不能是 + 或 #
			if topicParts[i] == "" || topicParts[i] == "+" || topicParts[i] == "#" {
				return false
			}
		case "+":
			// + 部分不可以是 # 通配符，可以是其他任意字符包括 + 通配符
			if topicParts[i] == "" || topicParts[i] == "#" {
				return false
			}
		default:
			// 其他部分必须相等
			if topicParts[i] != patternParts[i] {
				return false
			}
		}
	}

	return true
}
