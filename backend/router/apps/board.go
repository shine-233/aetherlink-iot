// 文件用途：注册看板相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Board struct {
}

func (*Board) InitBoard(Router *gin.RouterGroup) {
	url := Router.Group("board")
	{
		// 增
		url.POST("", api.Controllers.BoardApi.CreateBoard)

		// 删
		url.DELETE(":id", api.Controllers.BoardApi.DeleteBoard)
		url.POST(":id/publish", api.Controllers.BoardApi.PublishBoard)

		// 改
		url.PUT("", api.Controllers.BoardApi.UpdateBoard)

		// 查
		url.GET("", api.Controllers.BoardApi.HandleBoardListByPage)

		// 单条详情
		url.GET(":id", api.Controllers.BoardApi.HandleBoard)

		// 首页看板
		url.GET("home", api.Controllers.BoardApi.HandleBoardListByTenantId)

		// 租户设备在线离线趋势图
		url.GET("trend", api.Controllers.BoardApi.GetDeviceTrend)

	}
	// 设备数据
	devices(url)
	// 租客数据
	tenant(url)
	// 用户数据
	user(url)
}

func devices(Router *gin.RouterGroup) {
	url := Router.Group("device")
	// 设备总数
	url.GET("total", api.Controllers.BoardApi.HandleDeviceTotal)
	// 设备总数/激活数
	url.GET("", api.Controllers.BoardApi.HandleDevice)
}

func tenant(Router *gin.RouterGroup) {
	url := Router.Group("tenant")
	// 租户总数
	url.GET("", api.Controllers.BoardApi.HandleTenant)
	// 租户下用户数据
	url.GET("user/info", api.Controllers.BoardApi.HandleTenantUserInfo)
	// 租户下设备数据
	url.GET("device/info", api.Controllers.BoardApi.HandleTenantDeviceInfo)
}

func user(Router *gin.RouterGroup) {
	url := Router.Group("user")
	// 个人信息
	url.GET("info", api.Controllers.BoardApi.HandleUserInfo)
	// 个人信息修改
	url.POST("update", api.Controllers.BoardApi.UpdateUserInfo)
	// 个人密码修改
	url.POST("update/password", api.Controllers.BoardApi.UpdateUserInfoPassword)
}
