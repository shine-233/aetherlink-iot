// 文件用途：注册OTA 升级相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type OTA struct{}

func (*OTA) InitOTA(Router *gin.RouterGroup) {
	otaapi := Router.Group("ota")
	{
		upgradePackage := otaapi.Group("package")
		{
			upgradePackage.POST("", api.Controllers.OTAApi.CreateOTAUpgradePackage)
			upgradePackage.DELETE(":id", api.Controllers.OTAApi.DeleteOTAUpgradePackage)
			upgradePackage.PUT("", api.Controllers.OTAApi.UpdateOTAUpgradePackage)
			upgradePackage.GET("", api.Controllers.OTAApi.HandleOTAUpgradePackageByPage)
		}

		task := otaapi.Group("task")
		{
			task.POST("preview", api.Controllers.OTAApi.PreviewOTAUpgradeTask)

			task.POST("", api.Controllers.OTAApi.CreateOTAUpgradeTask)

			task.GET(":id/support-bundle", api.Controllers.OTAApi.GetOTAUpgradeTaskSupportBundle)

			task.DELETE(":id", api.Controllers.OTAApi.DeleteOTAUpgradeTask)

			task.GET("", api.Controllers.OTAApi.HandleOTAUpgradeTaskByPage)

			task.GET("detail", api.Controllers.OTAApi.HandleOTAUpgradeTaskDetailByPage)

			task.PUT("detail", api.Controllers.OTAApi.UpdateOTAUpgradeTaskStatus)

			// 只读治理预览：读取 task 状态 + detail 分状态计数，用纯规划器推演下一步动作，不下发、不改行。
			task.GET(":id/governance-preview", api.Controllers.OTAApi.PreviewOTARolloutGovernance)
		}
	}
}
