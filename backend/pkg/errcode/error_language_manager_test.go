// 文件用途：验证错误码对象、多语言解析和错误消息管理器行为。
// 核心逻辑：创建临时 YAML 配置，覆盖构造函数、语言权重排序、默认兜底、缓存清理和错误码边界。
// 关键注意事项：测试使用临时文件模拟配置，能证明配置读取规则但不覆盖生产配置文件完整性。
// 重构建议：后续可增加真实消息配置的静态校验测试，确保新增错误码都有多语言文案。
package errcode

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestErrorConstructorsPreserveCodeDataMessagesAndVariables(t *testing.T) {
	if got := New(CodeParamError); got.Code != CodeParamError || got.Data != nil || got.UseCustomMsg {
		t.Fatalf("New = %+v", got)
	}
	if got := New(CodeParamError).Error(); got != "Error Code: 100002" {
		t.Fatalf("Error() = %q", got)
	}

	custom := NewWithMessage(CodeUnauthorized, "login required")
	if custom.Code != CodeUnauthorized || custom.CustomMsg != "login required" || !custom.UseCustomMsg {
		t.Fatalf("NewWithMessage = %+v", custom)
	}

	data := map[string]any{"field": "email"}
	withData := WithData(CodeParamError, data)
	if withData.Code != CodeParamError || !reflect.DeepEqual(withData.Data, data) {
		t.Fatalf("WithData = %+v", withData)
	}

	formatted := Newf(CodeDBError, "devices", 12)
	if formatted.Code != CodeDBError || !reflect.DeepEqual(formatted.Args, []interface{}{"devices", 12}) {
		t.Fatalf("Newf = %+v", formatted)
	}

	vars := map[string]interface{}{"invalid_chars": "#"}
	withVars := WithVars(200053, vars)
	if withVars.Code != 200053 || !reflect.DeepEqual(withVars.Variables, vars) {
		t.Fatalf("WithVars = %+v", withVars)
	}
}

func TestParseAcceptLanguageSortsByWeightAndNormalizesTags(t *testing.T) {
	langs := ParseAcceptLanguage("fr-FR, zh-CN;q=0.9, en-US;q=0.8, invalid;q=bad, es;q=0.7")
	if len(langs) != 5 {
		t.Fatalf("ParseAcceptLanguage length = %d, want 5", len(langs))
	}
	if langs[0].Tag != "fr-FR" || langs[0].Weight != 1 {
		t.Fatalf("first language = %+v, want fr-FR weight 1", langs[0])
	}
	if langs[1].Tag != "invalid" || langs[1].Weight != 1 {
		t.Fatalf("invalid q should keep default weight 1, got %+v", langs[1])
	}
	if langs[2].Tag != "zh-CN" || langs[2].Weight != 0.9 {
		t.Fatalf("third language = %+v, want zh-CN weight 0.9", langs[2])
	}
	if got := ParseAcceptLanguage(""); got != nil {
		t.Fatalf("ParseAcceptLanguage empty = %#v, want nil", got)
	}

	if got := NormalizeLanguage("zh-CN;q=0.9"); got != "zh_CN" {
		t.Fatalf("NormalizeLanguage = %q, want zh_CN", got)
	}
	if got := NormalizeLanguage("en-US"); got != "en_US" {
		t.Fatalf("NormalizeLanguage = %q, want en_US", got)
	}
}

func TestErrorManagerLoadsMessagesAndFallsBackByLanguage(t *testing.T) {
	dir := t.TempDir()
	codePath := filepath.Join(dir, "codes.yaml")
	stringPath := filepath.Join(dir, "strings.yaml")
	if err := os.WriteFile(codePath, []byte(`
metadata:
  version: "1"
  last_updated: "2026-06-27"
  supported_languages: ["zh_CN", "en_US"]
messages:
  200:
    zh_CN: "success-cn"
    en_US: "success"
  100002:
    zh_CN: "param-cn"
    en_US: "param-en"
`), 0o600); err != nil {
		t.Fatalf("write code yaml: %v", err)
	}
	if err := os.WriteFile(stringPath, []byte(`
metadata:
  version: "1"
  last_updated: "2026-06-27"
  supported_languages: ["zh_CN", "en_US"]
messages:
  login.required:
    zh_CN: "login-cn"
    en_US: "login-en"
`), 0o600); err != nil {
		t.Fatalf("write string yaml: %v", err)
	}

	manager := NewErrorManager(codePath, stringPath)
	if err := manager.LoadMessages(); err != nil {
		t.Fatalf("LoadMessages returned error: %v", err)
	}

	if got := manager.GetMessage(100002, "fr-FR, en-US;q=0.8"); got != "param-en" {
		t.Fatalf("GetMessage fallback = %q, want param-en", got)
	}
	if got := manager.GetMessage(100002, ""); got != "param-cn" {
		t.Fatalf("GetMessage default = %q, want param-cn", got)
	}
	if got := manager.GetMessageStr("login.required", "en-US"); got != "login-en" {
		t.Fatalf("GetMessageStr en = %q, want login-en", got)
	}
	if got := manager.GetMessageStr("missing.key", ""); got != "missing.key" {
		t.Fatalf("GetMessageStr missing = %q, want key fallback", got)
	}

	manager.SetDefaultLanguage("en_US")
	if got := manager.GetMessage(999999, ""); got != "Unknown Error" {
		t.Fatalf("GetMessage unknown en = %q, want Unknown Error", got)
	}
	manager.ClearCache()
	if got := manager.GetMessage(100002, "zh-CN"); got != "param-cn" {
		t.Fatalf("GetMessage after ClearCache = %q, want param-cn", got)
	}
}

func TestErrorManagerLoadMessagesRejectsMissingInvalidYamlAndInvalidCodes(t *testing.T) {
	dir := t.TempDir()
	stringPath := filepath.Join(dir, "strings.yaml")
	if err := os.WriteFile(stringPath, []byte("messages: {}\n"), 0o600); err != nil {
		t.Fatalf("write string yaml: %v", err)
	}

	if err := NewErrorManager(filepath.Join(dir, "missing.yaml"), stringPath).LoadMessages(); err == nil {
		t.Fatal("LoadMessages expected error for missing code yaml")
	}

	invalidYaml := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalidYaml, []byte("messages: ["), 0o600); err != nil {
		t.Fatalf("write invalid yaml: %v", err)
	}
	if err := NewErrorManager(invalidYaml, stringPath).LoadMessages(); err == nil {
		t.Fatal("LoadMessages expected parse error for invalid yaml")
	}

	invalidCode := filepath.Join(dir, "invalid-code.yaml")
	if err := os.WriteFile(invalidCode, []byte(`
messages:
  99999:
    zh_CN: "bad"
`), 0o600); err != nil {
		t.Fatalf("write invalid code yaml: %v", err)
	}
	if err := NewErrorManager(invalidCode, stringPath).LoadMessages(); err == nil {
		t.Fatal("LoadMessages expected invalid code error")
	}
}

func TestErrorManagerValidateCodeBoundaries(t *testing.T) {
	manager := NewErrorManager("", "")
	valid := []int{200, 100000, 199999, 200001, 300001, 400001, 500001, 599999}
	for _, code := range valid {
		if !manager.validateCode(code) {
			t.Fatalf("validateCode(%d) = false, want true", code)
		}
	}

	invalid := []int{0, 199, 99999, 600000, 900000}
	for _, code := range invalid {
		if manager.validateCode(code) {
			t.Fatalf("validateCode(%d) = true, want false", code)
		}
	}
}
