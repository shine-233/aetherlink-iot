// 文件用途：告警指派的 HTTP handler（ROADMAP TB-1 第二片）。
// 核心逻辑：指派（POST）与流水列表（GET）两条路由的入参绑定与 service 转发。
// 关键注意事项：
//  1. handler 只做绑定与转发，租户边界、被指派人归属、告警存在性都在 service 层；
//     不要把校验复制到这里，两处判定迟早会分叉。
//  2. 路由形状统一挂在 history/:id/assignment 下：`history/:id/...` 与
//     `history/assignment/...` 在 Gin 的路由树里会在同一段同时出现参数与静态串而 panic，
//     与评论片（history/:id/comment）同构。
//  3. POST 的正文里 assignee_user_id 允许为 null（= 取消指派），所以**不能**给它加
//     required；是否真的取消由 service 按指针是否为 nil 判定。
//
// 重构建议：若指派要支持批量改派，另开 batch 路由，不要复用单条路径塞数组。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// bindAlarmAssignmentURI 只绑定路径参数，**不做**结构体校验。
//
// 为什么不能顺手校验：POST 的正文此刻还没绑定，对整个结构体跑 validate 会把
// 还没到来的字段误报成 required，让一个合法请求在绑定正文之前就被拒掉。
// 校验统一放在正文绑定之后做。
func bindAlarmAssignmentURI(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindUri(req); err != nil {
		reportParamError(c, err)
		return false
	}
	return true
}

// validateAlarmAssignmentReq 在路径参数与正文都绑定完成后统一校验。
func validateAlarmAssignmentReq(c *gin.Context, req interface{}) bool {
	if err := ValidateStructLang(req, c.GetHeader("Accept-Language")); err != nil {
		reportParamError(c, err)
		return false
	}
	return true
}

// CreateAlarmAssignment 指派或取消指派一条告警历史。
// @Summary 指派告警
// @Description 追加一条指派流水。assignee_user_id 为 null 表示取消指派（历史流水不删不改，当前处理人 = 最新一行）。
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param id path string true "告警历史 ID"
// @Param data body model.CreateAlarmAssignmentReq true "指派内容"
// @Success 200 {object} model.AlarmAssignment
// @Router /api/v1/alarm/info/history/{id}/assignment [post]
func (*AlarmApi) CreateAlarmAssignment(c *gin.Context) {
	var req model.CreateAlarmAssignmentReq
	if !bindAlarmAssignmentURI(c, &req) {
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		reportParamError(c, err)
		return
	}
	if !validateAlarmAssignmentReq(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.Alarm.CreateAlarmAssignment(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ListAlarmAssignments 列出某条告警的指派流水（倒序，首行即当前处理人）。
// @Summary 获取告警指派流水
// @Description 按告警历史 ID 返回指派流水，时间倒序。首行的 assignee_user_id 为 null 表示当前无人指派。
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param id path string true "告警历史 ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alarm/info/history/{id}/assignment [get]
func (*AlarmApi) ListAlarmAssignments(c *gin.Context) {
	var req model.ListAlarmAssignmentsReq
	if !bindAlarmAssignmentURI(c, &req) {
		return
	}
	if !validateAlarmAssignmentReq(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.Alarm.ListAlarmAssignments(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
