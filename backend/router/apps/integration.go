// 文件用途：统一集成实体（Integration，TB-45）路由定义。
// 核心逻辑：挂载 /api/v1/integrations 路由组并关联 IntegrationApi（Casbin 登记见 130.sql）。
// 关键注意事项：组内路径变更必须同步 130.sql 之后的迁移 Casbin 登记与
//
//	router/apps/casbin_route_registration_contract_test.go 的挂载清单，否则启动审计 fail-fast。
//
// 重构建议：下行转换器执行端点（明确不做项）落地时新增独立 POST 子路由，不在本组混挂 PUT 语义。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type IntegrationRouter struct{}

func (*IntegrationRouter) InitIntegration(Router *gin.RouterGroup) {
	r := Router.Group("integrations")
	{
		r.POST("", api.Controllers.IntegrationApi.CreateIntegration)
		r.PUT("", api.Controllers.IntegrationApi.UpdateIntegration)
		r.GET("", api.Controllers.IntegrationApi.ListIntegrations)
		r.GET(":id", api.Controllers.IntegrationApi.GetIntegrationByID)
		r.DELETE(":id", api.Controllers.IntegrationApi.DeleteIntegration)
	}
}
