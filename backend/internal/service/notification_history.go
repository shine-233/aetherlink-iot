// 文件用途：维护通知发送历史和前端查询服务。
// 核心逻辑：按租户、类型和时间条件查询通知记录，并转换为前端可展示响应。
// 关键注意事项：历史记录可能包含接收人和错误信息，分页、脱敏和跨租户过滤必须稳定。
// 重构建议：抽出历史查询仓储，补齐权限、分页、时间范围和敏感字段脱敏测试。
package service

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

type NotificationHisory struct{}

// NotificationHistory orm define:
// type NotificationHistory struct {
// 	ID               string    `gorm:"column:id;primaryKey" json:"id"`
// 	SendTime         time.Time `gorm:"column:send_time;not null" json:"send_time"`
// 	SendContent      *string   `gorm:"column:send_content" json:"send_content"`
// 	SendTarget       string    `gorm:"column:send_target;not null" json:"send_target"`
// 	SendResult       *string   `gorm:"column:send_result" json:"send_result"`
// 	NotificationType string    `gorm:"column:notification_type;not null" json:"notification_type"`
// 	TenantID         string    `gorm:"column:tenant_id;not null" json:"tenant_id"`
// 	Remark           *string   `gorm:"column:remark" json:"remark"`
// }

// ensureNotificationHistoryOwnerScope rejects unauthenticated reads. Tenant
// users are allowed through because notification history writes now persist
// the affected devices and the DAL applies an all-devices-owned filter. Rows
// without that scope, or rows spanning another owner's device, remain hidden.
func ensureNotificationHistoryOwnerScope(claims *utils.UserClaims) error {
	if claims == nil || (claims.Authority != constant.TENANT_USER && claims.Authority != constant.TENANT_ADMIN && claims.Authority != constant.SYS_ADMIN) {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query notification history")
	}
	return nil
}

func (*NotificationHisory) GetNotificationHistoryListByPage(pageParam *model.GetNotificationHistoryListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := ensureNotificationHistoryOwnerScope(claims); err != nil {
		return nil, err
	}
	if claims.Authority == constant.TENANT_USER {
		// The device relation proves which notification events belong to the
		// caller's devices, but it does not prove that send_target belongs to the
		// caller. Ignore the target filter to avoid using counts as an address
		// oracle; sensitive fields are redacted from the returned rows below.
		pageParam.SendTarget = nil
	}
	pageParam.TenantID = claims.TenantID
	total, list, err := dal.GetNotificationHisoryListByPage(pageParam, deviceOwnerUserIDFilterForClaims(claims))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	redactNotificationHistoryForTenantUser(list, claims)

	return notificationHistoryListResponse(total, list), err
}

func redactNotificationHistoryForTenantUser(list []*model.NotificationHistory, claims *utils.UserClaims) {
	if claims == nil || claims.Authority != constant.TENANT_USER {
		return
	}
	for _, history := range list {
		if history == nil {
			continue
		}
		history.SendTarget = ""
		history.SendContent = nil
		history.Remark = nil
	}
}

func (*NotificationHisory) SaveNotificationHistory(req *model.NotificationHistory, deviceIDs ...string) error {
	err := dal.CreateNotificationHistory(req, deviceIDs...)
	if err != nil {
		return err
	}
	return nil

}

func notificationHistoryListResponse(total interface{}, list interface{}) map[string]interface{} {
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}
}
