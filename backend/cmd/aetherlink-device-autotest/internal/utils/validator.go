// 文件用途：提供自动测试断言所需的数据、响应和时间戳校验工具。
// 核心逻辑：比较期望上报值与数据库字段指针，解析事件 JSON，校验平台响应 result，并按容忍窗口检查时间戳。
// 关键注意事项：数字比较只处理当前测试数据形态；ParseMessageFromTopic 只覆盖当前 MQTT 测试 topic 的单层通配契约。
// 重构建议：可把类型归一化、事件结构解析和 topic message_id 提取拆成可单测的小函数，并补齐负例覆盖。

/*
Purpose: 提供自动测试断言所需的数据、响应和时间戳校验工具。
Core logic: 比较期望上报值与数据库字段指针，解析事件 JSON，校验平台响应 result，并按容忍窗口检查时间戳。
Important notes: 数字比较只处理当前测试数据形态；ParseMessageFromTopic 只覆盖当前 MQTT 测试 topic 的单层通配契约。
Refactor suggestion: 可把类型归一化、事件结构解析和 topic message_id 提取拆成可单测的小函数，并补齐负例覆盖。
*/
package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// ValidateTelemetryValue 验证遥测数据值
func ValidateTelemetryValue(expected, actual interface{}) error {
	switch exp := expected.(type) {
	case bool:
		act, ok := actual.(*bool)
		if !ok || act == nil {
			return fmt.Errorf("expected bool value, got %T", actual)
		}
		if *act != exp {
			return fmt.Errorf("value mismatch: expected %v, got %v", exp, *act)
		}
	case float64:
		act, ok := actual.(*float64)
		if !ok || act == nil {
			return fmt.Errorf("expected number value, got %T", actual)
		}
		if math.Abs(*act-exp) > 0.0001 {
			return fmt.Errorf("value mismatch: expected %v, got %v", exp, *act)
		}
	case string:
		act, ok := actual.(*string)
		if !ok || act == nil {
			return fmt.Errorf("expected string value, got %T", actual)
		}
		if *act != exp {
			return fmt.Errorf("value mismatch: expected %v, got %v", exp, *act)
		}
	default:
		return fmt.Errorf("unsupported type: %T", expected)
	}
	return nil
}

// ValidateTelemetryData 验证遥测数据
func ValidateTelemetryData(expectedData map[string]interface{}, actualKey string, actualBoolV *bool, actualNumberV *float64, actualStringV *string) error {
	expected, ok := expectedData[actualKey]
	if !ok {
		return fmt.Errorf("unexpected key: %s", actualKey)
	}

	switch exp := expected.(type) {
	case bool:
		return ValidateTelemetryValue(exp, actualBoolV)
	case float64:
		return ValidateTelemetryValue(exp, actualNumberV)
	case string:
		return ValidateTelemetryValue(exp, actualStringV)
	default:
		// 尝试作为数字处理
		if num, ok := expected.(int); ok {
			return ValidateTelemetryValue(float64(num), actualNumberV)
		}
		return fmt.Errorf("unsupported expected type: %T", expected)
	}
}

// ValidateAttributeData 验证属性数据
func ValidateAttributeData(expectedData map[string]interface{}, actualKey string, actualBoolV *bool, actualNumberV *float64, actualStringV *string) error {
	return ValidateTelemetryData(expectedData, actualKey, actualBoolV, actualNumberV, actualStringV)
}

// ValidateEventData 验证事件数据
func ValidateEventData(expectedMethod string, expectedParams map[string]interface{}, actualData string) error {
	var actual map[string]interface{}
	if err := json.Unmarshal([]byte(actualData), &actual); err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}

	// 检查数据格式: 可能是完整格式 {"method": "xxx", "params": {...}}
	// 也可能只有 params {...}

	// 情况1: 完整格式,包含 method 和 params
	if method, ok := actual["method"].(string); ok {
		if method != expectedMethod {
			return fmt.Errorf("method mismatch: expected %s, got %s", expectedMethod, method)
		}

		actualParams, ok := actual["params"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("params not found in event data")
		}

		return validateParams(expectedParams, actualParams)
	}

	// 情况2: 只有 params,没有 method。当前用于兼容把 identify 放在外层表字段的落库格式。
	// 直接验证整个 actual 作为 params
	return validateParams(expectedParams, actual)
}

// validateParams 验证参数
func validateParams(expected, actual map[string]interface{}) error {
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok {
			return fmt.Errorf("param key %s not found", key)
		}

		// 对于数字类型,需要统一处理
		if expNum, ok := expectedValue.(float64); ok {
			if actNum, ok := actualValue.(float64); ok {
				if expNum != actNum {
					return fmt.Errorf("param %s mismatch: expected %v, got %v", key, expectedValue, actualValue)
				}
				continue
			}
		}

		// 其他类型直接比较
		if fmt.Sprintf("%v", expectedValue) != fmt.Sprintf("%v", actualValue) {
			return fmt.Errorf("param %s mismatch: expected %v, got %v", key, expectedValue, actualValue)
		}
	}

	return nil
}

// ValidateResponse 验证响应格式
func ValidateResponse(response map[string]interface{}) error {
	result, ok := response["result"]
	if !ok {
		return fmt.Errorf("result field not found")
	}

	resultNum, ok := result.(float64)
	if !ok {
		return fmt.Errorf("result field is not a number")
	}

	if resultNum != 0 {
		errcode, _ := response["errcode"].(string)
		message, _ := response["message"].(string)
		return fmt.Errorf("response indicates failure: result=%v, errcode=%s, message=%s",
			resultNum, errcode, message)
	}

	return nil
}

// ValidateTimestamp 验证时间戳
func ValidateTimestamp(expected time.Time, actual int64, toleranceSeconds int) error {
	actualTime := time.Unix(actual, 0)
	diff := math.Abs(float64(actualTime.Sub(expected).Seconds()))

	if diff > float64(toleranceSeconds) {
		return fmt.Errorf("timestamp out of tolerance: expected %v, got %v, diff %.0f seconds",
			expected, actualTime, diff)
	}

	return nil
}

// ValidateJSON 验证JSON字符串格式
func ValidateJSON(jsonStr string) error {
	var js interface{}
	if err := json.Unmarshal([]byte(jsonStr), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// ParseMessageFromTopic extracts the message_id from a concrete topic.
// When pattern contains '+', the last matching '+' segment is treated as the
// message_id slot. With an empty pattern, the last topic segment is returned.
func ParseMessageFromTopic(topic string, pattern string) (string, error) {
	topicParts := splitTopicSegments(topic)
	if len(topicParts) == 0 {
		return "", fmt.Errorf("topic is empty")
	}

	if strings.TrimSpace(pattern) == "" {
		return lastMessageIDSegment(topicParts)
	}

	patternParts := splitTopicSegments(pattern)
	if len(patternParts) == 0 {
		return "", fmt.Errorf("pattern is empty")
	}

	messageIndex := -1
	for i, patternPart := range patternParts {
		if patternPart == "#" {
			if i != len(patternParts)-1 {
				return "", fmt.Errorf("invalid topic pattern %q: # must be the final segment", pattern)
			}
			if len(topicParts) < i {
				return "", fmt.Errorf("topic %q does not match pattern %q", topic, pattern)
			}
			break
		}
		if i >= len(topicParts) {
			return "", fmt.Errorf("topic %q does not match pattern %q", topic, pattern)
		}
		if patternPart == "+" {
			messageIndex = i
			continue
		}
		if patternPart != topicParts[i] {
			return "", fmt.Errorf("topic %q does not match pattern %q", topic, pattern)
		}
	}

	if patternParts[len(patternParts)-1] != "#" && len(topicParts) != len(patternParts) {
		return "", fmt.Errorf("topic %q does not match pattern %q", topic, pattern)
	}
	if messageIndex >= 0 {
		return lastMessageIDSegment(topicParts[:messageIndex+1])
	}
	return lastMessageIDSegment(topicParts)
}

func splitTopicSegments(topic string) []string {
	topic = strings.Trim(topic, "/")
	if topic == "" {
		return nil
	}

	rawParts := strings.Split(topic, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func lastMessageIDSegment(parts []string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("message_id segment not found")
	}
	messageID := parts[len(parts)-1]
	if messageID == "" || messageID == "+" || messageID == "#" {
		return "", fmt.Errorf("message_id segment is not concrete")
	}
	return messageID, nil
}
