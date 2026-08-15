// 文件用途：维护 plugin\aetherlink\util\check_pub_topic.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package util

import (
	"strings"
)

// pubList 是允许设备发布（上行）的主题模式列表。
// '+' 表示单层通配符，匹配任意非空单层字符串。
var pubList = []string{
	"devices/telemetry",                 // 遥测上报
	"devices/status/+",                  // 设备上线/下线状态上报
	"devices/attributes/+",              // 属性上报
	"devices/event/+",                   // 事件上报
	"ota/devices/progress",              // 设备升级进度更新
	"devices/attributes/set/response/+", // 属性设置响应上报
	"devices/command/response/+",        // 命令响应上报

	"gateway/telemetry",                 // 设备遥测（网关）
	"gateway/attributes/+",              // 属性上报（网关）
	"gateway/event/+",                   // 事件上报（网关）
	"gateway/attributes/set/response/+", // 属性设置响应上报（网关）
	"gateway/command/response/+",        // 命令响应上报（网关）

	"devices/register",    // 网关子设备注册
	"devices/config/down", // 设备配置下载

	"+/up", // [REMOVED]一体机上行数据
}

// mqttWildcard 是 MQTT 单层通配符。
const mqttWildcard = "+"

// ValidateTopic 检查主题是否符合 pubList 中的任一模式。
func ValidateTopic(topic string) bool {
	for _, pattern := range pubList {
		if matchesPattern(topic, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern 检查主题是否符合给定模式（'+' 匹配单层）。
func matchesPattern(topic, pattern string) bool {
	topicParts := strings.Split(topic, "/")
	patternParts := strings.Split(pattern, "/")

	// 主题和模式层数不一致则不匹配
	if len(topicParts) != len(patternParts) {
		return false
	}

	// 逐层匹配：通配符 '+' 匹配任意单层，其余需完全相等
	for i := range topicParts {
		if patternParts[i] == mqttWildcard {
			if topicParts[i] == "" {
				return false
			}
			continue
		}
		if topicParts[i] != patternParts[i] {
			return false
		}
	}

	return true
}
