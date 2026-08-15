// 文件用途：覆盖设备数据处理器 processor 行为的 Go 测试。
// 核心逻辑：验证 Lua 执行、模型校验、错误包装、缓存键和上下行处理契约，主要围绕 type fakeBusinessProcessor、func (p fakeBusinessProcessor) Decode、func (p fakeBusinessProcessor) Encode、func TestDataProcessorContractCoversUplinkDecodeAndDownlinkEncode 等声明展开。
// 关键注意事项：处理器测试需区分业务脚本错误、输入方向错误和沙箱限制，避免过宽断言。
// 重构建议：后续可补充更多真实脚本样例和故障注入用例，强化编解码契约。

package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeBusinessProcessor struct {
	decodeErr error
	encodeErr error
}

func (p fakeBusinessProcessor) Decode(ctx context.Context, input *DecodeInput) (*DecodeOutput, error) {
	if err := input.Validate(); err != nil {
		return &DecodeOutput{Success: false, Error: err}, err
	}
	if p.decodeErr != nil {
		return &DecodeOutput{Success: false, Error: p.decodeErr}, p.decodeErr
	}
	return &DecodeOutput{
		Success: true,
		Data:    json.RawMessage(`{"temperature":32,"alarm":false}`),
	}, nil
}

func (p fakeBusinessProcessor) Encode(ctx context.Context, input *EncodeInput) (*EncodeOutput, error) {
	if err := input.Validate(); err != nil {
		return &EncodeOutput{Success: false, Error: err}, err
	}
	if p.encodeErr != nil {
		return &EncodeOutput{Success: false, Error: p.encodeErr}, p.encodeErr
	}
	return &EncodeOutput{
		Success:     true,
		EncodedData: []byte(`set_led:green:true`),
	}, nil
}

func TestDataProcessorContractCoversUplinkDecodeAndDownlinkEncode(t *testing.T) {
	var processor DataProcessor = fakeBusinessProcessor{}

	decoded, err := processor.Decode(context.Background(), &DecodeInput{
		DeviceConfigID: "config-1",
		Type:           DataTypeTelemetry,
		RawData:        []byte(`{"temp":32}`),
	})
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if !decoded.Success {
		t.Fatal("Decode Success = false, want true")
	}
	var telemetry map[string]interface{}
	if err := json.Unmarshal(decoded.Data, &telemetry); err != nil {
		t.Fatalf("Decode Data is not JSON telemetry: %v", err)
	}
	if telemetry["temperature"] != float64(32) || telemetry["alarm"] != false {
		t.Fatalf("Decode telemetry = %#v, want temperature 32 and alarm false", telemetry)
	}

	encoded, err := processor.Encode(context.Background(), &EncodeInput{
		DeviceConfigID: "config-1",
		Type:           DataTypeCommand,
		Data:           json.RawMessage(`{"method":"set_led","params":{"led":"green","enabled":true}}`),
	})
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	if !encoded.Success {
		t.Fatal("Encode Success = false, want true")
	}
	if string(encoded.EncodedData) != "set_led:green:true" {
		t.Fatalf("Encode payload = %q, want set_led:green:true", encoded.EncodedData)
	}
}

func TestDataProcessorContractRejectsInvalidDirectionAndPropagatesBusinessErrors(t *testing.T) {
	var processor DataProcessor = fakeBusinessProcessor{}

	decoded, err := processor.Decode(context.Background(), &DecodeInput{
		DeviceConfigID: "config-1",
		Type:           DataTypeCommand,
		RawData:        []byte(`{"method":"set_led"}`),
	})
	if err == nil {
		t.Fatal("Decode expected error for downlink command type")
	}
	if decoded == nil || decoded.Success || decoded.Error == nil {
		t.Fatalf("Decode output = %#v, want failed output with error", decoded)
	}
	var processorErr *ProcessorError
	if !errors.As(err, &processorErr) || processorErr.Code != ErrCodeInvalidInput {
		t.Fatalf("Decode error = %#v, want INVALID_INPUT ProcessorError", err)
	}

	businessErr := NewProcessorError("PIPELINE_DISPATCH_FAILED", "processor dispatch failed", errors.New("route missing"))
	processor = fakeBusinessProcessor{encodeErr: businessErr}
	encoded, err := processor.Encode(context.Background(), &EncodeInput{
		DeviceConfigID: "config-1",
		Type:           DataTypeCommand,
		Data:           json.RawMessage(`{"method":"set_led"}`),
	})
	if !errors.Is(err, businessErr) {
		t.Fatalf("Encode error = %v, want business error propagation", err)
	}
	if encoded == nil || encoded.Success || encoded.Error == nil {
		t.Fatalf("Encode output = %#v, want failed output with propagated error", encoded)
	}
}
