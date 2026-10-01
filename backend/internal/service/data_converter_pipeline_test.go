// 文件用途：ExecuteUplinkDataConverter 单测（TB-45 上行管线桥接面）——三模式分派与失败语义。
// 核心逻辑：JSON_PATH/HEX_BINARY 正常映射、无输出/坏模式/nil 转换器按 error 返回，
//
//	attributes 与 telemetry 合并以 telemetry 优先。
//
// 关键注意事项：脚本模式依赖 ScriptDeal 运行时，此处不覆盖（data_converter_test.go 已有）；
// 失败返回 error 由 collector.IntegrationResolver 统一丢弃计数。
// 重构建议：Downlink 执行入口落地时对偶补齐其失败语义用例。
package service

import (
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
)

func TestExecuteUplinkDataConverterJsonPath(t *testing.T) {
	conv := &model.DataConverter{
		ID:            "c1",
		ConverterMode: "JSON_PATH",
		Configuration: `{"telemetry":{"temperature":"temp"}}`,
	}
	out, err := ExecuteUplinkDataConverter(conv, map[string]interface{}{"temp": 26.5}, map[string]string{"device_id": "d1"})
	if err != nil {
		t.Fatalf("JSON_PATH 转换不应报错: %v", err)
	}
	if out["temperature"] != 26.5 {
		t.Fatalf("映射结果不符: %v", out)
	}
}

func TestExecuteUplinkDataConverterHexBinary(t *testing.T) {
	conv := &model.DataConverter{
		ID:            "c2",
		ConverterMode: "HEX_BINARY",
		Configuration: `[{"key":"v","offset":0,"length":2,"type":"uint16"}]`,
	}
	// 0x01A2 = 418
	out, err := ExecuteUplinkDataConverter(conv, map[string]interface{}{"raw": "01A2"}, nil)
	if err != nil {
		t.Fatalf("HEX_BINARY 转换不应报错: %v", err)
	}
	if v, ok := out["v"].(uint64); !ok || v != 418 {
		t.Fatalf("解码结果不符: %v(%T)", out["v"], out["v"])
	}
}

func TestExecuteUplinkDataConverterMergesAttributesTelemetryWins(t *testing.T) {
	conv := &model.DataConverter{
		ID:            "c3",
		ConverterMode: "JSON_PATH",
		Configuration: `{"telemetry":{"k":"a"},"attributes":{"k":"b","attr":"c"}}`,
	}
	out, err := ExecuteUplinkDataConverter(conv, map[string]interface{}{"a": 1.0, "b": 2.0, "c": 3.0}, nil)
	if err != nil {
		t.Fatalf("转换不应报错: %v", err)
	}
	if out["k"] != 1.0 {
		t.Fatalf("键冲突应以 telemetry 为准: %v", out)
	}
	if out["attr"] != 3.0 {
		t.Fatalf("attributes 应并入输出: %v", out)
	}
}

func TestExecuteUplinkDataConverterFailurePaths(t *testing.T) {
	cases := []struct {
		name    string
		conv    *model.DataConverter
		values  map[string]interface{}
		wantErr string
	}{
		{"nil 转换器", nil, map[string]interface{}{"k": 1.0}, "converter is nil"},
		{"坏模式", &model.DataConverter{ID: "c", ConverterMode: "BOGUS"}, map[string]interface{}{"k": 1.0}, "unsupported converter mode"},
		{"无输出", &model.DataConverter{ID: "c", ConverterMode: "JSON_PATH", Configuration: `{"telemetry":{"x":"missing"}}`}, map[string]interface{}{"k": 1.0}, "no telemetry output"},
		{"HEX 多歧义字符串", &model.DataConverter{ID: "c", ConverterMode: "HEX_BINARY", Configuration: `[]`}, map[string]interface{}{"a": "01", "b": "02"}, "exactly one raw string telemetry value"},
		{"HEX 无字符串", &model.DataConverter{ID: "c", ConverterMode: "HEX_BINARY", Configuration: `[]`}, map[string]interface{}{"a": 1.0}, "exactly one raw string telemetry value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExecuteUplinkDataConverter(tc.conv, tc.values, nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望错误含 %q，实际 %v", tc.wantErr, err)
			}
		})
	}
}

func TestExecuteUplinkDataConverterHexPayloadKeyPreference(t *testing.T) {
	// HEX_BINARY 优先取 raw/payload/data 键，即使键值面还有别的字符串。
	conv := &model.DataConverter{
		ID:            "c4",
		ConverterMode: "HEX_BINARY",
		Configuration: `[{"key":"v","offset":0,"length":1,"type":"uint8"}]`,
	}
	out, err := ExecuteUplinkDataConverter(conv, map[string]interface{}{"raw": "7B", "note": "01"}, nil)
	if err != nil {
		t.Fatalf("raw 键优先不应报错: %v", err)
	}
	if v, ok := out["v"].(uint64); !ok || v != 0x7B {
		t.Fatalf("应解码 raw 键的 0x7B: %v(%T)", out["v"], out["v"])
	}
}
