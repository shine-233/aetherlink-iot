// 文件用途：ThingsBoard 核心数据转换器（Data Converters / 载荷解析）路由定义。
// 核心逻辑：挂载 /api/v1/converters 与 /api/v1/data-converters 路由组并关联 DataConverterApi。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type DataConverterRouter struct{}

func (*DataConverterRouter) InitDataConverter(Router *gin.RouterGroup) {
	r := Router.Group("converters")
	{
		r.POST("", api.Controllers.DataConverterApi.CreateDataConverter)
		r.PUT("", api.Controllers.DataConverterApi.UpdateDataConverter)
		r.GET("", api.Controllers.DataConverterApi.ListDataConverters)
		r.GET(":id", api.Controllers.DataConverterApi.GetDataConverterByID)
		r.DELETE(":id", api.Controllers.DataConverterApi.DeleteDataConverter)
		r.POST("test", api.Controllers.DataConverterApi.TestDataConverter)
	}

	rAlias := Router.Group("data-converters")
	{
		rAlias.POST("", api.Controllers.DataConverterApi.CreateDataConverter)
		rAlias.PUT("", api.Controllers.DataConverterApi.UpdateDataConverter)
		rAlias.GET("", api.Controllers.DataConverterApi.ListDataConverters)
		rAlias.GET(":id", api.Controllers.DataConverterApi.GetDataConverterByID)
		rAlias.DELETE(":id", api.Controllers.DataConverterApi.DeleteDataConverter)
		rAlias.POST("test", api.Controllers.DataConverterApi.TestDataConverter)
	}
}
