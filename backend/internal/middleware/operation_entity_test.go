// 文件用途：覆盖 TB-10 实体级审计纯函数解析器（operation_entity.go）的 Go 测试。
// 核心逻辑：对 operationActionForMethod 与 operationEntityForPath 做表驱动断言，
// 覆盖方法映射全分支、路径形态（前缀/缺ID/子资源/动词段/脱敏段/超长段）与 UUID 判定边界。
// 关键注意事项：解析器是落库语义的唯一来源，用例变更需与 127.sql 列宽、
// operations_log_test.go 的捕获策略用例保持同一口径。
// 重构建议：若解析规则演进（如支持非 UUID 主键），先扩表驱动用例再改实现。

package middleware

import (
	"strings"
	"testing"
)

func TestOperationActionForMethod(t *testing.T) {
	cases := []struct {
		method string
		want   string
	}{
		{"POST", "create"},
		{"post", "create"},
		{" PUT ", "update"},
		{"PUT", "update"},
		{"PATCH", "update"},
		{"DELETE", "delete"},
		{"delete", "delete"},
		{"GET", "read"},
		{"HEAD", "read"},
		{"OPTIONS", "other"},
		{"TRACE", "other"},
		{"", "other"},
	}
	for _, tc := range cases {
		if got := operationActionForMethod(tc.method); got != tc.want {
			t.Errorf("operationActionForMethod(%q) = %q, want %q", tc.method, got, tc.want)
		}
	}
}

func TestOperationEntityForPath(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		wantType string
		wantID   string
	}{
		{"create without id", "/api/v1/customer", "customer", ""},
		{"update with uuid id", "/api/v1/customer/6ba7b810-9dad-11d1-80b4-00c04fd430c8", "customer", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
		{"sub-resource keeps entity pair", "/api/v1/customer/6ba7b810-9dad-11d1-80b4-00c04fd430c8/devices", "customer", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
		{"device delete with uppercase uuid", "/api/v1/device/6BA7B810-9DAD-11D1-80B4-00C04FD430C8", "device", "6BA7B810-9DAD-11D1-80B4-00C04FD430C8"},
		{"verb second segment is not an id", "/api/v1/device/update/voucher", "device", ""},
		{"verb second segment password", "/api/v1/board/update/password", "board", ""},
		{"redacted share token is not an id", "/api/v1/rdi/share-tokens/[REDACTED]", "rdi", ""},
		{"non-api path yields empty", "/login", "", ""},
		{"prefix only yields empty", "/api/v1/", "", ""},
		{"empty path yields empty", "", "", ""},
		{"api root without trailing slash yields empty", "/api/v1", "", ""},
		{"oversized entity type truncated", "/api/v1/" + repeatByte('a', 80), repeatByte('a', 64), ""},
	}
	for _, tc := range cases {
		gotType, gotID := operationEntityForPath(tc.path)
		if gotType != tc.wantType || gotID != tc.wantID {
			t.Errorf("%s: operationEntityForPath(%q) = (%q, %q), want (%q, %q)",
				tc.name, tc.path, gotType, gotID, tc.wantType, tc.wantID)
		}
	}
}

func TestIsUUIDShape(t *testing.T) {
	valid := []string{
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"6BA7B810-9DAD-11D1-80B4-00C04FD430C8",
		"00000000-0000-0000-0000-000000000000",
	}
	for _, s := range valid {
		if !isUUIDShape(s) {
			t.Errorf("isUUIDShape(%q) = false, want true", s)
		}
	}
	invalid := []string{
		"",
		"update",
		"6ba7b8109dad11d180b400c04fd430c8",      // 无连字符
		"6ba7b810-9dad-11d1-80b4-00c04fd430c",   // 35 位
		"6ba7b810-9dad-11d1-80b4-00c04fd430c80", // 37 位
		"6ba7b810_9dad-11d1-80b4-00c04fd430c8",  // 连字符位置错
		"6ba7b810-9dad-11d1-80b4-00c04fd430cz",  // 非十六进制
		"[REDACTED]",
		"/api/v1/customer",
	}
	for _, s := range invalid {
		if isUUIDShape(s) {
			t.Errorf("isUUIDShape(%q) = true, want false", s)
		}
	}
}

func TestOperationEntityFromRedactedShareTokenPath(t *testing.T) {
	// 落库语义串联：脱敏路径 → 实体解析。rdi/share-tokens 的真实 token
	// 在 safeOperationLogPath 已替换为 [REDACTED]，解析结果不得再含原始 token。
	raw := "/api/v1/rdi/share-tokens/9f8e7d6c-secret-token/accept"
	redacted := safeOperationLogPath(raw)
	gotType, gotID := operationEntityForPath(redacted)
	if gotType != "rdi" || gotID != "" {
		t.Fatalf("entity from redacted path = (%q, %q), want (rdi, \"\")", gotType, gotID)
	}
	if strings.Contains(gotID, "secret") {
		t.Fatalf("raw token leaked into entity_id: %q", gotID)
	}
	// 未脱敏路径直接解析也会得到空 ID（token 非 UUID 形态），证明 UUID 门闩兜底了脱敏缺口。
	if _, rawID := operationEntityForPath(raw); rawID != "" {
		t.Fatalf("non-uuid token must not become entity_id, got %q", rawID)
	}
}

// repeatByte 生成 n 个 b 组成的串（表驱动用例里构造超长路径段）。
func repeatByte(b byte, n int) string {
	return strings.Repeat(string(b), n)
}
