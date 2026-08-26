// 文件用途：提供登录/刷新令牌的 HttpOnly Cookie 双模式支持。
// 核心逻辑：登录与刷新成功后通过 Set-Cookie 下发会话 token（HttpOnly; SameSite=Lax），
// 刷新端点优先接受 cookie 中的 token，兼容仅带 x-token 头的存量客户端。
// 关键注意事项：cookie 仅收敛到认证相关 API 路径；开关由 GOTP_AUTH_COOKIE_ENABLED 控制（默认开启），
// Secure 由 GOTP_AUTH_COOKIE_SECURE 控制（默认关闭，保证本地 HTTP 联调可用）。
// 重构建议：旧前端缓存淘汰后，可将 x-token 头回退分支与响应体 token 一起评估下线。

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const (
	// AuthCookieName 认证 cookie 的名称。
	AuthCookieName = "aetherlink_auth_token"
	// AuthCookiePath cookie 作用路径，收敛到认证相关 API 前缀（登录、登出、刷新均在其下）。
	AuthCookiePath = "/api/v1"
	// AuthRefreshRoutePath 刷新端点的路由路径（Gin FullPath 形式），双模式 token 选择仅作用于该端点。
	AuthRefreshRoutePath = "/api/v1/user/refresh"

	authCookieEnabledKey = "auth.cookie.enabled"
	authCookieSecureKey  = "auth.cookie.secure"
)

// ensureAuthCookieDefaults 注册认证 cookie 配置默认值。
// 环境变量 GOTP_AUTH_COOKIE_ENABLED / GOTP_AUTH_COOKIE_SECURE 经 viper AutomaticEnv
// 映射后始终优先于这里的默认值；每次调用都重设以容忍其他测试中的 viper.Reset。
func ensureAuthCookieDefaults() {
	viper.SetDefault(authCookieEnabledKey, true)
	viper.SetDefault(authCookieSecureKey, false)
}

// AuthCookieEnabled 返回认证 cookie 是否启用；默认 true。
func AuthCookieEnabled() bool {
	ensureAuthCookieDefaults()
	return viper.GetBool(authCookieEnabledKey)
}

// AuthCookieSecure 返回认证 cookie 是否带 Secure 标志；默认 false 以兼容本地 HTTP 部署。
func AuthCookieSecure() bool {
	ensureAuthCookieDefaults()
	return viper.GetBool(authCookieSecureKey)
}

// SetAuthCookie 在登录/刷新成功后的响应上下文中下发认证 cookie。
// 未启用或 token 为空时静默跳过，保持纯 header 模式的既有行为。
func SetAuthCookie(c *gin.Context, token string, maxAgeSeconds int) {
	if !AuthCookieEnabled() || token == "" {
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AuthCookieName, token, maxAgeSeconds, AuthCookiePath, "", AuthCookieSecure(), true)
}

// ReadAuthCookie 返回认证 cookie 中的 token；未启用或缺失时返回空串。
func ReadAuthCookie(c *gin.Context) string {
	if !AuthCookieEnabled() {
		return ""
	}
	token, err := c.Cookie(AuthCookieName)
	if err != nil {
		return ""
	}
	return token
}

// authCookiePreferred 判断当前请求是否属于 cookie 优先的双模式路由（目前仅刷新端点）。
// 该路由为静态路径，因此同时兼容 Gin 路由匹配后的 FullPath 与未匹配场景下的原始 URL.Path。
// 只对刷新端点放开 cookie 来源，避免把 CSRF 暴露面扩散到全部业务接口。
func authCookiePreferred(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	return c.FullPath() == AuthRefreshRoutePath || c.Request.URL.Path == AuthRefreshRoutePath
}

// selectJWTAuthToken 决定本次请求使用的认证 token 来源：
// 刷新端点 cookie 优先，其余请求维持 x-token 头单一来源。
func selectJWTAuthToken(c *gin.Context, headerToken string) string {
	if authCookiePreferred(c) {
		if cookieToken := ReadAuthCookie(c); cookieToken != "" {
			return cookieToken
		}
	}
	return headerToken
}

// SelectJWTAuthToken 按与鉴权中间件完全一致的顺序提取原始 token（认证 cookie 优先，其次 x-token 头）。
// 供刷新端点计算旧会话摘要使用，避免 api 层自行拼装取值逻辑导致与中间件来源漂移。
func SelectJWTAuthToken(c *gin.Context) string {
	return selectJWTAuthToken(c, c.Request.Header.Get("x-token"))
}
