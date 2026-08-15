// 文件用途：注册OpenAPI Key相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type OpenAPIKey struct{}

func (*OpenAPIKey) InitOpenAPIKey(Router *gin.RouterGroup) {
	openAPIRouter := Router.Group("open/keys")
	{
		// OpenAPI密钥管理
		openAPIRouter.POST("", api.Controllers.OpenAPIKeyApi.CreateOpenAPIKey)      // 创建密钥
		openAPIRouter.GET("", api.Controllers.OpenAPIKeyApi.GetOpenAPIKeyList)      // 获取列表
		openAPIRouter.PUT("", api.Controllers.OpenAPIKeyApi.UpdateOpenAPIKey)       // 更新密钥
		openAPIRouter.DELETE(":id", api.Controllers.OpenAPIKeyApi.DeleteOpenAPIKey) // 删除密钥
	}
}
