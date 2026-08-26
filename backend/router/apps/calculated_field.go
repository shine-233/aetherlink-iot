// 文件用途：注册计算字段相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载列表、创建、更新、删除、启停开关与详情端点。
// 关键注意事项：路由位于 JWT+Casbin 中间件之后的 v1 组内；路径是前后端与自动化契约，
// 新增或调整必须同步 endpoint catalog 与 business capabilities 清单。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// CalculatedField 计算字段路由组。
type CalculatedField struct{}

// InitCalculatedField 挂载 /calculated_fields 路由。
func (*CalculatedField) InitCalculatedField(Router *gin.RouterGroup) {
	url := Router.Group("calculated_fields")
	{
		url.GET("", api.Controllers.CalculatedFieldApi.HandleGetCalculatedFieldList)
		url.POST("", api.Controllers.CalculatedFieldApi.HandleCreateCalculatedField)
		url.PUT(":id", api.Controllers.CalculatedFieldApi.HandleUpdateCalculatedField)
		url.DELETE(":id", api.Controllers.CalculatedFieldApi.HandleDeleteCalculatedField)
		url.PUT(":id/toggle", api.Controllers.CalculatedFieldApi.HandleToggleCalculatedField)
		url.GET(":id", api.Controllers.CalculatedFieldApi.HandleGetCalculatedField)
	}
}
