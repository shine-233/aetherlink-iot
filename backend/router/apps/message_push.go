// 文件用途：注册消息推送相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"
	"github.com/gin-gonic/gin"
)

type MessagePush struct {
}

func (*MessagePush) Init(Router *gin.RouterGroup) {
	url := Router.Group("message_push")
	{
		// 增
		url.POST("", api.Controllers.MessagePushApi.CreateMessagePush)
		//注销
		url.POST("/logout", api.Controllers.MessagePushApi.MessagePushMangeLogout)
		//获取配置
		url.GET("/config", api.Controllers.MessagePushApi.GetMessagePushConfig)
		//设置配置
		url.POST("/config", api.Controllers.MessagePushApi.SetMessagePushConfig)
	}
}
