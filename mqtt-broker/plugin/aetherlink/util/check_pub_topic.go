// 文件用途：定义设备 MQTT 标准上行发布主题白名单及其设备身份绑定校验。
// 核心逻辑：在形状匹配之上，把携带设备身份的槽位绑定到当前认证设备的身份，
//
//	与订阅侧 check_sub_topic.go 的 ValidateSubTopicForDevice 保持对称。
//
// 安全职责：devices/status 槽位承载平台设备 ID 且后端直接信任该层身份，
// 任意已认证设备不得为其它设备伪造状态；'+/up' 与订阅侧 '{device_number}/down'
// 平行，首层按设备编号绑定。message_id 类槽位不是设备身份，不做绑定，
// 消息归因由 hooks_messages.go 的 payload 重包（device_id=发布者自身）兜底。

package util

import (
	"strings"
)

// pubIdentityKind 标识身份槽需要绑定的认证设备身份类型。
type pubIdentityKind int

const (
	// identityNone 表示该模式不含可与认证设备绑定的身份槽。
	identityNone pubIdentityKind = iota
	// identityDeviceID 表示槽位必须等于发布者自身的平台设备 ID。
	identityDeviceID
	// identityDeviceNumber 表示槽位必须等于发布者自身的设备编号。
	identityDeviceNumber
)

// pubTopicPattern 是允许设备发布（上行）的主题模式及可选的设备身份槽绑定。
type pubTopicPattern struct {
	pattern      string
	identityKind pubIdentityKind
}

// slotIndex 返回模式中 '+' 通配层的下标；无 '+' 时返回 -1。
func (p pubTopicPattern) slotIndex() int {
	for i, part := range strings.Split(p.pattern, "/") {
		if part == mqttWildcard {
			return i
		}
	}
	return -1
}

// pubList 是允许设备发布（上行）的主题模式列表。
// '+' 表示单层通配符，匹配任意非空单层字符串。
var pubList = []pubTopicPattern{
	{pattern: "devices/telemetry"},                                // 遥测上报
	{pattern: "devices/status/+", identityKind: identityDeviceID}, // 状态上报：槽位绑定发布者设备 ID
	{pattern: "devices/attributes/+"},                             // 属性上报（槽位为 message_id）
	{pattern: "devices/event/+"},                                  // 事件上报（槽位为 message_id）
	{pattern: "ota/devices/progress"},                             // 设备升级进度更新
	{pattern: "devices/attributes/set/response/+"},                // 属性设置响应上报（槽位为 message_id）
	{pattern: "devices/command/response/+"},                       // 命令响应上报（槽位为 message_id）

	{pattern: "gateway/telemetry"},                 // 设备遥测（网关）
	{pattern: "gateway/attributes/+"},              // 属性上报（网关，槽位为 message_id）
	{pattern: "gateway/event/+"},                   // 事件上报（网关，槽位为 message_id）
	{pattern: "gateway/attributes/set/response/+"}, // 属性设置响应上报（网关，槽位为 message_id）
	{pattern: "gateway/command/response/+"},        // 命令响应上报（网关，槽位为 message_id）

	{pattern: "devices/register", identityKind: identityNone},    // 网关子设备注册（子设备信息在 payload 内，归因仍为网关自身）
	{pattern: "devices/config/down", identityKind: identityNone}, // 设备配置下载

	{pattern: "+/up", identityKind: identityDeviceNumber}, // 心智悦喷淋一体机上行数据：首层绑定发布者设备编号
}

// mqttWildcard 是 MQTT 单层通配符。
const mqttWildcard = "+"

// ValidateTopic 检查主题是否符合 pubList 中的任一模式（仅形状校验）。
func ValidateTopic(topic string) bool {
	for _, p := range pubList {
		if matchesPattern(topic, p.pattern) {
			return true
		}
	}
	return false
}

// ValidatePubTopicForDevice 在形状校验之上绑定设备身份，与订阅侧
// ValidateSubTopicForDevice 对称：
//   - devices/status 的 '+' 槽位是平台设备 ID，后端从 topic 解析并信任该身份，
//     因此槽位必须等于发布者自身设备 ID，防止跨设备伪造上下线状态；
//   - '+/up' 首层与订阅侧 '{device_number}/down' 对应，必须等于发布者设备编号；
//   - 其余 '+' 槽位是平台下发的 message_id 或无身份槽的共享主题，
//     无法与设备身份绑定，保持形状校验并由 payload 重包保证归因；
//   - 形状不匹配、或所需身份缺失/不一致时一律拒绝（fail-closed）。
func ValidatePubTopicForDevice(topic, deviceID, deviceNumber string) bool {
	for _, p := range pubList {
		if !matchesPattern(topic, p.pattern) {
			continue
		}
		if !pubTopicIdentitySatisfied(p, topic, deviceID, deviceNumber) {
			continue
		}
		return true
	}
	return false
}

// pubTopicIdentitySatisfied 检查主题是否满足模式的设备身份槽绑定约束。
func pubTopicIdentitySatisfied(p pubTopicPattern, topic, deviceID, deviceNumber string) bool {
	switch p.identityKind {
	case identityNone:
		return true
	case identityDeviceID:
		if deviceID == "" {
			return false
		}
		return topicSlotValue(topic, p.slotIndex()) == deviceID
	case identityDeviceNumber:
		if deviceNumber == "" {
			return false
		}
		return topicSlotValue(topic, p.slotIndex()) == deviceNumber
	default:
		return false
	}
}

// topicSlotValue 返回主题指定层的内容；下标越界时返回空串。
func topicSlotValue(topic string, index int) string {
	parts := strings.Split(topic, "/")
	if index < 0 || index >= len(parts) {
		return ""
	}
	return parts[index]
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
