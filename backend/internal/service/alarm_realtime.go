// alarm_realtime.go 实现 TB-30 告警状态实时订阅的发布侧：
// 告警生命周期事件（触发、恢复、处理状态变更）经 Redis Pub/Sub 扇出到所有后端实例，
// 由 api/alarm_status_ws.go 的租户级 WebSocket 订阅转发给前端。
//
// 关键约束：
// - 实时推送是尽力而为：发布失败只记日志，绝不影响产生事件的告警事务本身；
// - 频道按租户隔离（alarm:tenant:{tenant_id}），WS 订阅侧以 JWT claims 的租户为准；
// - 事件载荷只含告警摘要字段，不携带通知凭据等敏感信息。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// AlarmStatusChannel 返回租户级告警实时事件频道。
func AlarmStatusChannel(tenantID string) string {
	return "alarm:tenant:" + tenantID
}

// PublishAlarmEvent 将一条告警生命周期事件发布到租户频道。
func PublishAlarmEvent(ctx context.Context, tenantID string, event map[string]interface{}) {
	if tenantID == "" || global.REDIS == nil {
		return
	}
	event["timestamp"] = time.Now().UnixMilli()
	payload, err := json.Marshal(event)
	if err != nil {
		logrus.WithError(err).Error("marshal alarm realtime event failed")
		return
	}
	if err := global.REDIS.Publish(ctx, AlarmStatusChannel(tenantID), payload).Err(); err != nil {
		logrus.WithError(err).WithField("tenant_id", tenantID).Debug("alarm realtime publish failed")
	}
}

// alarmHistorySnapshotItem 把告警历史行收敛成 WS 快照条目。
func alarmHistorySnapshotItem(row *model.AlarmHistory) map[string]interface{} {
	item := map[string]interface{}{
		"alarm_id":   row.ID,
		"name":       row.Name,
		"level":      row.AlarmStatus,
		"device_ids": parseAlarmDeviceListIDs(row.AlarmDeviceList),
		"create_at":  row.CreateAt.UnixMilli(),
	}
	if row.Content != nil {
		item["content"] = *row.Content
	}
	return item
}

// parseAlarmDeviceListIDs 解析 alarm_device_list 里的设备 ID 列表（兼容 JSON 数组与逗号分隔）。
func parseAlarmDeviceListIDs(raw string) []string {
	result := make([]string, 0)
	if raw == "" {
		return result
	}
	var fromJSON []string
	if err := json.Unmarshal([]byte(raw), &fromJSON); err == nil {
		return append(result, fromJSON...)
	}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// RecentAlarmSnapshot 返回租户内最近 limit 条告警历史，作为订阅建立时的初始快照。
func (*Alarm) RecentAlarmSnapshot(tenantID string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := dal.ListRecentAlarmHistoryForSnapshot(context.Background(), tenantID, limit)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, alarmHistorySnapshotItem(row))
	}
	return items, nil
}
