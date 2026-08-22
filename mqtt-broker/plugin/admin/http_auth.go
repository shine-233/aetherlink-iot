// 文件用途：为 admin 管理面 HTTP/gRPC API 提供可选的应用层共享密钥认证。
// 核心逻辑：http_auth_secret 非空时安装根级 HTTP 中间件并注册 gRPC unary 拦截器，
// 要求 X-Admin-Secret 头 / x-admin-secret metadata 恒定时间匹配
// （HTTP 侧还接受内置管理页会话 cookie）；为空时保持既有网络边界行为，
// 但绑定非回环地址时打印启动告警。
// 安全职责：admin API 可列出客户端、断开会话并代发任意 MQTT 消息，
// 必须保证默认部署下不依赖单一网络边界即可拒绝未授权管理请求。

package admin

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AdminSecretHeader 是共享密钥认证使用的 HTTP 请求头。
const AdminSecretHeader = "X-Admin-Secret"

// AdminSecretMetadataKey 是共享密钥认证使用的 gRPC metadata 键。
// gRPC metadata 键统一小写。
const AdminSecretMetadataKey = "x-admin-secret"

// EnvHTTPAuthSecret 是 http_auth_secret 的环境变量注入入口，
// 与 GMQTT_ADMIN_USERNAME / GMQTT_ADMIN_PASSWORD 保持同一命名约定，
// 供 Compose 等只读挂载配置的部署方式使用。
const EnvHTTPAuthSecret = "GMQTT_ADMIN_HTTP_AUTH_SECRET"

// resolveHTTPAuthSecret 解析生效的共享密钥：插件配置优先，环境变量回退。
func resolveHTTPAuthSecret(cfg config.Config) string {
	secret := ""
	if pluginCfg, ok := cfg.Plugins[Name].(*Config); ok {
		secret = strings.TrimSpace(pluginCfg.HTTPAuthSecret)
	}
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv(EnvHTTPAuthSecret))
	}
	return secret
}

// setupHTTPAuth 读取 http_auth_secret 配置并按需安装认证中间件。
func (a *Admin) setupHTTPAuth(cfg config.Config, registrar server.APIRegistrar) {
	secret := resolveHTTPAuthSecret(cfg)
	a.httpAuthSecret = secret

	setter, canSet := registrar.(server.HTTPMiddlewareSetter)
	if !canSet {
		if secret != "" {
			log.Error("admin http_auth_secret is configured but the API registrar does not support middleware; admin HTTP APIs stay unauthenticated")
		}
		warnIfAPIBoundNonLoopbackWithoutSecret(secret, cfg)
		return
	}

	if secret == "" {
		warnIfAPIBoundNonLoopbackWithoutSecret("", cfg)
		return
	}

	setter.SetHTTPMiddleware(a.adminSecretMiddleware())
	log.Info("admin http shared-secret authentication enabled", zap.String("header", AdminSecretHeader))
}

// adminSecretMiddleware 返回校验 X-Admin-Secret 的根级中间件。
// 共享密钥使用 crypto/subtle 恒定时间比较；内置管理页登录后下发的会话
// cookie 同样被接受，保证启用共享密钥后 dashboard 数据接口仍可正常工作。
func (a *Admin) adminSecretMiddleware() server.HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !a.adminRequestAuthorized(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// setupGRPCAuth 在 http_auth_secret 非空时为 API gRPC server 注册共享密钥拦截器；
// 为空或 registrar 不支持该能力时保持既有行为。依赖 setupHTTPAuth 已解析出密钥。
func (a *Admin) setupGRPCAuth(registrar server.APIRegistrar) {
	setter, canSet := registrar.(server.GRPCUnaryInterceptorSetter)
	if !canSet {
		if a.httpAuthSecret != "" {
			log.Error("admin http_auth_secret is configured but the API registrar does not support gRPC unary interceptors; admin gRPC APIs stay unauthenticated")
		}
		return
	}
	if a.httpAuthSecret == "" {
		return
	}
	setter.AddGRPCUnaryInterceptor(a.adminSecretUnaryInterceptor())
	log.Info("admin gRPC shared-secret authentication enabled", zap.String("metadata", AdminSecretMetadataKey))
}

// adminSecretUnaryInterceptor 返回校验 x-admin-secret metadata 的 unary 拦截器，
// 共享密钥使用 crypto/subtle 恒定时间比较，不匹配返回 Unauthenticated。
func (a *Admin) adminSecretUnaryInterceptor() server.GRPCUnaryInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !a.grpcRequestAuthorized(ctx) {
			return nil, status.Error(codes.Unauthenticated, "unauthorized")
		}
		return handler(ctx, req)
	}
}

// grpcRequestAuthorized 报告 gRPC 请求是否携带匹配的 x-admin-secret metadata。
func (a *Admin) grpcRequestAuthorized(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	values := md.Get(AdminSecretMetadataKey)
	if len(values) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(values[0]), []byte(a.httpAuthSecret)) == 1
}

// adminRequestAuthorized 报告请求是否通过管理面应用层鉴权。
func (a *Admin) adminRequestAuthorized(r *http.Request) bool {
	if a.httpAuthSecret != "" &&
		subtle.ConstantTimeCompare([]byte(r.Header.Get(AdminSecretHeader)), []byte(a.httpAuthSecret)) == 1 {
		return true
	}
	return isAuthenticated(r)
}

// warnIfAPIBoundNonLoopbackWithoutSecret 在未配置共享密钥且管理面监听地址
// 可能暴露到非回环网络时打印告警。unix socket 与显式回环地址视为安全。
func warnIfAPIBoundNonLoopbackWithoutSecret(secret string, cfg config.Config) {
	if secret != "" {
		return
	}
	for _, endpoint := range cfg.API.HTTP {
		if isPubliclyReachableEndpoint(endpoint.Address) {
			log.Warn("admin HTTP API is bound to a non-loopback address without http_auth_secret; " +
				"set plugins.admin.http_auth_secret or GMQTT_ADMIN_HTTP_AUTH_SECRET or restrict the api.http listener")
		}
	}
	for _, endpoint := range cfg.API.GRPC {
		if isPubliclyReachableEndpoint(endpoint.Address) {
			log.Warn("admin gRPC API is bound to a non-loopback address; " +
				"restrict the api.grpc listener to a trusted network or unix socket")
		}
	}
}

// isPubliclyReachableEndpoint 判断 endpoint 是否可能暴露在非回环网络上。
// 空主机名表示通配地址（监听全部网卡），无法解析为 IP 的主机名按可能暴露处理。
func isPubliclyReachableEndpoint(endpoint string) bool {
	schema, addr := splitAPIEndpoint(endpoint)
	if schema == "unix" {
		return false
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

// splitAPIEndpoint 解析 "tcp://host:port"、"unix:///path" 形式的 endpoint。
func splitAPIEndpoint(endpoint string) (schema string, addr string) {
	parts := strings.SplitN(endpoint, "://", 2)
	if len(parts) == 1 && parts[0] != "" {
		return "tcp", parts[0]
	}
	return parts[0], parts[1]
}
