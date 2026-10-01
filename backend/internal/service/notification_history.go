// 文件用途：维护通知发送历史和前端查询服务。
// 核心逻辑：按租户、类型和时间条件查询通知记录，并转换为前端可展示响应。
// 关键注意事项：历史记录可能包含接收人和错误信息，分页、脱敏和跨租户过滤必须稳定。
// 重构建议：抽出历史查询仓储，补齐权限、分页、时间范围和敏感字段脱敏测试。
package service

import (
	"aetherlink-iot/backend/internal/authz"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service/kit"
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

// notificationHistoryListScopes 解析通知历史读作用域（ROADMAP C2，自上而下）：
// TENANT_USER 保持 self-only——其可见性由 DAL 的 device-owner 关系 EXISTS 钳制，跨层展开无意义；
// 空租户（SYS_ADMIN 平台空租户行）→ [""]，保持旧行为；
// TENANT_ADMIN/SYS_ADMIN 非空租户 → expandTenantIDScope self∪子孙。
func notificationHistoryListScopes(claims *utils.UserClaims) []string {
	return claimsTenantReadListScopes(claims)
}

func (*NotificationHisory) GetNotificationHistoryListByPage(pageParam *model.GetNotificationHistoryListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := ensureNotificationHistoryOwnerScope(claims); err != nil {
		return nil, err
	}
	if authz.HasRole(claims, authz.TenantUser) {
		// The device relation proves which notification events belong to the
		// caller's devices, but it does not prove that send_target belongs to the
		// caller. Ignore the target filter to avoid using counts as an address
		// oracle; sensitive fields are redacted from the returned rows below.
		pageParam.SendTarget = nil
	}
	total, list, err := dal.GetNotificationHisoryListByPage(pageParam, notificationHistoryListScopes(claims), deviceOwnerUserIDFilterForClaims(claims))
	if err != nil {
		return nil, dbError(err)
	}
	redactNotificationHistoryForTenantUser(list, claims)
	return kit.ListMap(total, list), nil
}

func redactNotificationHistoryForTenantUser(list []*model.NotificationHistory, claims *utils.UserClaims) {
	if claims == nil || !authz.HasRole(claims, authz.TenantUser) {
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
	return dal.CreateNotificationHistory(req, deviceIDs...)
}
