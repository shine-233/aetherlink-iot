// 文件用途：注册属性数据相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type AttributeData struct{}

func (*AttributeData) InitAttributeData(Router *gin.RouterGroup) {
	attributedataapi := Router.Group("attribute/datas")
	{
		// 设备属性列表查询
		attributedataapi.GET(":id", api.Controllers.AttributeDataApi.HandleDataList)

		// 获取属性下发记录（分页）
		attributedataapi.GET("set/logs", api.Controllers.AttributeDataApi.HandleAttributeSetLogsDataListByPage)

		// 删除
		attributedataapi.DELETE(":id", api.Controllers.AttributeDataApi.DeleteData)

		// 下发属性
		attributedataapi.POST("pub", api.Controllers.AttributeDataApi.AttributePutMessage)

		// 向设备请求属性
		attributedataapi.GET("get", api.Controllers.AttributeDataApi.AttributeGetMessage)

		// 根据key查询设备属性
		attributedataapi.GET("key", api.Controllers.AttributeDataApi.HandleAttributeDataByKey)
	}
}
