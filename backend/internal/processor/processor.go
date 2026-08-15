// 文件用途：承载设备编解码脚本处理模块的 processor 逻辑。
// 核心逻辑：围绕脚本缓存、Lua 沙箱执行、输入输出模型和处理器接口实现上下行数据转换，主要围绕 type DataProcessor 等声明展开。
// 关键注意事项：脚本处理涉及超时、沙箱和错误码，修改需保持上下行方向及失败语义清晰。
// 重构建议：后续可进一步拆分执行器、缓存和领域模型，降低处理器聚合复杂度。

package processor

import "context"

// DataProcessor 数据处理器核心接口
type DataProcessor interface {
	// Decode 上行数据解码：设备协议数据 -> 标准化数据
	// 用于：telemetry、attribute、event
	Decode(ctx context.Context, input *DecodeInput) (*DecodeOutput, error)

	// Encode 下行数据编码：标准化数据 -> 设备协议数据
	// 用于：telemetry_control、attribute_set、command
	Encode(ctx context.Context, input *EncodeInput) (*EncodeOutput, error)
}
