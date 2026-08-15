// 文件用途：提供 HTTP 请求链路中的 cors 中间件能力。
// 核心逻辑：在 Gin 请求处理前后执行认证、鉴权、跨域、指标、响应包装或操作日志处理，主要围绕 var defaultCorsHeaders、var defaultCorsMethods、func Cors、func configuredStringList 等声明展开。
// 关键注意事项：中间件位于安全与兼容边界，修改需保持状态码、上下文键和响应格式稳定。
// 重构建议：后续可将外部依赖抽成接口，便于独立测试和不同部署模式复用。

package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var defaultCorsHeaders = []string{
	"Content-Type",
	"Authorization",
	"X-Token",
	"X-API-Key",
	"Accept-Language",
}

var defaultCorsMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodDelete,
	http.MethodOptions,
}

// Cors 按 allowlist 放行跨域请求；生产环境未配置时不默认开放跨域。
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := strings.TrimSpace(c.Request.Header.Get("Origin"))

		if origin != "" && isAllowedOrigin(origin, configuredStringList("cors.allowed_origins")) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			if viper.GetBool("cors.allow_credentials") {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Header("Access-Control-Allow-Headers", strings.Join(configuredStringListWithDefault("cors.allowed_headers", defaultCorsHeaders), ", "))
		c.Header("Access-Control-Allow-Methods", strings.Join(configuredStringListWithDefault("cors.allowed_methods", defaultCorsMethods), ", "))
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type, New-Token, New-Expires-At")

		// 放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		// 处理请求
		c.Next()
	}
}

func configuredStringList(key string) []string {
	rawList := viper.GetStringSlice(key)
	if len(rawList) == 0 {
		raw := strings.TrimSpace(viper.GetString(key))
		if raw != "" {
			rawList = strings.Split(raw, ",")
		}
	}

	values := make([]string, 0, len(rawList))
	for _, item := range rawList {
		value := strings.TrimSpace(item)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func configuredStringListWithDefault(key string, defaults []string) []string {
	values := configuredStringList(key)
	if len(values) > 0 {
		return values
	}
	return defaults
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}
