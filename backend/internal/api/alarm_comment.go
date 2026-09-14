// 文件用途：告警评论的 HTTP handler（ROADMAP TB-1 第一片）。
// 核心逻辑：新增、列出、删除三条路由的入参绑定与 service 转发。
// 关键注意事项：
//  1. handler 只做绑定与转发，租户边界与"谁能删"的判定都在 service 层；
//     不要把权限判断复制到这里，两处判定迟早会分叉。
//  2. 路由形状统一挂在 history/:id/comment 下：`history/:id/...` 与
//     `history/comment/...` 在 Gin 的路由树里会在同一段同时出现参数与静态串而 panic，
//     所以删除也用 :id + :comment_id 两个参数，不另起静态段。
//
// 重构建议：若评论要支持分页或增量拉取，在 ListAlarmComments 上扩展游标参数。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// bindAlarmCommentURI 只绑定路径参数（id / comment_id），**不做**结构体校验。
//
// 为什么不能顺手校验：POST 的请求体字段（content）此刻还没绑定，
// 对整个结构体跑 validate 会把"正文必填"误报成"Field 'Content' is required"，
// 让一个合法请求在绑定正文之前就被拒掉。校验统一放在正文绑定之后做。
func bindAlarmCommentURI(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindUri(req); err != nil {
		reportParamError(c, err)
		return false
	}
	return true
}

// validateAlarmCommentReq 在路径参数与正文都绑定完成后统一校验。
func validateAlarmCommentReq(c *gin.Context, req interface{}) bool {
	if err := ValidateStructLang(req, c.GetHeader("Accept-Language")); err != nil {
		reportParamError(c, err)
		return false
	}
	return true
}

// CreateAlarmComment 新增一条告警评论。
// @Summary 新增告警评论
// @Description 对指定告警历史追加一条评论。评论只增不改；租户边界取告警自身归属。
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param id path string true "告警历史 ID"
// @Param data body model.CreateAlarmCommentReq true "评论内容"
// @Success 200 {object} model.AlarmComment
// @Router /api/v1/alarm/info/history/{id}/comment [post]
func (*AlarmApi) CreateAlarmComment(c *gin.Context) {
	var req model.CreateAlarmCommentReq
	if !bindAlarmCommentURI(c, &req) {
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		reportParamError(c, err)
		return
	}
	if !validateAlarmCommentReq(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.Alarm.CreateAlarmComment(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ListAlarmComments 列出某条告警的评论。
// @Summary 获取告警评论列表
// @Description 按告警历史 ID 返回评论，时间正序（对话顺序）。
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param id path string true "告警历史 ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alarm/info/history/{id}/comment [get]
func (*AlarmApi) ListAlarmComments(c *gin.Context) {
	var req model.ListAlarmCommentsReq
	if !bindAlarmCommentURI(c, &req) {
		return
	}
	if !validateAlarmCommentReq(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.Alarm.ListAlarmComments(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteAlarmComment 删除一条告警评论。
// @Summary 删除告警评论
// @Description 仅评论作者本人、租户管理员或平台管理员可删。
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param id path string true "告警历史 ID"
// @Param comment_id path string true "评论 ID"
// @Success 200
// @Router /api/v1/alarm/info/history/{id}/comment/{comment_id} [delete]
func (*AlarmApi) DeleteAlarmComment(c *gin.Context) {
	var req model.DeleteAlarmCommentReq
	if !bindAlarmCommentURI(c, &req) {
		return
	}
	if !validateAlarmCommentReq(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	if err := service.GroupApp.Alarm.DeleteAlarmComment(&req, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}
