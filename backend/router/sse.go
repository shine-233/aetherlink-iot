// 文件用途：注册后端系统事件 SSE 路由。
// 核心逻辑：在已鉴权的 `events` 分组下挂载 `HandleSystemEvents`。
// 关键注意事项：该路由依赖上游 JWT/Casbin 中间件顺序，移动注册位置会改变访问权限。
// 重构建议：后续可与其他实时通信路由统一整理，并补充路由契约测试。
package router

import (
	sseapi "aetherlink-iot/backend/internal/api/sseapi"

	"github.com/gin-gonic/gin"
)

func SSERouter(Router *gin.RouterGroup) {
	var sseApi sseapi.SSEApi
	sa := Router.Group("events")
	{
		sa.GET("", sseApi.HandleSystemEvents)

	}
}
