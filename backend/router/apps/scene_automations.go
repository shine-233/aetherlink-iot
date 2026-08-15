// 文件用途：注册场景自动化相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type SceneAutomations struct{}

func (*SceneAutomations) Init(Router *gin.RouterGroup) {
	url := Router.Group("scene_automations")
	{
		// 新
		url.POST("", api.Controllers.SceneAutomationsApi.CreateSceneAutomations)

		// 删
		url.DELETE(":id", api.Controllers.SceneAutomationsApi.DeleteSceneAutomations)

		// 改
		url.PUT("", api.Controllers.SceneAutomationsApi.UpdateSceneAutomations)

		// 启/停
		url.POST("switch/:id", api.Controllers.SceneAutomationsApi.SwitchSceneAutomations)

		url.POST("dry-run", api.Controllers.SceneAutomationsApi.DryRunSceneAutomations)

		// 查列表
		url.GET("list", api.Controllers.SceneAutomationsApi.HandleSceneAutomationsByPage)

		// 查详情
		url.GET("detail/:id", api.Controllers.SceneAutomationsApi.HandleSceneAutomations)

		// 查日志
		url.GET("log", api.Controllers.SceneAutomationsApi.HandleSceneAutomationsLog)

		// 查列表 根据设备id 查询包含告警的场景联动
		url.GET("alarm", api.Controllers.SceneAutomationsApi.HandleSceneAutomationsWithAlarmByPage)

	}
}
