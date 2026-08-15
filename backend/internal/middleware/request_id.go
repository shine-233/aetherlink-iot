// 文件用途：为每个 HTTP 请求提供可安全回传和记录的关联标识。
// 核心逻辑：复用格式受限的 X-Request-ID；否则在本地生成新标识，并写入 Gin 上下文和响应头。
// 关键注意事项：请求头来自不可信边界，必须限制字符和长度，避免日志注入及无界字段。
package middleware

import (
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/go-basic/uuid"
)

const (
	requestIDContextKey = "X-Request-ID"
	requestIDHeader     = "X-Request-ID"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// RequestID preserves a safe caller-provided correlation ID or generates one.
// It has no external collector dependency and keeps the existing response body contract unchanged.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = uuid.New()
		}

		c.Set(requestIDContextKey, requestID)
		c.Header(requestIDHeader, requestID)
		c.Next()
	}
}
