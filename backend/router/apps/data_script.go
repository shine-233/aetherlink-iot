// 文件用途：注册数据脚本相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type DataScript struct {
}

func (*DataScript) Init(Router *gin.RouterGroup) {
	url := Router.Group("data_script")
	{
		// 增
		url.POST("", api.Controllers.DataScriptApi.CreateDataScript)

		// 删
		url.DELETE(":id", api.Controllers.DataScriptApi.DeleteDataScript)

		// 改
		url.PUT("", api.Controllers.DataScriptApi.UpdateDataScript)

		// 查
		url.GET("", api.Controllers.DataScriptApi.HandleDataScriptListByPage)

		// 调试
		url.POST("quiz", api.Controllers.DataScriptApi.QuizDataScript)

		// 启用禁用
		url.PUT("enable", api.Controllers.DataScriptApi.EnableDataScript)
	}
}
