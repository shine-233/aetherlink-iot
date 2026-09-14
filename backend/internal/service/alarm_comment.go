// 文件用途：告警评论的 service（ROADMAP TB-1 第一片）。
// 核心逻辑：新增、按告警列出、删除（仅作者或租户管理员）。
// 关键注意事项：
//  1. 评论的租户归属一律取**告警自身的 tenant_id**，不取 claims.TenantID ——
//     SYS_ADMIN 的 claims.TenantID 为空，取 claims 会写出 tenant_id=” 的行，
//     直接撞表上的 CHECK 约束。
//  2. 读写前都必须先过 ensureAlarmHistoryReadAccess：它同时校验告警存在、租户边界
//     与 owner 作用域，漏掉就会出现"能对别的租户的告警留言"。
//     "看得到就能评论"是有意为之——评论是协作面，不是权限提升面。
//  3. 删除是硬删且限作者本人（或 TENANT_ADMIN / SYS_ADMIN）；普通用户不能删别人的话，
//     但管理员需要兜底手段处理不当内容。
//
// 重构建议：若后续要支持编辑与 @提及，改成软删 + 独立 mention 表。
package service

import (
	"errors"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const alarmCommentDeletePermissionMessage = "only the author or a tenant admin can delete this comment"

// CreateAlarmComment 新增一条告警评论。
func (*Alarm) CreateAlarmComment(req *model.CreateAlarmCommentReq, claims *utils.UserClaims) (interface{}, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request cannot be empty")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "comment content cannot be empty")
	}

	history, err := ensureAlarmHistoryReadAccess(req.AlarmHistoryID, claims)
	if err != nil {
		return nil, err
	}

	row := &model.AlarmComment{
		ID: uuid.New().String(),
		// 租户取告警自身，避免 SYS_ADMIN（TenantID 为空）写出空租户行。
		TenantID:       history.TenantID,
		AlarmHistoryID: history.ID,
		Content:        content,
		AuthorUserID:   claims.ID,
		CreatedAt:      time.Now().UTC(),
	}
	if err := dal.CreateAlarmComment(row); err != nil {
		return nil, wrapAlarmDBError(err)
	}
	return row, nil
}

// ListAlarmComments 列出某条告警的评论（时间正序，即对话顺序）。
func (*Alarm) ListAlarmComments(req *model.ListAlarmCommentsReq, claims *utils.UserClaims) (interface{}, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request cannot be empty")
	}
	history, err := ensureAlarmHistoryReadAccess(req.AlarmHistoryID, claims)
	if err != nil {
		return nil, err
	}
	rows, err := dal.ListAlarmComments(history.TenantID, history.ID)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	if rows == nil {
		rows = []*model.AlarmComment{}
	}
	return map[string]interface{}{"list": rows}, nil
}

// DeleteAlarmComment 删除评论：作者本人或 TENANT_ADMIN / SYS_ADMIN 可删。
func (*Alarm) DeleteAlarmComment(req *model.DeleteAlarmCommentReq, claims *utils.UserClaims) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "request cannot be empty")
	}

	// 先确认调用者能看到这条告警，再动评论 —— 否则可以通过猜 comment_id
	// 去探测/删除别的租户的评论。
	if _, err := ensureAlarmHistoryReadAccess(req.AlarmHistoryID, claims); err != nil {
		return err
	}

	row, err := loadAlarmCommentForDelete(claims, req.CommentID)
	if err != nil {
		return err
	}

	// 评论必须真的挂在这条告警上，防止拿 A 告警的权限去删 B 告警的评论。
	if row.AlarmHistoryID != req.AlarmHistoryID {
		return errcode.NewWithMessage(errcode.CodeNotFound, "alarm comment not found")
	}
	if row.AuthorUserID != claims.ID && claims.Authority != constant.TENANT_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, alarmCommentDeletePermissionMessage)
	}

	if err := dal.DeleteAlarmComment(row.TenantID, row.ID); err != nil {
		return wrapAlarmDBError(err)
	}
	return nil
}

// loadAlarmCommentForDelete 取待删评论。
// 常规按 (租户, id) 取；SYS_ADMIN 的 claims.TenantID 为空，按租户取必然落空，
// 故对 SYS_ADMIN 退回 unscoped 查询，并由调用方继续做告警侧校验。
func loadAlarmCommentForDelete(claims *utils.UserClaims, commentID string) (*model.AlarmComment, error) {
	row, err := dal.GetAlarmCommentByID(claims.TenantID, commentID)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wrapAlarmDBError(err)
	}
	if claims.Authority != constant.SYS_ADMIN {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "alarm comment not found")
	}
	row, err = dal.GetAlarmCommentByIDUnscoped(commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "alarm comment not found")
		}
		return nil, wrapAlarmDBError(err)
	}
	return row, nil
}
