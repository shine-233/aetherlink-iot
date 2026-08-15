// 文件用途：注册告警相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Alarm struct{}

func (*Alarm) Init(Router *gin.RouterGroup) {
	url := Router.Group("alarm")
	alarmconfig(url)
	alarminfo(url)

	// 设备统计相关接口
	url.GET("device/counts", api.Controllers.AlarmApi.GetAlarmDeviceCountsByTenant)
}

func alarmconfig(Router *gin.RouterGroup) {
	url := Router.Group("config")
	{
		// 增
		url.POST("", api.Controllers.AlarmApi.CreateAlarmConfig)

		// 删
		url.DELETE(":id", api.Controllers.AlarmApi.DeleteAlarmConfig)

		// 改
		url.PUT("", api.Controllers.AlarmApi.UpdateAlarmConfig)

		// 查
		url.GET("", api.Controllers.AlarmApi.ServeAlarmConfigListByPage)
	}
}

func alarminfo(Router *gin.RouterGroup) {
	url := Router.Group("info")
	{
		// 改
		url.PUT("", api.Controllers.AlarmApi.UpdateAlarmInfo)

		// 批量改
		url.PUT("batch", api.Controllers.AlarmApi.BatchUpdateAlarmInfo)

		// 查
		url.GET("", api.Controllers.AlarmApi.HandleAlarmInfoListByPage)

		url.GET("history", api.Controllers.AlarmApi.HandleAlarmHisttoryListByPage)
		url.GET("history/monthly", api.Controllers.AlarmApi.HandleAlarmHistoryMonthlyTrend)

		url.PUT("history", api.Controllers.AlarmApi.AlarmHistoryDescUpdate)

		url.GET("history/device", api.Controllers.AlarmApi.HandleDeviceAlarmStatus)

		url.GET("config/device", api.Controllers.AlarmApi.HandleConfigByDevice)

		url.PUT("history/batch-action", api.Controllers.AlarmApi.BatchAlarmHistoryAction)

		url.PUT("history/:id/acknowledge", api.Controllers.AlarmApi.AcknowledgeAlarmHistory)

		url.PUT("history/:id/reset", api.Controllers.AlarmApi.ResetAlarmHistory)

		url.GET("history/:id", api.Controllers.AlarmApi.HandleAlarmInfoHistory)

		// 兼容旧客户端；service 会鉴权后按审计留存策略拒绝物理删除。
		url.DELETE("history/:id", api.Controllers.AlarmApi.DeleteAlarmHistory)
	}
}
