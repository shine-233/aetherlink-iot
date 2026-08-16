// 文件用途：维护 plugin\admin\web_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminCredentialsMatchRequiresExplicitEnvironment(t *testing.T) {
	t.Setenv("GMQTT_ADMIN_USERNAME", "")
	t.Setenv("GMQTT_ADMIN_PASSWORD", "")

	if adminCredentialsMatch("admin", "admin") {
		t.Fatal("built-in default credentials must not be accepted")
	}

	t.Setenv("GMQTT_ADMIN_USERNAME", "operator")
	t.Setenv("GMQTT_ADMIN_PASSWORD", "change-me")

	if !adminCredentialsMatch("operator", "change-me") {
		t.Fatal("explicit admin environment credentials should be accepted")
	}
	if adminCredentialsMatch("admin", "admin") {
		t.Fatal("built-in default credentials should remain rejected")
	}
}

func TestSessionCookiesUseSecureAttribute(t *testing.T) {
	tests := []struct {
		name string
		set  func(http.ResponseWriter)
	}{
		{name: "set", set: setSessionCookie},
		{name: "clear", set: clearSessionCookie},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tt.set(recorder)
			cookies := recorder.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookies = %d, want 1", len(cookies))
			}
			cookie := cookies[0]
			if !cookie.Secure {
				t.Fatal("session cookie must set Secure")
			}
			if !cookie.HttpOnly {
				t.Fatal("session cookie must set HttpOnly")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
			}
			if tt.name == "clear" && cookie.MaxAge != -1 {
				t.Fatalf("clear cookie MaxAge = %d, want -1", cookie.MaxAge)
			}
		})
	}
}
