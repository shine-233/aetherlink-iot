// 文件用途：通用 Secrets Storage（ROADMAP TB-18）路由定义。
// 核心逻辑：在 Gin 路由组上挂载 /api/v1/secrets 相关端点并关联 SecretApi。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type SecretsRouter struct{}

func (*SecretsRouter) InitSecrets(Router *gin.RouterGroup) {
	r := Router.Group("secrets")
	{
		r.POST("", api.Controllers.SecretApi.CreateSecret)
		r.GET("", api.Controllers.SecretApi.GetSecretList)
		r.GET(":id", api.Controllers.SecretApi.GetSecretDetail)
		r.PUT(":id", api.Controllers.SecretApi.UpdateSecret)
		r.DELETE(":id", api.Controllers.SecretApi.DeleteSecret)
		r.POST(":id/reveal", api.Controllers.SecretApi.RevealSecret)
		r.POST(":id/reseal", api.Controllers.SecretApi.ResealSecret)
	}
}
