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
		// 创建产品（支持 TB-15 conflict_policy）
		productapi.POST("", api.Controllers.ProductApi.HandleCreateProduct)

		// 修改产品
		productapi.PUT("", api.Controllers.ProductApi.HandleUpdateProduct)

		// 删除产品
		productapi.DELETE(":id", api.Controllers.ProductApi.HandleDeleteProduct)

		// 查询产品详情
		productapi.GET(":id", api.Controllers.ProductApi.HandleGetProductByID)

		// 产品列表与选择列表（预注册建档等下拉数据源）
		productapi.GET("", api.Controllers.ProductApi.HandleProductSelectListByPage)
	}
}
