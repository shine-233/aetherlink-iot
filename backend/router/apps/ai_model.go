package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// AiModel AI 2.0 模型中心 + 助手路由组（ROADMAP D7）。
type AiModel struct{}

func (*AiModel) InitAiModel(Router *gin.RouterGroup) {
	ai := Router.Group("ai")
	{
		models := ai.Group("models")
		{
			models.POST("", api.Controllers.AiModelApi.CreateModel)
			models.PUT("", api.Controllers.AiModelApi.UpdateModel)
			models.DELETE(":id", api.Controllers.AiModelApi.DeleteModel)
			models.GET("", api.Controllers.AiModelApi.ListModels)
			models.GET(":id", api.Controllers.AiModelApi.GetModel)
		}
		ai.POST("assistant/chat", api.Controllers.AiModelApi.Chat)
	}
}
