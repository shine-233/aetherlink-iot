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
		// PHASE-D-D1 BEGIN 节点级调试 trace 查询
		ruleChains.GET(":id/nodes/:nodeId/traces", ruleChainApi.HandleGetRuleChainNodeTraces)
		// PHASE-D-D1 END
		// P1.2 BEGIN 草稿/发布版本与回滚（对应迁移 93.sql 的 Casbin 登记）
		ruleChains.GET(":id/versions", ruleChainApi.HandleListRuleChainVersions)
		ruleChains.POST(":id/versions", ruleChainApi.HandleCreateRuleChainDraftVersion)
		ruleChains.POST("versions/publish", ruleChainApi.HandlePublishRuleChainVersion)
		ruleChains.POST("versions/rollback", ruleChainApi.HandleRollbackRuleChainVersion)
		// P1.2 END
	}
}
