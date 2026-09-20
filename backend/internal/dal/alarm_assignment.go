// 文件用途：告警指派流水的 DAL（ROADMAP TB-1 第二片）。
// 核心逻辑：写一条指派流水、按 (租户, 告警) 倒序列出流水。
// 关键注意事项：
//  1. 本文件所有查询都带 tenant_id 条件（租户边界在 SQL 层收口，不依赖调用方记性）。
//     跨租户读必须是"未命中"而不是"命中但无权"——后者的报错本身就在泄露存在性。
//  2. 没有 UPDATE / DELETE 接口，是**故意的**：流水只增不改，取消指派靠新写一行
//     assignee_user_id = NULL。需要修数据走运维流程，不从业务代码开口子。
//  3. 倒序（created_at DESC）不是展示偏好：调用方第一个要的是"当前处理人"，
//     倒序能让它落在结果集首行，省掉一次额外的 max(created_at) 查询。
// 重构建议：若流水量增长到需要分页，把 ListAlarmAssignments 换成带游标的版本，
// 但"当前处理人"仍应走倒序首行，不要改成另存一列冗余字段。
package dal

import (
	"context"
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// errAlarmAssignmentDBNotReady 数据库未初始化时的哨兵错误。
var errAlarmAssignmentDBNotReady = errors.New("alarm assignment dal: database is not initialized")

// CreateAlarmAssignment 写入一条指派流水（指派或取消指派）。
func CreateAlarmAssignment(row *model.AlarmAssignment) error {
	if global.DB == nil {
		return errAlarmAssignmentDBNotReady
	}
	return global.DB.WithContext(context.Background()).
		Table(model.TableNameAlarmAssignment).
		Create(row).Error
}

// ListAlarmAssignments 按 (租户, 告警) 列出指派流水，时间倒序。
func ListAlarmAssignments(tenantID, alarmHistoryID string) ([]*model.AlarmAssignment, error) {
	if global.DB == nil {
		return nil, errAlarmAssignmentDBNotReady
	}
	var rows []*model.AlarmAssignment
	err := global.DB.WithContext(context.Background()).
		Table(model.TableNameAlarmAssignment).
		Where("tenant_id = ? AND alarm_history_id = ?", tenantID, alarmHistoryID).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
