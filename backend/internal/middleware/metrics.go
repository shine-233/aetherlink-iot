// 文件用途：提供 HTTP 请求链路中的 metrics 中间件能力。
// 核心逻辑：在 Gin 请求处理前后执行认证、鉴权、跨域、指标、响应包装或操作日志处理，主要围绕 func MetricsMiddleware 等声明展开。
// 关键注意事项：中间件位于安全与兼容边界，修改需保持状态码、上下文键和响应格式稳定。
// 重构建议：后续可将外部依赖抽成接口，便于独立测试和不同部署模式复用。

package middleware

import (
	"aetherlink-iot/backend/pkg/metrics"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// MetricsMiddleware 创建监控中间件
func MetricsMiddleware(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath() // 获取路由路径而不是实际URL

		// 记录请求
		m.RecordAPIRequest(path, c.Request.Method)

		// 使用 defer 确保在请求结束时记录指标
		defer func() {
			// 记录响应时间
			duration := time.Since(start).Seconds()
			m.RecordAPILatency(path, duration)

			// 记录响应大小
			m.RecordResponseSize(path, float64(c.Writer.Size()))

			// 处理 panic：记录指标并恢复，避免服务崩溃
			// 不再重新抛出 panic，由上层 Recovery 中间件统一处理
			if err := recover(); err != nil {
				m.RecordAPIError("panic")
				m.RecordCriticalError()
				logrus.Errorf("请求处理发生 panic: %v, path: %s", err, path)
			}
		}()

		c.Next()

		// 记录错误
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				if e.IsType(gin.ErrorTypePrivate) {
					m.RecordAPIError("system")
				} else {
					m.RecordAPIError("business")
				}
			}
		}
	}
}
