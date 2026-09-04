// 文件用途：覆盖 CasbinRBAC 中间件的注册表判定与拒绝行为。
// 核心逻辑：使用与 configs/casbin.conf 一致的内存模型构造 g2 资源注册、角色绑定和 allow 策略，验证未注册 URL 放行、已注册未授权返回 403。
// 关键注意事项：中间件以去掉前导斜杠后的路径查注册表，g2/p 策略数据须按同一格式登记；拒绝响应体 gin.H{"error": "非法访问"} 是前端解析契约，状态码固定 403。
// 重构建议：后续可补充路径归一化（多斜杠前缀）和策略加载失败场景。

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func setupCasbinRBACEnforcer(t *testing.T) {
	t.Helper()

	// 与 configs/casbin.conf 保持一致：matcher 双通道（g2 分组 + urlPatternMatch 锚定模式），
	// urlPatternMatch 与生产同源（utils.URLPatternCasbinFunction）。
	modelStr := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _
g2 = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (g2(r.obj, p.obj) || urlPatternMatch(r.obj, p.obj)) && r.act == p.act
`
	casbinModel, err := model.NewModelFromString(modelStr)
	if err != nil {
		t.Fatalf("build test casbin model: %v", err)
	}

	enforcer, err := casbin.NewEnforcer(casbinModel)
	if err != nil {
		t.Fatalf("create test casbin enforcer: %v", err)
	}
	// casbin v2.135 的 AddFunction 无返回值（失败以其内部 panic 暴露）。
	enforcer.AddFunction("urlPatternMatch", utils.URLPatternCasbinFunction())

	oldEnforcer := global.CasbinEnforcer
	global.CasbinEnforcer = enforcer
	t.Cleanup(func() {
		global.CasbinEnforcer = oldEnforcer
	})

	// 中间件用 TrimLeft("/") 后的路径查表，g2 资源按同一格式登记：只有登记过的 URL 才需要校验
	if _, err := enforcer.AddNamedGroupingPolicies("g2", [][]string{
		{"api/v1/devices", "device-resource"},
		{"telemetry/ws", "ws-resource"},
	}); err != nil {
		t.Fatalf("register g2 resources: %v", err)
	}
	if _, err := enforcer.AddNamedGroupingPolicies("g", [][]string{
		{"user-device-admin", "role-device"},
	}); err != nil {
		t.Fatalf("bind user roles: %v", err)
	}
	if _, err := enforcer.AddNamedPolicies("p", [][]string{
		{"role-device", "device-resource", "allow"},
	}); err != nil {
		t.Fatalf("add allow policies: %v", err)
	}
}

func newCasbinRBACTestRouter(claims *utils.UserClaims) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("claims", claims)
		c.Next()
	})
	router.Use(CasbinRBAC())
	router.GET("/api/v1/devices", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/v1/not-in-casbin", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/telemetry/ws", func(c *gin.Context) { c.Status(http.StatusOK) })
	return router
}

func TestCasbinRBACAllowsUnregisteredURLsAndAuthorizedUsers(t *testing.T) {
	setupCasbinRBACEnforcer(t)
	router := newCasbinRBACTestRouter(&utils.UserClaims{ID: "user-device-admin"})

	tests := []struct {
		name string
		path string
	}{
		{name: "registered url with authorized role", path: "/api/v1/devices"},
		{name: "unregistered url passes without verification", path: "/api/v1/not-in-casbin"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tc.path, recorder.Code, http.StatusOK)
			}
		})
	}
}

func TestCasbinRBACRejectsUnauthorizedUserWith403AndLegacyBody(t *testing.T) {
	setupCasbinRBACEnforcer(t)
	router := newCasbinRBACTestRouter(&utils.UserClaims{ID: "user-without-role"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d for unauthorized registered url", recorder.Code, http.StatusForbidden)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
	}
	if body["error"] != "非法访问" {
		t.Fatalf(`error = %q, want "非法访问"`, body["error"])
	}
}

func TestCasbinRBACVerifiesRegisteredNonAPIPrefixedPathsToo(t *testing.T) {
	setupCasbinRBACEnforcer(t)
	// 路径不含 "api" 子串但已在注册表中，同样必须走校验（旧的 Contains 预过滤会跳过它）
	if ok, err := global.CasbinEnforcer.AddNamedPolicies("p", [][]string{
		{"role-ws", "ws-resource", "allow"},
	}); err != nil || !ok {
		t.Fatalf("add ws allow policy: ok=%v err=%v", ok, err)
	}
	router := newCasbinRBACTestRouter(&utils.UserClaims{ID: "user-without-role"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/telemetry/ws", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d for registered non-api-prefixed path", recorder.Code, http.StatusForbidden)
	}
}

func TestCasbinRBACSkipsVerificationWithoutEnforcer(t *testing.T) {
	oldEnforcer := global.CasbinEnforcer
	global.CasbinEnforcer = nil
	t.Cleanup(func() {
		global.CasbinEnforcer = oldEnforcer
	})
	router := newCasbinRBACTestRouter(&utils.UserClaims{ID: "user-any"})
	// enforcer 缺失时 GetUrl 返回 false，未注册即不校验

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d when casbin enforcer is unavailable", recorder.Code, http.StatusOK)
	}
}

func TestCasbinRBACDeniesUnregisteredWhenStrictMode(t *testing.T) {
	setupCasbinRBACEnforcer(t)
	viper.Set("casbin.deny-unregistered", true)
	t.Cleanup(func() { viper.Reset() })
	router := newCasbinRBACTestRouter(&utils.UserClaims{ID: "user-device-admin"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/not-in-casbin", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d for unregistered url in strict mode", recorder.Code, http.StatusForbidden)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
	}
	if body["error"] != "非法访问" {
		t.Fatalf(`error = %q, want "非法访问"（与已注册未授权保持同一拒绝契约）`, body["error"])
	}
}

func TestCasbinRBACStrictModeKeepsRegisteredAuthorizedPass(t *testing.T) {
	setupCasbinRBACEnforcer(t)
	viper.Set("casbin.deny-unregistered", true)
	t.Cleanup(func() { viper.Reset() })
	router := newCasbinRBACTestRouter(&utils.UserClaims{ID: "user-device-admin"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d for registered+authorized url in strict mode", recorder.Code, http.StatusOK)
	}
}

func TestCasbinRBACStrictModePatternRegisteredRouteGoesToVerify(t *testing.T) {
	setupCasbinRBACEnforcer(t)
	viper.Set("casbin.deny-unregistered", true)
	t.Cleanup(func() { viper.Reset() })
	// 登记参数模式（p 策略 obj 仍是资源组 device-resource）→ GetUrl 模式命中 → 走 Verify；
// 但具体路径既不在 g2 组内也无匹配模式策略 → Enforce 拒绝 403（验证严格模式下判定链完整走通）
	if _, err := global.CasbinEnforcer.AddNamedGroupingPolicies("g2", [][]string{
		{"api/v1/device/:id", "device-resource"},
	}); err != nil {
		t.Fatalf("register param pattern: %v", err)
	}
	router := newCasbinRBACTestRouter(&utils.UserClaims{ID: "user-device-admin"})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/device/123", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (pattern-registered but unauthorized)", recorder.Code, http.StatusForbidden)
	}
}
