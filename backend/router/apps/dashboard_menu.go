// 文件用途：注册仪表盘菜单相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type DashboardMenu struct{}

func (*DashboardMenu) Init(Router *gin.RouterGroup) {
	url := Router.Group("dashboard-menu")
	{
		url.POST("batch", api.Controllers.DashboardMenuApi.BatchGetDashboardMenus)
		url.GET(":dashboardId", api.Controllers.DashboardMenuApi.GetDashboardMenu)
		url.PUT(":dashboardId", api.Controllers.DashboardMenuApi.SaveDashboardMenu)
		url.DELETE(":dashboardId", api.Controllers.DashboardMenuApi.DeleteDashboardMenu)
	}
}
