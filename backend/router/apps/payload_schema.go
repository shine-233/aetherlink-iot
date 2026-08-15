// 文件用途：注册 payload schema registry 相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：当前只暴露无副作用的静态校验接口；broker 侧真实拦截属于外部 MQTT 契约变更，不在此注册。
// 重构建议：若后续引入持久化 registry CRUD，可在此分组继续挂载，并复用同一校验引擎。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type PayloadSchema struct{}

func (*PayloadSchema) InitPayloadSchema(Router *gin.RouterGroup) {
	url := Router.Group("payload-schema")
	{
		// 静态校验：比对样本 payload 与声明的字段约束，不落库、不连 broker、不下发消息。
		url.POST("validate", api.Controllers.PayloadSchemaApi.ValidatePayload)

		// 持久化 registry CRUD：只保存"声明的约束"，broker 侧真实拦截仍需运行时验证。
		url.POST("", api.Controllers.PayloadSchemaApi.SavePayloadSchema)
		url.GET("", api.Controllers.PayloadSchemaApi.ListPayloadSchemas)
		url.PUT(":schema_id", api.Controllers.PayloadSchemaApi.UpdatePayloadSchema)
		url.DELETE(":schema_id", api.Controllers.PayloadSchemaApi.DeletePayloadSchema)
	}
}
