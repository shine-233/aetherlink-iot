// 文件用途：定义自动测试报文构建器的统一接口。
// 核心逻辑：抽象遥测、属性、事件和平台下行响应的 JSON payload 构建能力，供直连和网关实现复用。
// 关键注意事项：接口只约束输出字节和错误，不约束 payload schema；schema 责任落在具体 builder 与测试数据上。
// 重构建议：可引入契约 fixture 测试，确保各 builder 输出与设备接入规范保持一致。

/*
Purpose: 定义自动测试报文构建器的统一接口。
Core logic: 抽象遥测、属性、事件和平台下行响应的 JSON payload 构建能力，供直连和网关设备实现复用。
Important notes: 接口只约束输出字节和错误，不约束 payload schema；schema 责任落在具体 builder 与测试数据上。
Refactor suggestion: 可引入契约 fixture 测试，确保各 builder 输出与 docs 中的设备接入规范保持一致。
*/
package protocol

// MessageBuilder 消息构建器接口
type MessageBuilder interface {
	// BuildTelemetry 构建设备主动上报的遥测 payload。
	BuildTelemetry(data interface{}) ([]byte, error)

	// BuildAttribute 构建设备主动上报的属性 payload。
	BuildAttribute(data interface{}) ([]byte, error)

	// BuildEvent 构建设备主动上报的事件 payload；直连与网关可有不同 schema。
	BuildEvent(method string, params interface{}) ([]byte, error)

	// BuildResponse 构建平台下行后的设备响应 payload。
	BuildResponse(success bool, method string) ([]byte, error)
}
