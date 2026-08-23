// 文件用途：为公开的协议插件接入端点（/api/v1/plugin/*）提供边界认证。
// 核心逻辑：配置了 plugin.service.key 时对全部来源严格校验 X-Plugin-Key（常量时间比较）；
// 未配置时仅放行回环与私网来源，公网来源一律 401，堵住匿名远程调用面。
// 关键注意事项：响应沿用 ErrorResponse 结构保持契约稳定；环境变量覆盖键为
// GOTP_PLUGIN_SERVICE_KEY。外部插件升级路径：部署侧生成共享密钥注入后端，
// 插件请求附带 X-Plugin-Key 头。
package middleware

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// pluginKeyHeader 是插件接入共享密钥的请求头名称。
const pluginKeyHeader = "X-Plugin-Key"

// isTrustedPluginSource 判定未配置共享密钥时是否允许该 TCP 来源访问。
// 回环 = 本机插件与本地开发链路；私网（RFC1918/ULA）= 同内网部署的插件容器；
// 公网来源在未配置密钥时始终拒绝，避免匿名远程调用。
// 判定只基于 RemoteAddr，不读取代理头，防止伪造绕过。
func isTrustedPluginSource(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

// PluginAuth 返回协议插件接入端点的边界认证中间件。
func PluginAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := viper.GetString("plugin.service.key")
		if key != "" {
			provided := c.GetHeader(pluginKeyHeader)
			if subtle.ConstantTimeCompare([]byte(provided), []byte(key)) != 1 {
				rejectPluginAccess(c)
				return
			}
			c.Next()
			return
		}
		if !isTrustedPluginSource(c.Request.RemoteAddr) {
			rejectPluginAccess(c)
			return
		}
		c.Next()
	}
}

// rejectPluginAccess 以 401 拒绝未通过边界认证的插件请求。
func rejectPluginAccess(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, ErrorResponse{
		Code:      ErrCodeInvalidToken,
		Message:   "plugin access denied",
		RequestID: c.GetString("X-Request-ID"),
	})
	c.Abort()
}
