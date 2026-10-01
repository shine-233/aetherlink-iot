// alarm_realtime.go 提供告警实时推送（TB-30 告警状态 WebSocket）所需的数据访问：
// 订阅建立时的初始快照查询。发布侧与频道约定在 service/alarm_realtime.go。
//
// 关键约束：快照必须限定租户，且只读、无副作用；limit 由调用方收敛（默认 20，上限 100），
// 防止 WS 握手路径被大结果集拖垮。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"context"
)

// ListRecentAlarmHistoryForSnapshot 返回租户内最近的告警历史（按创建时间倒序），
// 用作告警实时 WebSocket 订阅建立后的初始快照。
func ListRecentAlarmHistoryForSnapshot(ctx context.Context, tenantID string, limit int) ([]*model.AlarmHistory, error) {
	if tenantID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return query.AlarmHistory.
		Where(query.AlarmHistory.TenantID.Eq(tenantID)).
		Order(query.AlarmHistory.CreateAt.Desc()).
		Limit(limit).
		Find()
}
