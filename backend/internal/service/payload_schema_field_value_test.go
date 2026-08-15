// 文件用途：对纯函数 validatePayloadFieldValue 做逐分支表驱动覆盖。
// 关键注意事项：这是可离线验证的纯校验引擎,不落库、不连 broker;
//
//	broker 侧对上行 payload 的真实拦截仍需运行时(broker+PG+设备)验证。
package service

import (
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
)

func TestValidatePayloadFieldValue_AllBranches(t *testing.T) {
	cases := []struct {
		name      string
		field     model.PayloadSchemaField
		value     any
		wantEmpty bool   // true = 校验通过(空串)
		wantSub   string // 非通过时,错误消息应包含的子串
	}{
		// number
		{
			name:      "number ok within range",
			field:     model.PayloadSchemaField{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber, Min: floatPtr(-40), Max: floatPtr(125)},
			value:     42.0,
			wantEmpty: true,
		},
		{
			name:    "number wrong type",
			field:   model.PayloadSchemaField{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber},
			value:   "not-a-number",
			wantSub: "must be a number",
		},
		{
			name:    "number below min",
			field:   model.PayloadSchemaField{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber, Min: floatPtr(0)},
			value:   -1.0,
			wantSub: "below min",
		},
		{
			name:    "number above max",
			field:   model.PayloadSchemaField{Name: "temp", Type: model.PayloadSchemaFieldTypeNumber, Max: floatPtr(100)},
			value:   101.0,
			wantSub: "above max",
		},
		// string
		{
			name:      "string ok no constraints",
			field:     model.PayloadSchemaField{Name: "mode", Type: model.PayloadSchemaFieldTypeString},
			value:     "auto",
			wantEmpty: true,
		},
		{
			name:    "string wrong type",
			field:   model.PayloadSchemaField{Name: "mode", Type: model.PayloadSchemaFieldTypeString},
			value:   123.0,
			wantSub: "must be a string",
		},
		{
			name:    "string not in enum",
			field:   model.PayloadSchemaField{Name: "mode", Type: model.PayloadSchemaFieldTypeString, Enum: []string{"auto", "manual"}},
			value:   "turbo",
			wantSub: "not in the allowed enum",
		},
		{
			name:      "string in enum",
			field:     model.PayloadSchemaField{Name: "mode", Type: model.PayloadSchemaFieldTypeString, Enum: []string{"auto", "manual"}},
			value:     "manual",
			wantEmpty: true,
		},
		{
			name:    "string invalid pattern compile error",
			field:   model.PayloadSchemaField{Name: "code", Type: model.PayloadSchemaFieldTypeString, Pattern: strPtr("[unclosed")},
			value:   "anything",
			wantSub: "invalid pattern",
		},
		{
			name:    "string pattern mismatch",
			field:   model.PayloadSchemaField{Name: "code", Type: model.PayloadSchemaFieldTypeString, Pattern: strPtr("^[0-9]+$")},
			value:   "abc",
			wantSub: "does not match pattern",
		},
		{
			name:      "string pattern match",
			field:     model.PayloadSchemaField{Name: "code", Type: model.PayloadSchemaFieldTypeString, Pattern: strPtr("^[0-9]+$")},
			value:     "12345",
			wantEmpty: true,
		},
		// boolean
		{
			name:      "boolean ok",
			field:     model.PayloadSchemaField{Name: "on", Type: model.PayloadSchemaFieldTypeBoolean},
			value:     true,
			wantEmpty: true,
		},
		{
			name:    "boolean wrong type",
			field:   model.PayloadSchemaField{Name: "on", Type: model.PayloadSchemaFieldTypeBoolean},
			value:   "true",
			wantSub: "must be a boolean",
		},
		// object
		{
			name:      "object ok",
			field:     model.PayloadSchemaField{Name: "meta", Type: model.PayloadSchemaFieldTypeObject},
			value:     map[string]any{"k": "v"},
			wantEmpty: true,
		},
		{
			name:    "object wrong type",
			field:   model.PayloadSchemaField{Name: "meta", Type: model.PayloadSchemaFieldTypeObject},
			value:   []any{1, 2},
			wantSub: "must be an object",
		},
		// array
		{
			name:      "array ok",
			field:     model.PayloadSchemaField{Name: "items", Type: model.PayloadSchemaFieldTypeArray},
			value:     []any{1.0, 2.0},
			wantEmpty: true,
		},
		{
			name:    "array wrong type",
			field:   model.PayloadSchemaField{Name: "items", Type: model.PayloadSchemaFieldTypeArray},
			value:   map[string]any{},
			wantSub: "must be an array",
		},
		// unknown type falls through to pass (no constraint enforced)
		{
			name:      "unknown type passes through",
			field:     model.PayloadSchemaField{Name: "x", Type: "geo"},
			value:     "anything",
			wantEmpty: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validatePayloadFieldValue(tc.field, tc.value)
			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("expected pass, got error: %q", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("expected error containing %q, got pass", tc.wantSub)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("error %q does not contain %q", got, tc.wantSub)
			}
		})
	}
}
