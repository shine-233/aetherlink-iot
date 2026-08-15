// 文件用途：为后端所有 HTTP 响应设置兼容 API、静态文件和错误响应的基础安全头。
// 核心逻辑：在 handler 写出状态码或正文前设置浏览器防 MIME 嗅探、点击劫持和来源泄露策略。
// 关键注意事项：HSTS 只能由确认启用 HTTPS 的边缘层设置；这里不假定本地或私有部署已启用 TLS。
package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders 返回不依赖外部代理的基础响应安全头中间件。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}
