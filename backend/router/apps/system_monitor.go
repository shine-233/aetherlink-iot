// 文件用途：注册系统监控相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"
	"aetherlink-iot/backend/pkg/metrics"

	"github.com/gin-gonic/gin"
)

// SystemMonitor 系统监控模块
type SystemMonitor struct{}

// InitSystemMonitor 初始化系统监控相关路由
func (m *SystemMonitor) InitSystemMonitor(r *gin.RouterGroup, metricsManager *metrics.Metrics) {
	// 注册路由
	r.GET("system/metrics/current", api.Controllers.SystemMonitorApi.GetCurrentSystemMetrics)
	r.GET("system/metrics/history", api.Controllers.SystemMonitorApi.GetHistorySystemMetrics)
}
