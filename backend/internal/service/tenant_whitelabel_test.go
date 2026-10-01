// 文件用途：白标服务（TB-47）校验/合并纯函数与租户作用域解析的单测。
// 核心逻辑：不依赖数据库，直接驱动 NormalizeTenantTranslationLang /
//
//	ValidateTenantTranslationKey / ValidateTenantTranslationValue /
//	SanitizeTenantCustomCSS / ApplyTranslationOverrides /
//	BuildTenantWhitelabelOverrides 与 whitelabelWriteScope：
//	覆盖 lang 白名单、key 形态、value/css 内容约束、合并优先级与角色矩阵的
//	正负用例（TENANT_USER 拒绝、TENANT_ADMIN 缺租户拒绝、SYS_ADMIN 全局空串放行）。
//
// 关键注意事项：这里断的是业务码而非文案，避免绑定 messages.yaml 措辞；
//
//	SanitizeTenantCustomCSS 的 '</style' 拒绝是纵深防御断言——最终 XSS 防线
//	仍是前端 textContent 注入（禁止 innerHTML），两道防线缺一不可。
//
// 重构建议：若校验口径调整（如放开语言白名单），先改本文件的负向用例预期，
//
//	再动实现，保证口径变化显式过审。
package service

import (
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// TestNormalizeTenantTranslationLang 语言白名单：四语言放行（大小写/空白容错），其余拒绝。
func TestNormalizeTenantTranslationLang(t *testing.T) {
	valid := map[string]string{
		"zh-cn": "zh-cn", " en-us ": "en-us", "ES-ES": "es-es", "FR-FR": "fr-fr",
	}
	for input, want := range valid {
		got, ok := NormalizeTenantTranslationLang(input)
		if !ok || got != want {
			t.Fatalf("NormalizeTenantTranslationLang(%q) = (%q,%v), want (%q,true)", input, got, ok, want)
		}
	}
	for _, input := range []string{"", "zh_CN", "de-de", "en", "zh-cn-x-custom", "zh-CN;"} {
		if got, ok := NormalizeTenantTranslationLang(input); ok {
			t.Fatalf("NormalizeTenantTranslationLang(%q) = (%q,true), want reject", input, got)
		}
	}
}

// TestValidateTenantTranslationKey 词条键形态：合法键放行，非法形态逐类拒绝。
func TestValidateTenantTranslationKey(t *testing.T) {
	valid := []string{"page.customer.title", "route.mobile-app_app-center", "A1", "key-with.dots_and-dash"}
	for _, key := range valid {
		if err := ValidateTenantTranslationKey(key); err != nil {
			t.Fatalf("ValidateTenantTranslationKey(%q) unexpected error: %v", key, err)
		}
	}
	invalid := map[string]string{
		"":                       "empty",
		" ":                      "blank",
		".leading":               "leading dot",
		"trailing.":              "trailing dot",
		"dou..ble":               "double dot",
		"with space":             "space",
		"sla/sh":                 "slash",
		"键":                      "non-ascii",
		strings.Repeat("a", 201): "too long",
	}
	for key, label := range invalid {
		err := ValidateTenantTranslationKey(key)
		if err == nil {
			t.Fatalf("ValidateTenantTranslationKey(%q) [%s] expected error, got nil", key, label)
		}
		if code := errCodeOf(err); code != errcode.CodeTenantTranslationInvalid {
			t.Fatalf("ValidateTenantTranslationKey(%q) [%s] code = %d, want %d", key, label, code, errcode.CodeTenantTranslationInvalid)
		}
	}
}

// TestValidateTenantTranslationValue 译文内容：长度与控制字符约束。
func TestValidateTenantTranslationValue(t *testing.T) {
	long := strings.Repeat("字", 4001)
	invalid := map[string]string{
		"":            "empty",
		long:          "too long",
		"bad\x00char": "nul control char",
		"bad\x07bell": "bell control char",
	}
	for value, label := range invalid {
		if err := ValidateTenantTranslationValue(value); err == nil {
			t.Fatalf("ValidateTenantTranslationValue [%s] expected error, got nil", label)
		}
	}
	valid := []string{"客户", "line1\nline2", "tab\tsep", strings.Repeat("a", 4000)}
	for _, value := range valid {
		if err := ValidateTenantTranslationValue(value); err != nil {
			t.Fatalf("ValidateTenantTranslationValue(%q) unexpected error: %v", value, err)
		}
	}
}

// TestSanitizeTenantCustomCSS 自定义 CSS：裁剪/限长/</style 拒绝/控制字符拒绝/空串合法。
func TestSanitizeTenantCustomCSS(t *testing.T) {
	css, err := SanitizeTenantCustomCSS("  .app { color: red; }\n")
	if err != nil || css != ".app { color: red; }" {
		t.Fatalf("trim failed: css=%q err=%v", css, err)
	}
	if css, err := SanitizeTenantCustomCSS(""); err != nil || css != "" {
		t.Fatalf("empty css must be allowed (clear semantics), got %q err=%v", css, err)
	}
	if _, err := SanitizeTenantCustomCSS(strings.Repeat("a", 65537)); err == nil {
		t.Fatal("css over 65536 must be rejected")
	}
	for _, malicious := range []string{".a{}</style><script>alert(1)</script>", "</STYLE>"} {
		_, err := SanitizeTenantCustomCSS(malicious)
		if err == nil {
			t.Fatalf("css %q must be rejected", malicious)
		}
		if code := errCodeOf(err); code != errcode.CodeTenantCustomCSSInvalid {
			t.Fatalf("css %q code = %d, want %d", malicious, code, errcode.CodeTenantCustomCSSInvalid)
		}
	}
	if _, err := SanitizeTenantCustomCSS(".a{}\x07"); err == nil {
		t.Fatal("css with control char must be rejected")
	}
	if _, err := SanitizeTenantCustomCSS(".a{}\r\n.b{}\t"); err != nil {
		t.Fatalf("css with newline/cr/tab must be allowed: %v", err)
	}
}

// TestApplyTranslationOverrides 合并语义：覆盖项胜出、基础项保留、入参不被改写、双方为空可用。
func TestApplyTranslationOverrides(t *testing.T) {
	base := map[string]string{"page.customer.title": "客户", "page.device.title": "设备"}
	overrides := map[string]string{"page.customer.title": "客户中心", "page.extra.title": "附加"}
	merged := ApplyTranslationOverrides(base, overrides)
	if merged["page.customer.title"] != "客户中心" || merged["page.device.title"] != "设备" || merged["page.extra.title"] != "附加" {
		t.Fatalf("merge priority wrong: %v", merged)
	}
	// 合并不得改写入参（前端 i18n 目录复用同一份静态 catalog）。
	if base["page.customer.title"] != "客户" {
		t.Fatalf("base mutated: %v", base)
	}
	empty := ApplyTranslationOverrides(base, nil)
	if len(empty) != 2 || empty["page.device.title"] != "设备" {
		t.Fatalf("nil overrides must copy base: %v", empty)
	}
	fromNil := ApplyTranslationOverrides(nil, overrides)
	if len(fromNil) != 2 || fromNil["page.extra.title"] != "附加" {
		t.Fatalf("nil base must copy overrides: %v", fromNil)
	}
}

// TestBuildTenantWhitelabelOverrides 装配：按语言分组、空输入为空 map（非 nil）。
func TestBuildTenantWhitelabelOverrides(t *testing.T) {
	rows := buildOverridesFixtureRows()
	resp := BuildTenantWhitelabelOverrides("tenant-a", rows, ".app{}")
	if resp.TenantID != "tenant-a" || resp.CSS != ".app{}" {
		t.Fatalf("scalar fields wrong: %+v", resp)
	}
	if resp.Translations["zh-cn"]["page.customer.title"] != "客户中心" ||
		resp.Translations["en-us"]["page.customer.title"] != "Customer" {
		t.Fatalf("grouping wrong: %+v", resp.Translations)
	}
	if len(resp.Translations) != 2 {
		t.Fatalf("expected 2 langs, got %d", len(resp.Translations))
	}
	empty := BuildTenantWhitelabelOverrides("tenant-a", nil, "")
	if empty.Translations == nil || len(empty.Translations) != 0 {
		t.Fatalf("empty rows must produce empty (non-nil) map: %+v", empty.Translations)
	}
}

// buildOverridesFixtureRows 构造两语言的覆盖行（多测试复用）。
func buildOverridesFixtureRows() []*model.TenantTranslation {
	return []*model.TenantTranslation{
		{TenantID: "tenant-a", Lang: "zh-cn", Key: "page.customer.title", Value: "客户中心"},
		{TenantID: "tenant-a", Lang: "en-us", Key: "page.customer.title", Value: "Customer"},
	}
}

// TestWhitelabelWriteScope 写作用域角色矩阵：SYS_ADMIN 全局空串、TENANT_ADMIN 本租户、
// TENANT_USER 与缺凭证拒绝（fail-closed）。
func TestWhitelabelWriteScope(t *testing.T) {
	cases := []struct {
		name      string
		claims    *utils.UserClaims
		wantScope string
		wantCode  int
	}{
		{name: "SYS_ADMIN 全局空串", claims: &utils.UserClaims{Authority: "SYS_ADMIN"}, wantScope: ""},
		{name: "TENANT_ADMIN 本租户", claims: &utils.UserClaims{Authority: "TENANT_ADMIN", TenantID: "tenant-a"}, wantScope: "tenant-a"},
		{name: "TENANT_ADMIN 缺租户上下文", claims: &utils.UserClaims{Authority: "TENANT_ADMIN"}, wantCode: errcode.CodeNoPermission},
		{name: "TENANT_USER 无写权限", claims: &utils.UserClaims{Authority: "TENANT_USER", TenantID: "tenant-a"}, wantCode: errcode.CodeNoPermission},
		{name: "缺凭证", claims: nil, wantCode: errcode.CodeNoPermission},
	}
	for _, tc := range cases {
		scope, err := whitelabelWriteScope(tc.claims)
		if tc.wantCode != 0 {
			if code := errCodeOf(err); code != tc.wantCode {
				t.Fatalf("%s: code = %d, want %d", tc.name, code, tc.wantCode)
			}
			continue
		}
		if err != nil || scope != tc.wantScope {
			t.Fatalf("%s: scope = %q err = %v, want %q", tc.name, scope, err, tc.wantScope)
		}
	}
}

// TestWhitelabelReadScope 读作用域：任何登录用户读本作用域，缺凭证拒绝。
func TestWhitelabelReadScope(t *testing.T) {
	if scope, err := whitelabelReadScope(&utils.UserClaims{Authority: "TENANT_USER", TenantID: "tenant-a"}); err != nil || scope != "tenant-a" {
		t.Fatalf("TENANT_USER read scope = %q err = %v, want tenant-a", scope, err)
	}
	if _, err := whitelabelReadScope(nil); err == nil {
		t.Fatal("missing claims must be rejected for read scope")
	}
}
