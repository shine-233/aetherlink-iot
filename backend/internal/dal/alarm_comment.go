// 文件用途：告警评论的 DAL（ROADMAP TB-1 第一片）。
// 核心逻辑：评论的增、按告警列、按 ID 取、按 ID 删。
// 关键注意事项：本文件所有查询都带 tenant_id 条件（租户边界在 SQL 层收口，
// 不依赖调用方记性）；删除的"仅作者或管理员"判定在 service 层，DAL 只按 ID+租户删。
// 重构建议：若评论量增长到需要分页，把 ListAlarmComments 换成带游标的版本。
package dal

import (
	"context"
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// errAlarmCommentDBNotReady 数据库未初始化时的哨兵错误。
var errAlarmCommentDBNotReady = errors.New("alarm comment dal: database is not initialized")

// CreateAlarmComment 写入一条评论。
func CreateAlarmComment(row *model.AlarmComment) error {
	if global.DB == nil {
		return errAlarmCommentDBNotReady
	}
	return global.DB.WithContext(context.Background()).
		Table(model.TableNameAlarmComment).
		Create(row).Error
}

// ListAlarmComments 按 (租户, 告警) 列出评论，时间正序（对话顺序）。
func ListAlarmComments(tenantID, alarmHistoryID string) ([]*model.AlarmComment, error) {
	if global.DB == nil {
		return nil, errAlarmCommentDBNotReady
	}
	var rows []*model.AlarmComment
	err := global.DB.WithContext(context.Background()).
		Table(model.TableNameAlarmComment).
		Where("tenant_id = ? AND alarm_history_id = ?", tenantID, alarmHistoryID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetAlarmCommentByID 取单条评论（带租户条件）。
func GetAlarmCommentByID(tenantID, id string) (*model.AlarmComment, error) {
	if global.DB == nil {
		return nil, errAlarmCommentDBNotReady
	}
	var row model.AlarmComment
	err := global.DB.WithContext(context.Background()).
		Table(model.TableNameAlarmComment).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetAlarmCommentByIDUnscoped 按 ID 全局取一条评论。
//
// 只给 SYS_ADMIN 路径用：平台管理员的 claims.TenantID 为空，按租户条件查必然落空。
// 调用方**必须**自己再做租户/权限校验，不要把本函数当成普通查询。
func GetAlarmCommentByIDUnscoped(id string) (*model.AlarmComment, error) {
	if global.DB == nil {
		return nil, errAlarmCommentDBNotReady
	}
	var row model.AlarmComment
	err := global.DB.WithContext(context.Background()).
		Table(model.TableNameAlarmComment).
		Where("id = ?", id).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// DeleteAlarmComment 删除评论（带租户条件）。
func DeleteAlarmComment(tenantID, id string) error {
	if global.DB == nil {
		return errAlarmCommentDBNotReady
	}
	return global.DB.WithContext(context.Background()).
		Table(model.TableNameAlarmComment).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.AlarmComment{}).Error
}
