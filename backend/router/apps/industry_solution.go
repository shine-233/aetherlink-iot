// 文件用途：注册行业方案模板引擎（TB-19）应用路由。
// 核心逻辑：挂载方案创建/列表/详情/删除与一键安装路由。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type IndustrySolution struct{}

func (*IndustrySolution) InitIndustrySolution(Router *gin.RouterGroup) {
	sol := Router.Group("solutions")
	{
		sol.POST("", api.Controllers.IndustrySolutionApi.CreateIndustrySolution)
		sol.GET("", api.Controllers.IndustrySolutionApi.ListIndustrySolutions)
		sol.GET(":id", api.Controllers.IndustrySolutionApi.GetIndustrySolution)
		sol.DELETE(":id", api.Controllers.IndustrySolutionApi.DeleteIndustrySolution)
		sol.POST(":id/install", api.Controllers.IndustrySolutionApi.InstallIndustrySolution)
	}
}
