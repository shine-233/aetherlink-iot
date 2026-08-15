// 文件用途：构建直连设备使用的 MQTT JSON 报文。
// 核心逻辑：将遥测、属性、事件和响应数据序列化为直连设备协议期望的 JSON 结构。
// 关键注意事项：响应字段变化会影响平台下行闭环验证，需保持 result、message、ts 和 method 约定稳定。
// 重构建议：可用固定 golden JSON fixture 覆盖成功、失败和 method 为空的响应分支。

package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// DirectMessageBuilder 直连设备消息构建器
type DirectMessageBuilder struct{}

// NewDirectMessageBuilder 创建直连设备消息构建器
func NewDirectMessageBuilder() *DirectMessageBuilder {
	return &DirectMessageBuilder{}
}

// BuildTelemetry 构建遥测数据报文(扁平JSON格式)
func (b *DirectMessageBuilder) BuildTelemetry(data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal telemetry data: %w", err)
	}
	return payload, nil
}

// BuildAttribute 构建属性数据报文(扁平JSON格式)
func (b *DirectMessageBuilder) BuildAttribute(data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attribute data: %w", err)
	}
	return payload, nil
}

// BuildEvent 构建事件数据报文
func (b *DirectMessageBuilder) BuildEvent(method string, params interface{}) ([]byte, error) {
	eventData := map[string]interface{}{
		"method": method,
		"params": params,
	}

	payload, err := json.Marshal(eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}
	return payload, nil
}

// BuildResponse 构建响应报文
func (b *DirectMessageBuilder) BuildResponse(success bool, method string) ([]byte, error) {
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

// WrapResponseEnvelope adds the device identity required by the backend MQTT
// adapter around a direct-device response.  The Values field is []byte on
// purpose: encoding/json encodes it as base64, which is the wire representation
// consumed by mqttadapter.publicPayload before ResponseUplink parses the inner
// result/message/method object.
func WrapResponseEnvelope(deviceID string, values []byte) ([]byte, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device id is required for response envelope")
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("response values are required for response envelope")
	}
	return json.Marshal(struct {
		DeviceID string `json:"device_id"`
		Values   []byte `json:"values"`
	}{
		DeviceID: deviceID,
		Values:   values,
	})
}
