// 文件用途：定时报表（ROADMAP D3）HTTP 入口。
// 边界说明：租户边界在 service 层处理；本层只做绑定、claims 提取与错误出口。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

type ReportScheduleApi struct{}

// Create 创建定时报表任务。
// POST /api/v1/report/schedules
func (*ReportScheduleApi) Create(c *gin.Context) {
	var req model.CreateReportScheduleReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ReportSchedule.CreateReportSchedule(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Update 更新定时报表任务。
// PUT /api/v1/report/schedules
func (*ReportScheduleApi) Update(c *gin.Context) {
	var req model.UpdateReportScheduleReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ReportSchedule.UpdateReportSchedule(&req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Delete 删除定时报表任务。
// DELETE /api/v1/report/schedules/:id
func (*ReportScheduleApi) Delete(c *gin.Context) {
	var req struct {
		ID string `json:"id" validate:"required"`
	}
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.ReportSchedule.DeleteReportSchedule(req.ID, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"id": req.ID})
}

// List 列出本租户报表任务。
// GET /api/v1/report/schedules
func (*ReportScheduleApi) List(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ReportSchedule.ListReportSchedules(claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Get 获取单条报表任务。
// GET /api/v1/report/schedules/:id
func (*ReportScheduleApi) Get(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.ReportSchedule.GetReportSchedule(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// RunNow 手动立即触发一次报表生成与投递（调试/验证用）。
// POST /api/v1/report/schedules/:id/run
func (*ReportScheduleApi) RunNow(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.ReportSchedule.RunNow(id, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"id": id, "triggered": true})
}
