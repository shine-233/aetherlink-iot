// 文件用途：注册设备配置相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type DeviceConfig struct {
}

func (*DeviceConfig) Init(Router *gin.RouterGroup) {
	url := Router.Group("device_config")
	{
		// 增
		url.POST("", api.Controllers.DeviceConfigApi.CreateDeviceConfig)

		// 删
		url.DELETE(":id", api.Controllers.DeviceConfigApi.DeleteDeviceConfig)

		// 改
		url.PUT("", api.Controllers.DeviceConfigApi.UpdateDeviceConfig)

		// 查
		url.GET("", api.Controllers.DeviceConfigApi.HandleDeviceConfigListByPage)

		// 查设备配置下拉菜单
		url.GET("menu", api.Controllers.DeviceConfigApi.HandleDeviceConfigListMenu)

		// 查
		url.GET("/:id", api.Controllers.DeviceConfigApi.HandleDeviceConfigById)

		// 批量修改设备配置
		url.PUT("batch", api.Controllers.DeviceConfigApi.BatchUpdateDeviceConfig)

		// 连接与认证下拉
		url.GET("connect", api.Controllers.DeviceConfigApi.HandleDeviceConfigConnect)

		// 设备配置-连接与认证下拉
		url.GET("voucher_type", api.Controllers.DeviceConfigApi.HandleVoucherType)

		// 单类设备自动化动作下拉菜单
		url.GET("metrics/menu", api.Controllers.DeviceConfigApi.HandleActionByDeviceConfigID)

		// 单类设备自动化条件下拉菜单
		url.GET("metrics/condition/menu", api.Controllers.DeviceConfigApi.HandleConditionByDeviceConfigID)

	}
}
