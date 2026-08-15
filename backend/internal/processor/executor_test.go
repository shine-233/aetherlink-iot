// 文件用途：覆盖设备数据处理器 executor 行为的 Go 测试。
// 核心逻辑：验证 Lua 执行、模型校验、错误包装、缓存键和上下行处理契约，主要围绕 func TestLuaExecutorExecuteDecodeRunsBusinessScriptWithJsonModule、func TestLuaExecutorExecuteEncodeBuildsDeviceCommandPayload、func TestLuaExecutorSandboxBlocksDangerousRuntimeLibraries、func TestLuaExecutorReportsMissingEntryPointAsProcessorError 等声明展开。
// 关键注意事项：处理器测试需区分业务脚本错误、输入方向错误和沙箱限制，避免过宽断言。
// 重构建议：后续可补充更多真实脚本样例和故障注入用例，强化编解码契约。

package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestLuaExecutorExecuteDecodeRunsBusinessScriptWithJsonModule(t *testing.T) {
	executor := NewLuaExecutor()
	script := `
local json = require("json")

function encodeInp(msg, topic)
  local decoded = json.decode(msg)
  return json.encode({
    temperature = decoded.temp,
    alarm = decoded.temp > 80,
    source_topic = topic
  })
end
`

	got, err := executor.ExecuteDecode(context.Background(), script, []byte(`{"temp":91}`))
	if err != nil {
		t.Fatalf("ExecuteDecode returned error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("ExecuteDecode returned non-JSON result %q: %v", got, err)
	}
	if decoded["temperature"] != float64(91) {
		t.Fatalf("temperature = %#v, want 91", decoded["temperature"])
	}
	if decoded["alarm"] != true {
		t.Fatalf("alarm = %#v, want true for over-threshold telemetry", decoded["alarm"])
	}
	if decoded["source_topic"] != "" {
		t.Fatalf("source_topic = %#v, want empty fallback topic", decoded["source_topic"])
	}
}

func TestLuaExecutorExecuteEncodeBuildsDeviceCommandPayload(t *testing.T) {
	executor := NewLuaExecutor()
	script := `
local json = require("json")

function encodeInp(msg, topic)
  local decoded = json.decode(msg)
  return decoded.method .. ":" .. decoded.params.led .. ":" .. tostring(decoded.params.enabled)
end
`

	got, err := executor.ExecuteEncode(context.Background(), script, []byte(`{"method":"set_led","params":{"led":"green","enabled":true}}`))
	if err != nil {
		t.Fatalf("ExecuteEncode returned error: %v", err)
	}
	if got != "set_led:green:true" {
		t.Fatalf("encoded command payload = %q, want set_led:green:true", got)
	}
}

func TestLuaExecutorSandboxBlocksDangerousRuntimeLibraries(t *testing.T) {
	executor := NewLuaExecutor()
	script := `
function encodeInp(msg, topic)
  if os ~= nil or io ~= nil or dofile ~= nil or load ~= nil or loadfile ~= nil or loadstring ~= nil then
    return "unsafe"
  end
  return "safe"
end
`

	got, err := executor.ExecuteDecode(context.Background(), script, []byte(`{}`))
	if err != nil {
		t.Fatalf("ExecuteDecode returned error: %v", err)
	}
	if got != "safe" {
		t.Fatalf("sandbox visibility = %q, want safe", got)
	}
}

func TestLuaExecutorReportsMissingEntryPointAsProcessorError(t *testing.T) {
	executor := NewLuaExecutor()

	tests := []struct {
		name string
		run  func() (string, error)
	}{
		{
			name: "decode",
			run: func() (string, error) {
				return executor.ExecuteDecode(context.Background(), `local json = require("json")`, []byte(`{"temp":91}`))
			},
		},
		{
			name: "encode",
			run: func() (string, error) {
				return executor.ExecuteEncode(context.Background(), `local json = require("json")`, []byte(`{"method":"set_led"}`))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.run()
			if err == nil {
				t.Fatal("expected error for script without encodeInp")
			}

			var processorErr *ProcessorError
			if !errors.As(err, &processorErr) {
				t.Fatalf("error = %#v, want ProcessorError", err)
			}
			if processorErr.Code != ErrCodeScriptExecuteFailed {
				t.Fatalf("ProcessorError code = %q, want %q", processorErr.Code, ErrCodeScriptExecuteFailed)
			}
			if processorErr.Message != "script execution failed" {
				t.Fatalf("ProcessorError message = %q, want script execution failed", processorErr.Message)
			}

			var apiErr *lua.ApiError
			if !errors.As(err, &apiErr) {
				t.Fatalf("ProcessorError cause = %#v, want lua.ApiError", processorErr.Unwrap())
			}
			if apiErr.Type != lua.ApiErrorRun {
				t.Fatalf("lua ApiError type = %v, want %v", apiErr.Type, lua.ApiErrorRun)
			}
			if apiErr.Object.String() != "function 'encodeInp' not found in script" {
				t.Fatalf("lua ApiError object = %q, want encodeInp missing message", apiErr.Object.String())
			}
			if err.Error() != "script execution failed: function 'encodeInp' not found in script" {
				t.Fatalf("ProcessorError Error() = %q, want exact missing entrypoint message", err.Error())
			}
		})
	}
}
