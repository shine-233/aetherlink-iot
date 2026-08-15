// 文件用途：注册场景相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Scene struct{}

func (*Scene) Init(Router *gin.RouterGroup) {
	url := Router.Group("scene")
	{
		// 新增
		url.POST("", api.Controllers.SceneApi.CreateScene)

		// dry-run
		url.POST("dry-run", api.Controllers.SceneApi.DryRunScene)

		// 删除
		url.DELETE(":id", api.Controllers.SceneApi.DeleteScene)

		// list
		url.GET("", api.Controllers.SceneApi.HandleSceneByPage)

		// detail
		url.GET("/detail/:id", api.Controllers.SceneApi.HandleScene)

		// 更新
		url.PUT("", api.Controllers.SceneApi.UpdateScene)

		// 激活
		url.POST("active/:id", api.Controllers.SceneApi.ActiveScene)

		// 场景日志查询
		url.GET("log", api.Controllers.SceneApi.HandleSceneLog)

	}
}
