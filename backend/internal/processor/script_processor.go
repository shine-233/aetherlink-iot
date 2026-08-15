// 文件用途：承载设备编解码脚本处理模块的 script processor 逻辑。
// 核心逻辑：围绕脚本缓存、Lua 沙箱执行、输入输出模型和处理器接口实现上下行数据转换，主要围绕 type ScriptProcessor、func NewScriptProcessor、func (p *ScriptProcessor) Decode、func (p *ScriptProcessor) Encode 等声明展开。
// 关键注意事项：脚本处理涉及超时、沙箱和错误码，修改需保持上下行方向及失败语义清晰。
// 重构建议：后续可进一步拆分执行器、缓存和领域模型，降低处理器聚合复杂度。

package processor

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
)

// ScriptProcessor 基于 Lua 脚本的数据处理器实现
type ScriptProcessor struct {
	cache    *ScriptCache // 脚本缓存管理器
	executor *LuaExecutor // Lua 执行引擎
}

// NewScriptProcessor 创建脚本处理器实例
func NewScriptProcessor() *ScriptProcessor {
	return &ScriptProcessor{
		cache:    NewScriptCache(),
		executor: NewLuaExecutor(),
	}
}

// Decode 上行数据解码：设备协议数据 -> 标准化数据
func (p *ScriptProcessor) Decode(ctx context.Context, input *DecodeInput) (*DecodeOutput, error) {
	startTime := time.Now()
	if err := validateDecodeInput(input); err != nil {
		return failedDecodeOutput(err), err
	}

	scriptType, err := resolveDecodeScriptType(input)
	if err != nil {
		return failedDecodeOutput(err), err
	}

	script, output, err := p.loadDecodeScript(ctx, input, scriptType)
	if output != nil || err != nil {
		return output, err
	}
	if err := ensureDecodeScriptEnabled(input, scriptType, script); err != nil {
		return failedDecodeOutput(err), err
	}

	resultStr, err := p.executeDecodeScript(ctx, input, scriptType, script, startTime)
	if err != nil {
		return failedDecodeOutput(err), err
	}
	data := normalizeDecodeScriptResult(input, scriptType, script, resultStr)
	logDecodeSuccess(input, scriptType, script, startTime)

	return &DecodeOutput{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
		Error:     nil,
	}, nil
}

func validateDecodeInput(input *DecodeInput) error {
	if err := input.Validate(); err != nil {
		logrus.WithFields(logrus.Fields{
			"module": "processor",
			"method": "Decode",
			"error":  err.Error(),
		}).Error("invalid input")
		return err
	}
	return nil
}

func failedDecodeOutput(err error) *DecodeOutput {
	return &DecodeOutput{
		Success:   false,
		Error:     err,
		Timestamp: time.Now().UnixMilli(),
	}
}

func resolveDecodeScriptType(input *DecodeInput) (string, error) {
	scriptType, ok := GetScriptType(input.Type)
	if ok {
		return scriptType, nil
	}
	err := NewInvalidInputError("unsupported data type: " + string(input.Type))
	logrus.WithFields(logrus.Fields{
		"module":    "processor",
		"method":    "Decode",
		"data_type": input.Type,
		"error":     err.Error(),
	}).Error("invalid data type")
	return "", err
}

func (p *ScriptProcessor) loadDecodeScript(ctx context.Context, input *DecodeInput, scriptType string) (*CachedScript, *DecodeOutput, error) {
	script, err := p.cache.GetScript(ctx, input.DeviceConfigID, scriptType)
	if err == nil {
		return script, nil, nil
	}
	if errors.Is(err, ErrScriptNotFound) {
		logrus.WithFields(logrus.Fields{
			"module":           "processor",
			"method":           "Decode",
			"device_config_id": input.DeviceConfigID,
			"data_type":        input.Type,
			"script_type":      scriptType,
		}).Debug("【脚本缓存】no script configured, using raw data")
		return nil, &DecodeOutput{
			Success:   true,
			Data:      input.RawData,
			Timestamp: time.Now().UnixMilli(),
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"module":           "processor",
		"method":           "Decode",
		"device_config_id": input.DeviceConfigID,
		"data_type":        input.Type,
		"script_type":      scriptType,
		"error":            err.Error(),
	}).Error("failed to get script")
	return nil, failedDecodeOutput(err), err
}

func ensureDecodeScriptEnabled(input *DecodeInput, scriptType string, script *CachedScript) error {
	if script.IsEnabled() {
		return nil
	}
	err := NewScriptDisabledError(input.DeviceConfigID, scriptType)
	logrus.WithFields(logrus.Fields{
		"module":           "processor",
		"method":           "Decode",
		"device_config_id": input.DeviceConfigID,
		"script_type":      scriptType,
		"script_id":        script.ID,
	}).Warn("script is disabled")
	return err
}

func (p *ScriptProcessor) executeDecodeScript(ctx context.Context, input *DecodeInput, scriptType string, script *CachedScript, startTime time.Time) (string, error) {
	resultStr, err := p.executor.ExecuteDecode(ctx, script.Content, input.RawData)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"module":           "processor",
			"method":           "Decode",
			"device_config_id": input.DeviceConfigID,
			"data_type":        input.Type,
			"script_type":      scriptType,
			"script_id":        script.ID,
			"duration_ms":      time.Since(startTime).Milliseconds(),
			"error":            err.Error(),
		}).Error("script execution failed")
		return "", err
	}
	return resultStr, nil
}

func normalizeDecodeScriptResult(input *DecodeInput, scriptType string, script *CachedScript, resultStr string) json.RawMessage {
	var testMap map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &testMap); err == nil {
		return json.RawMessage(resultStr)
	}

	logrus.WithFields(logrus.Fields{
		"module":           "processor",
		"method":           "Decode",
		"device_config_id": input.DeviceConfigID,
		"data_type":        input.Type,
		"script_type":      scriptType,
		"script_id":        script.ID,
		"raw_result":       resultStr,
	}).Warn("【脚本处理器】script returned non-JSON-object data, wrapping based on data type")

	wrappedData, _ := json.Marshal(wrapDecodeRawResult(input.Type, resultStr))
	return json.RawMessage(wrappedData)
}

func wrapDecodeRawResult(dataType DataType, resultStr string) map[string]interface{} {
	var rawValue interface{}
	if err := json.Unmarshal([]byte(resultStr), &rawValue); err != nil {
		rawValue = resultStr
	}
	if dataType == DataTypeEvent {
		return map[string]interface{}{
			"method": "_raw",
			"params": map[string]interface{}{
				"value": rawValue,
			},
		}
	}
	return map[string]interface{}{
		"_raw": rawValue,
	}
}

func logDecodeSuccess(input *DecodeInput, scriptType string, script *CachedScript, startTime time.Time) {
	logrus.WithFields(logrus.Fields{
		"module":           "processor",
		"method":           "Decode",
		"device_config_id": input.DeviceConfigID,
		"data_type":        input.Type,
		"script_type":      scriptType,
		"script_id":        script.ID,
		"duration_ms":      time.Since(startTime).Milliseconds(),
		"success":          true,
	}).Debug("【脚本处理器】decode completed")
}

// Encode 下行数据编码：标准化数据 -> 设备协议数据
func (p *ScriptProcessor) Encode(ctx context.Context, input *EncodeInput) (*EncodeOutput, error) {
	startTime := time.Now()

	// 1. 验证输入参数
	if err := input.Validate(); err != nil {
		logrus.WithFields(logrus.Fields{
			"module": "processor",
			"method": "Encode",
			"error":  err.Error(),
		}).Error("invalid input")
		return &EncodeOutput{
			Success: false,
			Error:   err,
		}, err
	}

	// 2. 获取 scriptType
	scriptType, ok := GetScriptType(input.Type)
	if !ok {
		err := NewInvalidInputError("unsupported data type: " + string(input.Type))
		logrus.WithFields(logrus.Fields{
			"module":    "processor",
			"method":    "Encode",
			"data_type": input.Type,
			"error":     err.Error(),
		}).Error("invalid data type")
		return &EncodeOutput{
			Success: false,
			Error:   err,
		}, err
	}

	// 3. 从缓存加载脚本
	script, err := p.cache.GetScript(ctx, input.DeviceConfigID, scriptType)
	if err != nil {
		// 如果脚本不存在,返回成功并使用原始数据(脚本是可选的)
		if errors.Is(err, ErrScriptNotFound) {
			logrus.WithFields(logrus.Fields{
				"module":           "processor",
				"method":           "Encode",
				"device_config_id": input.DeviceConfigID,
				"data_type":        input.Type,
				"script_type":      scriptType,
			}).Debug("no script configured, using raw data")

			// Encode时如果没有脚本,直接使用输入数据
			return &EncodeOutput{
				Success:     true,
				EncodedData: input.Data,
			}, nil
		}

		// 其他错误(缓存/数据库错误等)则返回失败
		logrus.WithFields(logrus.Fields{
			"module":           "processor",
			"method":           "Encode",
			"device_config_id": input.DeviceConfigID,
			"data_type":        input.Type,
			"script_type":      scriptType,
			"error":            err.Error(),
		}).Error("failed to get script")
		return &EncodeOutput{
			Success: false,
			Error:   err,
		}, err
	}

	// 4. 检查脚本是否启用
	if !script.IsEnabled() {
		err := NewScriptDisabledError(input.DeviceConfigID, scriptType)
		logrus.WithFields(logrus.Fields{
			"module":           "processor",
			"method":           "Encode",
			"device_config_id": input.DeviceConfigID,
			"script_type":      scriptType,
			"script_id":        script.ID,
		}).Warn("script is disabled")
		return &EncodeOutput{
			Success: false,
			Error:   err,
		}, err
	}

	// 5. 执行脚本编码
	resultStr, err := p.executor.ExecuteEncode(ctx, script.Content, input.Data)
	if err != nil {
		duration := time.Since(startTime)
		logrus.WithFields(logrus.Fields{
			"module":           "processor",
			"method":           "Encode",
			"device_config_id": input.DeviceConfigID,
			"data_type":        input.Type,
			"script_type":      scriptType,
			"script_id":        script.ID,
			"duration_ms":      duration.Milliseconds(),
			"error":            err.Error(),
		}).Error("script execution failed")
		return &EncodeOutput{
			Success: false,
			Error:   err,
		}, err
	}

	// 6. 将结果转换为字节数组
	encodedData := []byte(resultStr)

	// 7. 记录成功日志
	duration := time.Since(startTime)
	logrus.WithFields(logrus.Fields{
		"module":           "processor",
		"method":           "Encode",
		"device_config_id": input.DeviceConfigID,
		"data_type":        input.Type,
		"script_type":      scriptType,
		"script_id":        script.ID,
		"duration_ms":      duration.Milliseconds(),
		"success":          true,
	}).Info("encode completed")

	return &EncodeOutput{
		Success:     true,
		EncodedData: encodedData,
		Error:       nil,
	}, nil
}

// InvalidateScriptCache 使指定脚本缓存失效（供外部调用，脚本更新时使用）
func (p *ScriptProcessor) InvalidateScriptCache(ctx context.Context, deviceConfigID, scriptType string) error {
	// 清除 Redis 缓存
	return p.cache.InvalidateCache(ctx, deviceConfigID, scriptType)
}

// PreloadScripts 预加载脚本（可选，用于启动时预热缓存）
func (p *ScriptProcessor) PreloadScripts(ctx context.Context, deviceConfigID string) error {
	return p.cache.PreloadScripts(ctx, deviceConfigID)
}
