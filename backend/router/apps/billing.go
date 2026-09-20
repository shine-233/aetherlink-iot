// 文件用途：注册商业化计费与用量计量应用路由（P3 商业化）。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Billing struct{}

func (*Billing) InitBilling(Router *gin.RouterGroup) {
	billing := Router.Group("billing")
	{
		billing.GET("plans", api.Controllers.BillingApi.ListPlans)
		billing.GET("usage", api.Controllers.BillingApi.GetUsage)
		billing.POST("subscriptions", api.Controllers.BillingApi.SubscribePlan)
	}
}
