// 文件用途：覆盖 serialize struct 工具函数的 Go 测试。
// 核心逻辑：通过表驱动或边界用例验证通用工具的输入校验、格式转换和错误返回，主要围绕 type serializeSource、type serializeTarget、type structMapSample、func TestSerializeDataCopiesJSONAssignableBusinessPayload 等声明展开。
// 关键注意事项：工具包被多处业务代码复用，测试断言需保持跨调用方的兼容契约。
// 重构建议：后续可按工具类别拆分公共夹具，并补充失败路径和异常输入覆盖。

package utils

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type serializeSource struct {
	Name   string            `json:"name"`
	Count  int               `json:"count"`
	Labels map[string]string `json:"labels"`
}

type serializeTarget struct {
	Name   string            `json:"name"`
	Count  int               `json:"count"`
	Labels map[string]string `json:"labels"`
}

type structMapSample struct {
	ID      string
	Enabled bool
	Limit   int
}

func TestSerializeDataCopiesJSONAssignableBusinessPayload(t *testing.T) {
	source := serializeSource{
		Name:   "rdi-device-operations",
		Count:  3,
		Labels: map[string]string{"device": "pump-1"},
	}
	target := serializeTarget{}

	got, err := SerializeData(source, target)
	if err != nil {
		t.Fatalf("SerializeData returned error: %v", err)
	}

	converted, ok := got.(serializeTarget)
	if !ok {
		t.Fatalf("SerializeData returned %T, want serializeTarget", got)
	}
	if converted.Name != source.Name || converted.Count != source.Count {
		t.Fatalf("SerializeData converted scalar fields incorrectly: %+v", converted)
	}
	if !reflect.DeepEqual(converted.Labels, source.Labels) {
		t.Fatalf("SerializeData labels = %#v, want %#v", converted.Labels, source.Labels)
	}
}

func TestSerializeDataReturnsMarshalAndUnmarshalErrors(t *testing.T) {
	t.Run("marshal error", func(t *testing.T) {
		_, err := SerializeData(map[string]any{"bad": func() {}}, serializeTarget{})
		if err == nil {
			t.Fatal("SerializeData expected marshal error for function value")
		}
	})

	t.Run("unmarshal error", func(t *testing.T) {
		_, err := SerializeData(map[string]any{"count": "not-a-number"}, serializeTarget{})
		if err == nil {
			t.Fatal("SerializeData expected unmarshal error for mismatched target field")
		}
	})
}

func TestStructToMapRequiresNonNilPointerAndPreservesExportedFields(t *testing.T) {
	input := &structMapSample{
		ID:      "device-1",
		Enabled: true,
		Limit:   10,
	}

	got, err := StructToMap(input)
	if err != nil {
		t.Fatalf("StructToMap returned error: %v", err)
	}

	want := map[string]interface{}{
		"ID":      "device-1",
		"Enabled": true,
		"Limit":   10,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StructToMap = %#v, want %#v", got, want)
	}
}

func TestStructToMapRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{name: "non pointer", input: structMapSample{}},
		{name: "nil pointer", input: (*structMapSample)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := StructToMap(tt.input)
			if err == nil {
				t.Fatal("StructToMap expected error")
			}
			if !strings.Contains(err.Error(), "non-nil pointer") {
				t.Fatalf("StructToMap error = %v, want non-nil pointer error", err)
			}
		})
	}
}

func TestStructToMapRejectsNilInterfaceWithoutPanic(t *testing.T) {
	_, err := StructToMap(nil)
	if err == nil {
		t.Fatal("StructToMap(nil) expected an error")
	}
	if !strings.Contains(err.Error(), "non-nil pointer") {
		t.Fatalf("StructToMap(nil) error = %v, want non-nil pointer error", err)
	}
}

func TestSerializeDataDoesNotHideTypedJSONErrors(t *testing.T) {
	var unsupported = errors.New("json: unsupported type: func")
	_, err := SerializeData(map[string]any{"bad": func() {}}, serializeTarget{})
	if err == nil {
		t.Fatal("SerializeData expected an error")
	}
	if !strings.Contains(err.Error(), unsupported.Error()[:22]) {
		t.Fatalf("SerializeData error = %v, want unsupported type context", err)
	}
}
