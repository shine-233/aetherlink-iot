// 文件用途：设备影子消息 DAL 层，封装 device_shadow_messages 表的 CRUD 操作。
// 核心逻辑：创建影子消息、查询待投递列表、标记已投递/过期/取消、清理过期记录。
// 关键注意事项：查询和状态更新必须在同一事务中保证原子性；过期清理由定时任务或上线钩子触发。
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

// GetPendingShadowMessages 查询指定设备的所有 pending 影子消息（按创建时间排序）。
func GetPendingShadowMessages(deviceId string) ([]*model.DeviceShadowMessage, error) {
	var msgs []*model.DeviceShadowMessage
	err := global.DB.
		Where("device_id = ? AND status = 'pending' AND expires_at > now()", deviceId).
		Order("created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

// MarkShadowMessageDelivered 标记影子消息为已投递。
func MarkShadowMessageDelivered(id string) error {
	now := time.Now()
	return global.DB.Model(&model.DeviceShadowMessage{}).
		Where("id = ? AND status = 'pending'", id).
		Updates(map[string]interface{}{"status": "delivered", "delivered_at": &now}).Error
}

// MarkShadowMessageExpired 将过期的 pending 影子消息批量标记为 expired。
func MarkShadowMessageExpired() (int64, error) {
	result := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("status = 'pending' AND expires_at <= now()").
		Update("status", "expired")
	return result.RowsAffected, result.Error
}

// CancelShadowMessage 取消指定的 pending 影子消息。
func CancelShadowMessage(id string) error {
	result := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("id = ? AND status = 'pending'", id).
		Update("status", "canceled")
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// DeleteExpiredShadowMessages 物理删除已过期超过 7 天的影子消息。
func DeleteExpiredShadowMessages() (int64, error) {
	result := global.DB.
		Where("status = 'expired' AND created_at < now() - interval '7 days'").
		Delete(&model.DeviceShadowMessage{})
	return result.RowsAffected, result.Error
}
