// 文件用途：注册产品选择相关的应用路由。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Product struct{}

func (*Product) Init(Router *gin.RouterGroup) {
	productapi := Router.Group("product")
	{
		// 产品选择列表（预注册建档等下拉数据源）
		productapi.GET("", api.Controllers.ProductApi.HandleProductSelectListByPage)
	}
}
