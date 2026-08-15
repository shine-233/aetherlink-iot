// 文件用途：覆盖 HTTP 中间件 operations log 行为的 Go 测试。
// 核心逻辑：通过请求上下文、响应状态和边界输入验证认证、跨域、日志或安全处理逻辑，主要围绕 func TestSanitizeLogMessageRedactsNestedJSONSecrets、func TestSanitizeLogMessageRedactsFormLikeSecrets、func TestSanitizeLogMessageTruncatesLargePayload 等声明展开。
// 关键注意事项：中间件测试需保持状态码、上下文键和错误响应格式与客户端契约一致。
// 重构建议：后续可统一测试路由和上下文构造，减少重复的请求搭建代码。

package middleware

import (
	"strings"
	"testing"
)

func TestSanitizeLogMessageRedactsNestedJSONSecrets(t *testing.T) {
	got := sanitizeLogMessage(`{"email":"user@example.com","password":"secret","nested":{"access_token":"abc","api_key":"key"},"items":[{"voucher":"v1"}]}`)

	for _, leaked := range []string{`"password":"secret"`, `"access_token":"abc"`, `"api_key":"key"`, `"voucher":"v1"`} {
		if strings.Contains(got, leaked) {
			t.Fatalf("sanitizeLogMessage leaked %q in %s", leaked, got)
		}
	}
	if count := strings.Count(got, redactedValue); count != 4 {
		t.Fatalf("redacted value count = %d, want 4 in %s", count, got)
	}
}

func TestSanitizeLogMessageRedactsFormLikeSecrets(t *testing.T) {
	got := sanitizeLogMessage(`username=device&password=secret&token=abc`)

	if strings.Contains(got, "secret") || strings.Contains(got, "abc") {
		t.Fatalf("sanitizeLogMessage leaked form-like secret in %s", got)
	}
}

func TestSanitizeLogMessageTruncatesLargePayload(t *testing.T) {
	got := sanitizeLogMessage(strings.Repeat("a", maxLoggedBodyBytes+100))

	if len(got) <= maxLoggedBodyBytes || !strings.HasSuffix(got, "...[truncated]") {
		t.Fatalf("sanitizeLogMessage did not truncate as expected, len=%d", len(got))
	}
}

func TestPasswordRecoveryEndpointsNeverCaptureBodies(t *testing.T) {
	paths := []string{
		"/api/v1/reset/password/link",
		"/api/v1/reset/password",
	}
	for _, path := range paths {
		if got := operationLogCaptureModeFor("POST", path); got != operationLogCaptureNone {
			t.Errorf("POST %s capture mode = %v, want none", path, got)
		}
	}
}

func TestMQTTDebugOperationsUseMetadataOnlyCapture(t *testing.T) {
	paths := []string{
		"/api/v1/device/device-1/mqtt-debug/session",
		"/api/v1/device/device-1/mqtt-debug/session/session-1/command",
		"/api/v1/device/device-1/mqtt-debug/session/session-1",
	}
	for _, path := range paths {
		if got := operationLogCaptureModeFor("POST", path); got != operationLogCaptureMetadataOnly {
			t.Fatalf("POST %s capture mode = %v, want metadata only", path, got)
		}
		if got := operationLogCaptureModeFor("DELETE", path); got != operationLogCaptureMetadataOnly {
			t.Fatalf("DELETE %s capture mode = %v, want metadata only", path, got)
		}
	}
	if isDeviceMQTTDebugPath("/api/v1/device/mqtt-debug/session") {
		t.Fatal("path without a device segment must not match mqtt debug policy")
	}
}
