// 文件用途：TB-48 统一调度器 HTTP 处理器——三源聚合列表与 scheduler_events 注册面 CRUD。
// 核心逻辑：绑定请求、透传 c.Request.Context() 与 claims，把领域决策全部下沉给
//
//	SchedulerService；接口层只保留参数收敛与统一错误出口（口径同 report_schedule.go）。
//
// 关键注意事项：
//  1. 请求上下文一律 c.Request.Context()，不引入 context.Background()
//     （internal/api 的 context.Background() 有数量预算守卫，预算 4）；
//  2. 租户边界在 service 层 fail-closed（claims 缺失或无租户即拒），本层不解析
//     任何跨租户参数；
//  3. 聚合列表 GET /api/v1/scheduler/events 是只读面：不返回任何执行器内部字段
//     （租约、失败计数等仍由各存量系统的自身接口披露）。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SchedulerApi 统一调度器控制器。
type SchedulerApi struct{}

// schedulerClaims 从中间件注入的 claims 取用户上下文（缺失时 service 层 fail-closed）。
func schedulerClaims(c *gin.Context) *utils.UserClaims {
	value, exists := c.Get("claims")
	if !exists {
		return nil
	}
	claims, _ := value.(*utils.UserClaims)
	return claims
}

// ListSchedulerEvents 只读聚合三套存量调度 + 注册行为统一事件列表（含来源类型）。
// @Summary List unified scheduler events
// @Tags Scheduler
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size (max 500)"
// @Param source_type query string false "scene / report / rpc"
// @Param search query string false "Name contains (case-insensitive)"
// @Param enabled query bool false "Enabled filter"
// @Param from_ms query int false "Window start (epoch ms, next_run_at >= from)"
// @Param to_ms query int false "Window end (epoch ms, next_run_at <= to)"
// @Success 200 {object} model.SchedulerEventListResponse "Aggregated event list"
// @Router /api/v1/scheduler/events [get]
func (*SchedulerApi) ListSchedulerEvents(c *gin.Context) {
	HandlePublic(c, func(req *model.SchedulerEventListRequest) (interface{}, error) {
		logrus.Info("list scheduler events request")
		return service.GroupApp.Scheduler.ListSchedulerEvents(c.Request.Context(), req, schedulerClaims(c))
	})
}

// CreateSchedulerEvent 注册调度事件（scene 事件同步落既有 scene automation timer 机制执行）。
// @Summary Create a scheduler event
// @Tags Scheduler
// @Accept json
// @Produce json
// @Param request body model.SchedulerEventCreateReq true "Scheduler event payload"
// @Success 200 {object} model.SchedulerEvent "Created scheduler event"
// @Router /api/v1/scheduler/events [post]
func (*SchedulerApi) CreateSchedulerEvent(c *gin.Context) {
	HandlePublic(c, func(req *model.SchedulerEventCreateReq) (interface{}, error) {
		logrus.Info("create scheduler event request")
		return service.GroupApp.Scheduler.CreateSchedulerEvent(c.Request.Context(), req, schedulerClaims(c))
	})
}

// GetSchedulerEvent 注册事件详情。
// @Summary Get a scheduler event
// @Tags Scheduler
// @Produce json
// @Param id path string true "Scheduler event id"
// @Success 200 {object} model.SchedulerEvent "Scheduler event"
// @Router /api/v1/scheduler/events/{id} [get]
func (*SchedulerApi) GetSchedulerEvent(c *gin.Context) {
	logrus.Info("get scheduler event request")
	result, err := service.GroupApp.Scheduler.GetSchedulerEvent(c.Request.Context(), c.Param("id"), schedulerClaims(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", result)
}

// UpdateSchedulerEvent 更新注册事件（event_type 不可变；scene 事件同步执行行）。
// @Summary Update a scheduler event
// @Tags Scheduler
// @Accept json
// @Produce json
// @Param id path string true "Scheduler event id"
// @Param request body model.SchedulerEventUpdateReq true "Scheduler event patch"
// @Success 200 {object} model.SchedulerEvent "Updated scheduler event"
// @Router /api/v1/scheduler/events/{id} [put]
func (*SchedulerApi) UpdateSchedulerEvent(c *gin.Context) {
	HandlePublic(c, func(req *model.SchedulerEventUpdateReq) (interface{}, error) {
		logrus.Info("update scheduler event request")
		return service.GroupApp.Scheduler.UpdateSchedulerEvent(c.Request.Context(), c.Param("id"), req, schedulerClaims(c))
	})
}

// DeleteSchedulerEvent 删除注册事件（scene 事件同步删除执行行）。
// @Summary Delete a scheduler event
// @Tags Scheduler
// @Produce json
// @Param id path string true "Scheduler event id"
// @Success 200 {object} object "Deleted event id"
// @Router /api/v1/scheduler/events/{id} [delete]
func (*SchedulerApi) DeleteSchedulerEvent(c *gin.Context) {
	logrus.Info("delete scheduler event request")
	id, err := service.GroupApp.Scheduler.DeleteSchedulerEvent(c.Request.Context(), c.Param("id"), schedulerClaims(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"id": id})
}
