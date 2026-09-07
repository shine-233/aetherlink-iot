// 文件用途:计算字段历史重算 HTTP 入口(PHASE-D-D4)。
// 核心逻辑:创建任务(字段/设备/时间范围)与任务查询;直接调用 calcfield 包(规避导入环)。
package api

import (
	"aetherlink-iot/backend/internal/calcfield"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// HandleCreateCalcfieldRecomputeTask 创建重算任务。
// POST /api/v1/calcfield/recompute
func (*CalculatedFieldApi) HandleCreateCalcfieldRecomputeTask(c *gin.Context) {
	var req struct {
		FieldID  string `json:"field_id"`
		DeviceID string `json:"device_id"`
		FromTS   int64  `json:"from_ts"`
		ToTS     int64  `json:"to_ts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.FieldID == "" || req.DeviceID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "field_id/device_id/from_ts/to_ts are required"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	task, err := calcfield.RecomputeSvc.CreateRecomputeTask(req.FieldID, req.DeviceID, req.FromTS, req.ToTS, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", task)
}

// HandleListCalcfieldRecomputeTasks 任务列表。
// GET /api/v1/calcfield/recompute
func (*CalculatedFieldApi) HandleListCalcfieldRecomputeTasks(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	rows, err := calcfield.RecomputeSvc.ListRecomputeTasks(claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rows)
}

// HandleGetCalcfieldRecomputeTask 任务详情。
// GET /api/v1/calcfield/recompute/:id
func (*CalculatedFieldApi) HandleGetCalcfieldRecomputeTask(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	task, err := calcfield.RecomputeSvc.GetRecomputeTask(c.Param("id"), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", task)
}
