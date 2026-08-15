// 文件用途：注册系统字典相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Dict struct {
}

func (*Dict) InitDict(Router *gin.RouterGroup) {
	dictapi := Router.Group("dict")
	{
		// 新增字典列
		dictapi.POST("column", api.Controllers.DictApi.CreateDictColumn)

		// 新增字典多语言
		dictapi.POST("language", api.Controllers.CreateDictLanguage)

		// 枚举查询接口
		dictapi.GET("enum", api.Controllers.DictApi.HandleDict)

		// 字典列表分页查询
		dictapi.GET("", api.Controllers.DictApi.HandleDictLisyByPage)

		// 字典多语言列表查询
		dictapi.GET("language/:id", api.Controllers.HandleDictLanguage)

		// 删除字典
		dictapi.DELETE("column/:id", api.Controllers.DictApi.DeleteDictColumn)

		// 删除字典多语言
		dictapi.DELETE("language/:id", api.Controllers.DictApi.DeleteDictLanguage)

		// 获取协议服务下拉菜单
		dictapi.GET("protocol/service", api.Controllers.DictApi.HandleProtocolAndService)
	}
}
