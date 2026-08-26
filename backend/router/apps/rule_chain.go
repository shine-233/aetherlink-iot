// 文件用途：注册规则链相关路由（ROADMAP B2）。
// 核心逻辑：租户内 CRUD；鉴权由 v1 组中间件保证。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type RuleChain struct{}

func (*RuleChain) InitRuleChain(Router *gin.RouterGroup) {
	ruleChains := Router.Group("rule-chains")
	{
		ruleChainApi := api.Controllers.RuleChainApi
		ruleChains.POST("", ruleChainApi.HandleCreateRuleChain)
		ruleChains.PUT("", ruleChainApi.HandleUpdateRuleChain)
		ruleChains.GET("list", ruleChainApi.HandleListRuleChains)
		ruleChains.GET(":id", ruleChainApi.HandleGetRuleChain)
		ruleChains.DELETE(":id", ruleChainApi.HandleDeleteRuleChain)
	}
}
