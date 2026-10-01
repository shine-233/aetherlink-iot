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
		// TB-17：今日 API 配额（调用数/限额/剩余），前端配额页数据源。
		billing.GET("api-quota", api.Controllers.BillingApi.GetAPIQuota)
		billing.POST("subscriptions", api.Controllers.BillingApi.SubscribePlan)
	}
}
