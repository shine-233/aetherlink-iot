// 文件用途：注册系统 UI 元素相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type UiElements struct {
}

func (*UiElements) Init(Router *gin.RouterGroup) {
	url := Router.Group("ui_elements")
	{
		// 增
		url.POST("", api.Controllers.UiElementsApi.CreateUiElements)

		// 删
		url.DELETE(":id", api.Controllers.UiElementsApi.DeleteUiElements)

		// 改
		url.PUT("", api.Controllers.UiElementsApi.UpdateUiElements)

		// 分页查询,按照树状结构返回，父节点包含一个"children"，其中是子节点，按照order排序
		url.GET("", api.Controllers.UiElementsApi.ServeUiElementsListByPage)

		// 根据用户权限查询
		url.GET("menu", api.Controllers.UiElementsApi.ServeUiElementsListByAuthority)

		// 菜单配置表单
		url.GET("select/form", api.Controllers.UiElementsApi.ServeUiElementsListByTenant)
	}
}
