// 文件用途：注册数据转发规则相关应用路由。
// 核心逻辑：Gin 路由组 /forward_rules 上挂载 CRUD、启停与分页查询 Handler。
// 关键注意事项：位于 JWT+Casbin 保护块内；出参敏感字段已在 service 层脱敏。

package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type ForwardRule struct{}

func (*ForwardRule) Init(Router *gin.RouterGroup) {
	url := Router.Group("forward_rules")
	{
		url.GET("", api.Controllers.ForwardRuleApi.HandleListByPage)
		url.POST("", api.Controllers.ForwardRuleApi.HandleCreate)
		url.PUT("/:id", api.Controllers.ForwardRuleApi.HandleUpdate)
		url.DELETE("/:id", api.Controllers.ForwardRuleApi.HandleDelete)
		url.PUT("/:id/toggle", api.Controllers.ForwardRuleApi.HandleToggle)
		url.GET("/:id", api.Controllers.ForwardRuleApi.HandleGetByID)
	}
}
