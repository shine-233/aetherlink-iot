// 文件用途：注册Logo 配置相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Logo struct {
}

func (*Logo) Init(Router *gin.RouterGroup) {
	url := Router.Group("logo")
	{
		// 改
		url.PUT("", api.Controllers.LogoApi.UpdateLogo)

		// 查 已移动不用验证token
		// url.GET("", api.Controllers.LogoApi.GetLogoList)
	}
}
