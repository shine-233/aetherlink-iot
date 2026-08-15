// 文件用途：覆盖设备数据处理器 models errors 行为的 Go 测试。
// 核心逻辑：验证 Lua 执行、模型校验、错误包装、缓存键和上下行处理契约，主要围绕 func assertProcessorErrorExact、func TestDecodeInputValidateAcceptsOnlyUplinkPayloads、func TestEncodeInputValidateAcceptsOnlyDownlinkPayloads、func TestScriptTypeMappingCacheKeyAndEnabledFlag 等声明展开。
// 关键注意事项：处理器测试需区分业务脚本错误、输入方向错误和沙箱限制，避免过宽断言。
// 重构建议：后续可补充更多真实脚本样例和故障注入用例，强化编解码契约。

package processor

import (
	"encoding/json"
	"errors"
	"testing"
)

func assertProcessorErrorExact(t *testing.T, err error, code string, message string, cause error) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected ProcessorError %s/%q, got nil", code, message)
	}
	var processorErr *ProcessorError
	if !errors.As(err, &processorErr) {
		t.Fatalf("error = %#v, want ProcessorError", err)
	}
	if processorErr.Code != code {
		t.Fatalf("ProcessorError code = %q, want %q", processorErr.Code, code)
	}
	if processorErr.Message != message {
		t.Fatalf("ProcessorError message = %q, want %q", processorErr.Message, message)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("ProcessorError cause = %v, want errors.Is cause %v", processorErr.Unwrap(), cause)
	}
	wantError := message
	if cause != nil {
		wantError += ": " + cause.Error()
	}
	if err.Error() != wantError {
		t.Fatalf("ProcessorError Error() = %q, want %q", err.Error(), wantError)
	}
}

func TestDecodeInputValidateAcceptsOnlyUplinkPayloads(t *testing.T) {
	validTypes := []DataType{DataTypeTelemetry, DataTypeAttribute, DataTypeEvent}
	for _, dataType := range validTypes {
		t.Run(string(dataType), func(t *testing.T) {
			input := &DecodeInput{
				DeviceConfigID: "config-1",
				Type:           dataType,
				RawData:        []byte(`{"value":1}`),
			}
			if err := input.Validate(); err != nil {
				t.Fatalf("DecodeInput.Validate returned error: %v", err)
			}
		})
	}

	tests := []struct {
		name  string
		input DecodeInput
		want  string
	}{
		{name: "missing config id", input: DecodeInput{Type: DataTypeTelemetry, RawData: []byte(`{}`)}, want: "device_config_id is required"},
		{name: "missing type", input: DecodeInput{DeviceConfigID: "config-1", RawData: []byte(`{}`)}, want: "type is required"},
		{name: "missing raw data", input: DecodeInput{DeviceConfigID: "config-1", Type: DataTypeTelemetry}, want: "raw_data is required"},
		{name: "downlink type rejected", input: DecodeInput{DeviceConfigID: "config-1", Type: DataTypeCommand, RawData: []byte(`{}`)}, want: "invalid type for decode, expected: telemetry/attribute/event"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			assertProcessorErrorExact(t, err, ErrCodeInvalidInput, tt.want, ErrInvalidInput)
		})
	}
}

func TestEncodeInputValidateAcceptsOnlyDownlinkPayloads(t *testing.T) {
	validTypes := []DataType{DataTypeTelemetryControl, DataTypeAttributeSet, DataTypeCommand}
	for _, dataType := range validTypes {
		t.Run(string(dataType), func(t *testing.T) {
			input := &EncodeInput{
				DeviceConfigID: "config-1",
				Type:           dataType,
				Data:           json.RawMessage(`{"value":1}`),
			}
			if err := input.Validate(); err != nil {
				t.Fatalf("EncodeInput.Validate returned error: %v", err)
			}
		})
	}

	tests := []struct {
		name  string
		input EncodeInput
		want  string
	}{
		{name: "missing config id", input: EncodeInput{Type: DataTypeCommand, Data: json.RawMessage(`{}`)}, want: "device_config_id is required"},
		{name: "missing type", input: EncodeInput{DeviceConfigID: "config-1", Data: json.RawMessage(`{}`)}, want: "type is required"},
		{name: "missing data", input: EncodeInput{DeviceConfigID: "config-1", Type: DataTypeCommand}, want: "data is required"},
		{name: "uplink type rejected", input: EncodeInput{DeviceConfigID: "config-1", Type: DataTypeTelemetry, Data: json.RawMessage(`{}`)}, want: "invalid type for encode, expected: telemetry_control/attribute_set/command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			assertProcessorErrorExact(t, err, ErrCodeInvalidInput, tt.want, ErrInvalidInput)
		})
	}
}

func TestScriptTypeMappingCacheKeyAndEnabledFlag(t *testing.T) {
	tests := []struct {
		dataType DataType
		want     string
	}{
		{DataTypeTelemetry, ScriptTypeTelemetryUplink},
		{DataTypeAttribute, ScriptTypeAttributeUplink},
		{DataTypeEvent, ScriptTypeEvent},
		{DataTypeTelemetryControl, ScriptTypeTelemetryDownlink},
		{DataTypeAttributeSet, ScriptTypeAttributeDownlink},
		{DataTypeCommand, ScriptTypeCommand},
	}

	for _, tt := range tests {
		got, ok := GetScriptType(tt.dataType)
		if !ok || got != tt.want {
			t.Fatalf("GetScriptType(%q) = %q,%v want %q,true", tt.dataType, got, ok, tt.want)
		}
	}

	if got, ok := GetScriptType(DataType("unknown")); ok || got != "" {
		t.Fatalf("GetScriptType unknown = %q,%v want empty,false", got, ok)
	}
	if got := GetCacheKey("config-1", ScriptTypeCommand); got != "config-1_E_script" {
		t.Fatalf("GetCacheKey = %q, want config-1_E_script", got)
	}
	if !(&CachedScript{EnableFlag: EnableFlagEnabled}).IsEnabled() {
		t.Fatal("CachedScript with enabled flag should be enabled")
	}
	if (&CachedScript{EnableFlag: "N"}).IsEnabled() {
		t.Fatal("CachedScript with non-enabled flag should be disabled")
	}
}

func TestProcessorErrorConstructorsPreserveCodesMessagesAndCauses(t *testing.T) {
	root := errors.New("root cause")
	tests := []struct {
		name    string
		err     *ProcessorError
		code    string
		message string
		cause   error
	}{
		{name: "not found", err: NewScriptNotFoundError("config-1", ScriptTypeCommand), code: ErrCodeScriptNotFound, message: "script not found for device_config_id: config-1, script_type: E", cause: ErrScriptNotFound},
		{name: "disabled", err: NewScriptDisabledError("config-1", ScriptTypeCommand), code: ErrCodeScriptDisabled, message: "script is disabled for device_config_id: config-1, script_type: E", cause: ErrScriptDisabled},
		{name: "execute", err: NewScriptExecuteError(root), code: ErrCodeScriptExecuteFailed, message: "script execution failed", cause: root},
		{name: "timeout", err: NewScriptTimeoutError(), code: ErrCodeScriptTimeout, message: "script execution timeout", cause: ErrScriptTimeout},
		{name: "invalid input", err: NewInvalidInputError("bad payload"), code: ErrCodeInvalidInput, message: "bad payload", cause: ErrInvalidInput},
		{name: "cache", err: NewCacheError(root), code: ErrCodeCacheError, message: "cache operation failed", cause: root},
		{name: "database", err: NewDatabaseError(root), code: ErrCodeDatabaseError, message: "database query failed", cause: root},
		{name: "generic", err: NewProcessorError("CUSTOM", "custom message", root), code: "CUSTOM", message: "custom message", cause: root},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProcessorErrorExact(t, tt.err, tt.code, tt.message, tt.cause)
		})
	}
}
