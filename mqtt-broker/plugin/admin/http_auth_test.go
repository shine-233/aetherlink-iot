// 文件用途：验证 admin 管理面共享密钥认证与配置加载契约。
// 安全职责：http_auth_secret 非空时所有 admin HTTP API 必须携带匹配的
// X-Admin-Secret 头；为空时保持既有行为并对非回环绑定打印告警。

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
)

func newAdminTestRegistrar() *adminTestRegistrar {
	return &adminTestRegistrar{}
}

// installAdminTestLogger 用 nop logger 替换包级 logger，避免未走 Load 的测试触发 nil 日志。
func installAdminTestLogger(t *testing.T) {
	t.Helper()
	previousLog := log
	log = zap.NewNop()
	t.Cleanup(func() { log = previousLog })
}

type adminTestRegistrar struct {
	middleware       server.HTTPMiddleware
	grpcInterceptors []server.GRPCUnaryInterceptor
}

func (r *adminTestRegistrar) RegisterHTTPHandler(fn server.HTTPHandler) error { return nil }

func (r *adminTestRegistrar) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {}

func (r *adminTestRegistrar) SetHTTPMiddleware(mw server.HTTPMiddleware) {
	r.middleware = mw
}

func (r *adminTestRegistrar) AddGRPCUnaryInterceptor(ic server.GRPCUnaryInterceptor) {
	r.grpcInterceptors = append(r.grpcInterceptors, ic)
}

func TestAdminSecretMiddlewareRequiresHeaderWhenConfigured(t *testing.T) {
	a := &Admin{httpAuthSecret: "s3cret"}
	handler := a.adminSecretMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{name: "missing secret is rejected", want: http.StatusUnauthorized},
		{name: "wrong secret is rejected", headers: map[string]string{AdminSecretHeader: "wrong"}, want: http.StatusUnauthorized},
		{name: "matching secret is accepted", headers: map[string]string{AdminSecretHeader: "s3cret"}, want: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/v1/clients", nil)
			for key, value := range tt.headers {
				request.Header.Set(key, value)
			}
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}

	// 内置管理页会话 cookie 仍可访问，保证启用共享密钥后 dashboard 可用。
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/clients", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionCookieValue})
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("session cookie status = %d, want %d", recorder.Code, http.StatusOK)
	}

	// 无效会话 cookie 不能绕过共享密钥。
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/v1/clients", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "forged"})
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("forged cookie status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestSetupHTTPAuthInstallsMiddlewareOnlyWithSecret(t *testing.T) {
	installAdminTestLogger(t)
	registrar := newAdminTestRegistrar()
	cfg := config.DefaultConfig()
	cfg.Plugins[Name] = &Config{HTTPAuthSecret: "  s3cret  "}

	a := &Admin{}
	a.setupHTTPAuth(cfg, registrar)

	if a.httpAuthSecret != "s3cret" {
		t.Fatalf("httpAuthSecret = %q, want trimmed %q", a.httpAuthSecret, "s3cret")
	}
	if registrar.middleware == nil {
		t.Fatal("middleware should be installed when http_auth_secret is configured")
	}
}

func TestSetupHTTPAuthKeepsLegacyBehaviorWithoutSecret(t *testing.T) {
	installAdminTestLogger(t)
	registrar := newAdminTestRegistrar()

	a := &Admin{}
	a.setupHTTPAuth(config.DefaultConfig(), registrar)

	if a.httpAuthSecret != "" {
		t.Fatalf("httpAuthSecret = %q, want empty", a.httpAuthSecret)
	}
	if registrar.middleware != nil {
		t.Fatal("middleware must not be installed without http_auth_secret")
	}
}

func TestAdminSecretUnaryInterceptorRequiresMatchingMetadata(t *testing.T) {
	a := &Admin{httpAuthSecret: "s3cret"}
	ic := a.adminSecretUnaryInterceptor()
	okHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "passed", nil
	}

	tests := []struct {
		name     string
		md       metadata.MD
		wantCode codes.Code
	}{
		{name: "matching secret is accepted", md: metadata.Pairs(AdminSecretMetadataKey, "s3cret"), wantCode: codes.OK},
		{name: "wrong secret is rejected", md: metadata.Pairs(AdminSecretMetadataKey, "wrong"), wantCode: codes.Unauthenticated},
		{name: "missing secret is rejected", md: nil, wantCode: codes.Unauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}
			resp, err := ic(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/admin.Admin/Test"}, okHandler)
			if got := status.Code(err); got != tt.wantCode {
				t.Fatalf("code = %v, want %v", got, tt.wantCode)
			}
			if tt.wantCode == codes.OK && resp != "passed" {
				t.Fatalf("resp = %v, want passed", resp)
			}
		})
	}
}

func TestSetupGRPCAuthInstallsInterceptorOnlyWithSecret(t *testing.T) {
	installAdminTestLogger(t)

	// 配置共享密钥时注册一个 unary 拦截器。
	reg := newAdminTestRegistrar()
	a := &Admin{}
	cfg := config.DefaultConfig()
	cfg.Plugins[Name] = &Config{HTTPAuthSecret: "s3cret"}
	a.setupHTTPAuth(cfg, reg)
	a.setupGRPCAuth(reg)
	if len(reg.grpcInterceptors) != 1 {
		t.Fatalf("grpcInterceptors = %d, want 1", len(reg.grpcInterceptors))
	}

	// 未配置共享密钥时保持现状，不注册任何拦截器。
	secretless := newAdminTestRegistrar()
	bare := &Admin{}
	bare.setupHTTPAuth(config.DefaultConfig(), secretless)
	bare.setupGRPCAuth(secretless)
	if len(secretless.grpcInterceptors) != 0 {
		t.Fatalf("grpcInterceptors = %d, want 0 without http_auth_secret", len(secretless.grpcInterceptors))
	}
}

func TestIsPubliclyReachableEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		want     bool
	}{
		{endpoint: "tcp://127.0.0.1:8083", want: false},
		{endpoint: "tcp://[::1]:8083", want: false},
		{endpoint: "unix://./gmqttd.sock", want: false},
		{endpoint: "tcp://0.0.0.0:8083", want: true},
		{endpoint: "tcp://:8083", want: true},
		{endpoint: "tcp://192.168.1.10:8083", want: true},
	}
	for _, tt := range tests {
		if got := isPubliclyReachableEndpoint(tt.endpoint); got != tt.want {
			t.Fatalf("isPubliclyReachableEndpoint(%q) = %v, want %v", tt.endpoint, got, tt.want)
		}
	}
}

func TestAdminConfigParsesHTTPAuthSecret(t *testing.T) {
	yamlDoc := `
api:
  grpc:
    - address: "tcp://127.0.0.1:8084"
  http:
    - address: "tcp://127.0.0.1:8083"
      map: "tcp://127.0.0.1:8084"
plugins:
  admin:
    http_auth_secret: top-secret
`
	parsed := config.DefaultConfig()
	if err := yaml.Unmarshal([]byte(yamlDoc), &parsed); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if err := parsed.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}

	pluginCfg, ok := parsed.Plugins[Name].(*Config)
	if !ok {
		t.Fatalf("plugin config type = %T, want *Config", parsed.Plugins[Name])
	}
	if pluginCfg.HTTPAuthSecret != "top-secret" {
		t.Fatalf("http_auth_secret = %q, want top-secret", pluginCfg.HTTPAuthSecret)
	}
	// 仅配置 http_auth_secret 时，监听地址保持默认值而不是空值。
	if pluginCfg.HTTP != DefaultConfig.HTTP {
		t.Fatalf("http config = %+v, want default %+v", pluginCfg.HTTP, DefaultConfig.HTTP)
	}
	if pluginCfg.GRPC != DefaultConfig.GRPC {
		t.Fatalf("grpc config = %+v, want default %+v", pluginCfg.GRPC, DefaultConfig.GRPC)
	}
}

func TestResolveHTTPAuthSecretPrefersConfigThenEnv(t *testing.T) {
	installAdminTestLogger(t)
	t.Setenv(EnvHTTPAuthSecret, "  env-secret  ")

	cfg := config.DefaultConfig()
	delete(cfg.Plugins, Name)

	// 无插件配置时回退环境变量（含 trim）。
	if got := resolveHTTPAuthSecret(cfg); got != "env-secret" {
		t.Fatalf("resolve without plugin config = %q, want env-secret", got)
	}

	// 插件配置存在时配置优先，环境变量被忽略。
	pluginCfg := DefaultConfig
	pluginCfg.HTTPAuthSecret = "yaml-secret"
	cfg.Plugins[Name] = &pluginCfg
	if got := resolveHTTPAuthSecret(cfg); got != "yaml-secret" {
		t.Fatalf("resolve with plugin config = %q, want yaml-secret", got)
	}

	// 配置为空白字符串时同样回退环境变量。
	pluginCfg.HTTPAuthSecret = "   "
	if got := resolveHTTPAuthSecret(cfg); got != "env-secret" {
		t.Fatalf("resolve with blank plugin secret = %q, want env-secret", got)
	}
}
