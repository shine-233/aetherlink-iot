// 文件用途：覆盖 password utils 工具函数的 Go 测试。
// 核心逻辑：通过表驱动或边界用例验证通用工具的输入校验、格式转换和错误返回，主要围绕 func TestValidatePasswordRDIPolicy 等声明展开。
// 关键注意事项：工具包被多处业务代码复用，测试断言需保持跨调用方的兼容契约。
// 重构建议：后续可按工具类别拆分公共夹具，并补充失败路径和异常输入覆盖。

package utils

import (
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
)

func TestValidatePasswordRDIPolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantCode int
		wantVars map[string]interface{}
	}{
		{name: "valid minimum length", password: "Aa1!aaaa"},
		{name: "valid maximum length", password: "Aa1!aaaaaaaaaaaaaaaa"},
		{name: "too short", password: "Aa1!aaa", wantCode: 200040},
		{name: "too long", password: "Aa1!aaaaaaaaaaaaaaaaa", wantCode: 200040},
		{name: "missing uppercase", password: "aa1!aaaa", wantCode: 200054, wantVars: map[string]interface{}{"missing_elements": "大写字母"}},
		{name: "missing lowercase", password: "AA1!AAAA", wantCode: 200054, wantVars: map[string]interface{}{"missing_elements": "小写字母"}},
		{name: "missing number", password: "Aaa!aaaa", wantCode: 200054, wantVars: map[string]interface{}{"missing_elements": "数字"}},
		{name: "missing special", password: "Aa11aaaa", wantCode: 200054, wantVars: map[string]interface{}{"missing_elements": "特殊字符"}},
		{name: "missing multiple classes preserve order", password: "aaaaaaaa", wantCode: 200054, wantVars: map[string]interface{}{"missing_elements": "大写字母、数字、特殊字符"}},
		{name: "invalid character", password: "Aa1!aaa中", wantCode: 200053, wantVars: map[string]interface{}{"invalid_chars": "中"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("ValidatePassword(%q) returned error: %v", tt.password, err)
				}
				return
			}
			appErr, ok := err.(*errcode.Error)
			if !ok {
				t.Fatalf("ValidatePassword(%q) error type = %T, want *errcode.Error", tt.password, err)
			}
			if appErr.Code != tt.wantCode {
				t.Fatalf("ValidatePassword(%q) code = %d, want %d", tt.password, appErr.Code, tt.wantCode)
			}
			for key, want := range tt.wantVars {
				if got := appErr.Variables[key]; got != want {
					t.Fatalf("ValidatePassword(%q) variable %s = %#v, want %#v", tt.password, key, got, want)
				}
			}
		})
	}
}
