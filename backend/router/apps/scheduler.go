// 文件用途：TB-48 统一调度器（scheduler_events + 三源聚合）路由定义。
// 核心逻辑：挂载 /api/v1/scheduler/events 路由组并关联 SchedulerApi：
//
//	聚合列表（只读）+ 注册面 CRUD（创建/详情/更新/删除）。
//
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约；
//
//	新增路由必须同步在迁移（137.sql）做 Casbin g2/p 登记，否则启动审计 fail-fast；
//	路由组挂在 CasbinRBAC 中间件之后（router_init.go），路由面受资源表保护。
//
// 重构建议：若后续把 report/rpc 注册事件接到各自存量执行器（本阶段明确不做），
//
//	应按 event_type 拆分写面路径并单独评审授权，不在本路由组上混挂。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// SchedulerRouter 统一调度器路由组。
type SchedulerRouter struct{}

// InitScheduler 注册统一调度器路由。
func (*SchedulerRouter) InitScheduler(Router *gin.RouterGroup) {
	r := Router.Group("scheduler/events")
	{
		r.GET("", api.Controllers.SchedulerApi.ListSchedulerEvents)
		r.POST("", api.Controllers.SchedulerApi.CreateSchedulerEvent)
		r.GET(":id", api.Controllers.SchedulerApi.GetSchedulerEvent)
		r.PUT(":id", api.Controllers.SchedulerApi.UpdateSchedulerEvent)
		r.DELETE(":id", api.Controllers.SchedulerApi.DeleteSchedulerEvent)
	}
}
