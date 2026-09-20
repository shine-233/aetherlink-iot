// 文件用途：承载设备编解码脚本处理模块的 executor 逻辑。
// 核心逻辑：围绕脚本缓存、Lua 沙箱执行、输入输出模型和处理器接口实现上下行数据转换，主要围绕 type LuaExecutor、func NewLuaExecutor、func (e *LuaExecutor) ExecuteDecode、func (e *LuaExecutor) ExecuteEncode 等声明展开。
// 关键注意事项：脚本处理涉及超时、沙箱和错误码，修改需保持上下行方向及失败语义清晰。
// 重构建议：后续可进一步拆分执行器、缓存和领域模型，降低处理器聚合复杂度。

package processor

import (
	"context"
	"errors"

	"aetherlink-iot/backend/pkg/safelua"
)

// LuaExecutor Lua 脚本执行器
type LuaExecutor struct {
}

// NewLuaExecutor 创建 Lua 执行器
func NewLuaExecutor() *LuaExecutor {
	return &LuaExecutor{}
}

// ExecuteDecode 执行解码脚本（上行：设备原始数据 -> JSON）
// scriptContent: 脚本内容
// rawData: 原始字节数据
func (e *LuaExecutor) ExecuteDecode(ctx context.Context, scriptContent string, rawData []byte) (string, error) {
	return executeLuaScript(ctx, scriptContent, rawData)
}

// ExecuteEncode 执行编码脚本（下行：JSON -> 设备协议数据）
// scriptContent: 脚本内容
// jsonData: JSON 格式的标准化数据
func (e *LuaExecutor) ExecuteEncode(ctx context.Context, scriptContent string, jsonData []byte) (string, error) {
	return executeLuaScript(ctx, scriptContent, jsonData)
}

func executeLuaScript(ctx context.Context, scriptContent string, data []byte) (string, error) {
	result, err := safelua.Execute(ctx, scriptContent, data, "")
	if err == nil {
		return result, nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "", NewScriptTimeoutError()
	}
	return "", NewScriptExecuteError(err)
}
