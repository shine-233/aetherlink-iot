// 文件用途：构建网关设备及多层拓扑使用的 MQTT JSON 报文。
// 核心逻辑：透传或组装 gateway_data、sub_device_data、sub_gateway_data 嵌套结构，并复用响应报文格式。
// 关键注意事项：网关事件当前直接序列化完整嵌套结构，调用方必须传入平台可识别的完整 payload。
// 重构建议：可为嵌套结构定义显式类型和 schema 校验，减少 map[string]interface{} 带来的运行期错误。

package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// GatewayMessageBuilder 网关设备消息构建器
type GatewayMessageBuilder struct {
	topology interface{} // 拓扑结构（用于构建嵌套数据）
}

// NewGatewayMessageBuilder 创建网关设备消息构建器
func NewGatewayMessageBuilder(topology interface{}) *GatewayMessageBuilder {
	return &GatewayMessageBuilder{
		topology: topology,
	}
}

// BuildTelemetry 构建遥测数据报文(嵌套JSON格式)
// data 参数应该是一个 map，包含 gateway_data, sub_device_data, sub_gateway_data
func (b *GatewayMessageBuilder) BuildTelemetry(data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gateway telemetry data: %w", err)
	}
	return payload, nil
}

// BuildAttribute 构建属性数据报文(嵌套JSON格式)
func (b *GatewayMessageBuilder) BuildAttribute(data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gateway attribute data: %w", err)
	}
	return payload, nil
}

// BuildEvent 构建事件数据报文(嵌套JSON格式)
// 网关事件当前直接透传完整嵌套结构，不再额外包一层 method/params。
func (b *GatewayMessageBuilder) BuildEvent(method string, params interface{}) ([]byte, error) {
	// params 已经被调用方整理成平台可识别的网关事件结构。
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gateway event data: %w", err)
	}
	return payload, nil
}

// BuildResponse 构建响应报文(扁平格式)
func (b *GatewayMessageBuilder) BuildResponse(success bool, method string) ([]byte, error) {
	response := map[string]interface{}{
		"result":  0,
		"message": "success",
		"ts":      time.Now().Unix(),
	}

	if !success {
		response["result"] = 1
		response["message"] = "failed"
	}

	if method != "" {
		response["method"] = method
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return payload, nil
}

// BuildNestedTelemetry 构建嵌套的遥测数据（辅助方法）
func BuildNestedTelemetry(gatewayData, subDeviceData, subGatewayData map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 仅写入非 nil 片段，避免为不存在的层级制造空壳字段。
	if gatewayData != nil {
		result["gateway_data"] = gatewayData
	}

	if subDeviceData != nil {
		result["sub_device_data"] = subDeviceData
	}

	if subGatewayData != nil {
		result["sub_gateway_data"] = subGatewayData
	}

	return result
}

// BuildNestedAttributes 构建嵌套的属性数据（辅助方法）
func BuildNestedAttributes(gatewayData, subDeviceData, subGatewayData map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	if gatewayData != nil {
		result["gateway_data"] = gatewayData
	}

	if subDeviceData != nil {
		result["sub_device_data"] = subDeviceData
	}

	if subGatewayData != nil {
		result["sub_gateway_data"] = subGatewayData
	}

	return result
}

// BuildNestedEvents 构建嵌套的事件数据（辅助方法）
func BuildNestedEvents(gatewayEvent, subDeviceEvents, subGatewayEvents map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	if gatewayEvent != nil {
		result["gateway_data"] = gatewayEvent
	}

	if subDeviceEvents != nil {
		result["sub_device_data"] = subDeviceEvents
	}

	if subGatewayEvents != nil {
		result["sub_gateway_data"] = subGatewayEvents
	}

	return result
}
