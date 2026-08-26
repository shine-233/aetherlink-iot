// 文件用途：设备影子消息 DAL 层，封装 device_shadow_messages 表的 CRUD 操作。
// 核心逻辑：创建影子消息、查询待投递/全状态列表、标记已投递/过期/取消、清理过期记录。
// 关键注意事项：时间比较一律用参数化的 time.Time，禁止 now()/interval 等 PG 专有语法，
// 保证 sqlite 单测与 PostgreSQL 生产语义一致；过期清理由定时任务或上线钩子触发。
package dal

import (
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateShadowMessage 写入一条影子消息。
func CreateShadowMessage(msg *model.DeviceShadowMessage) error {
	return global.DB.Create(msg).Error
}

// GetPendingShadowMessages 查询指定设备所有未过期的 pending 影子消息（按创建时间排序）。
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetPendingShadowMessages(deviceId string) ([]*model.DeviceShadowMessage, error) {
	var msgs []*model.DeviceShadowMessage
	err := global.DB.
		Where("device_id = ? AND status = ? AND expires_at > ?", deviceId, "pending", time.Now().UTC()).
		Order("created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

// GetAllShadowMessages 查询指定设备的影子消息（可按状态过滤，status 为空时返回全部），新→旧排序。
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetAllShadowMessages(deviceId, status string) ([]*model.DeviceShadowMessage, error) {
	query := global.DB.Where("device_id = ?", deviceId)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var msgs []*model.DeviceShadowMessage
	err := query.Order("created_at DESC").Find(&msgs).Error
	return msgs, err
}

// CountShadowMessagesByDevice 统计指定设备各状态的影子消息数量。
// tenant-scope: caller-enforced?2026-08-26 ?????
func CountShadowMessagesByDevice(deviceId string) (map[string]int64, error) {
	type row struct {
		Status string `gorm:"column:status"`
		Count  int64  `gorm:"column:count"`
	}
	var rows []row
	err := global.DB.Model(&model.DeviceShadowMessage{}).
		Select("status, COUNT(*) AS count").
		Where("device_id = ?", deviceId).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, r := range rows {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

// MarkShadowMessageDelivered 标记影子消息为已投递。
func MarkShadowMessageDelivered(id string) error {
	now := time.Now().UTC()
	return global.DB.Model(&model.DeviceShadowMessage{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{"status": "delivered", "delivered_at": &now}).Error
}

// ExpireDueShadowMessages 将到期的 pending 影子消息批量标记为 expired，返回受影响行数。
func ExpireDueShadowMessages() (int64, error) {
	result := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("status = ? AND expires_at <= ?", "pending", time.Now().UTC()).
		Update("status", "expired")
	return result.RowsAffected, result.Error
}

// CancelShadowMessage 取消指定的 pending 影子消息；目标不存在或非 pending 时返回 gorm.ErrRecordNotFound。
func CancelShadowMessage(id string) error {
	result := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("id = ? AND status = ?", id, "pending").
		Update("status", "canceled")
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// DeleteStaleShadowMessages 物理删除终态（expired/canceled）且到期超过 7 天的影子消息。
// 保留期锚定 expires_at：它是该消息"不再相关"的业务时间点，对两种终态语义一致。
func DeleteStaleShadowMessages() (int64, error) {
	result := global.DB.
		Where("status IN (?, ?) AND expires_at < ?", "expired", "canceled", time.Now().UTC().Add(-7*24*time.Hour)).
		Delete(&model.DeviceShadowMessage{})
	return result.RowsAffected, result.Error
}
