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

// MarkShadowMessageSent 标记消息已下发并等待设备 ACK，attempts 递增并写入退避后的下次重投时间。
// 关键：绝不在此直接标 delivered——"发出去"不等于"设备确认收到"，P0.2 之前正是把两者混为一谈。
// 读-改-写放在事务内：attempts 不在生成结构体上，故由 DAL 读取并递增，
// 避免服务层为了拿计数而修改 *.gen.go（生成产物，策略上保留不手改）。
func MarkShadowMessageSent(id string) (int, error) {
	now := time.Now().UTC()
	nextAttempts := 0
	err := global.DB.Transaction(func(tx *gorm.DB) error {
		var current []int
		if err := tx.Model(&model.DeviceShadowMessage{}).
			Where("id = ? AND status = ?", id, model.ShadowStatusPending).
			Pluck("attempts", &current).Error; err != nil {
			return err
		}
		if len(current) == 0 {
			return gorm.ErrRecordNotFound
		}
		nextAttempts = current[0] + 1
		return tx.Model(&model.DeviceShadowMessage{}).
			Where("id = ? AND status = ?", id, model.ShadowStatusPending).
			Updates(map[string]interface{}{
				"status":          model.ShadowStatusSent,
				"sent_at":         &now,
				"attempts":        nextAttempts,
				"next_attempt_at": model.ShadowNextAttemptAt(now, nextAttempts).UTC(),
			}).Error
	})
	if err != nil {
		return 0, err
	}
	return nextAttempts, nil
}

// AckShadowMessage 设备确认送达：仅 pending/sent 可转为 delivered。
// 终态行重复 ACK 返回 gorm.ErrRecordNotFound，避免把历史消息改写成已确认。
func AckShadowMessage(deviceID, id string) error {
	now := time.Now().UTC()
	result := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("id = ? AND device_id = ?", id, deviceID).
		Where("status IN ?", []string{model.ShadowStatusPending, model.ShadowStatusSent}).
		Updates(map[string]interface{}{
			"status":       model.ShadowStatusDelivered,
			"delivered_at": &now,
			"ack_at":       &now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ExpireAndRetryShadowMessages 扫描到期影子消息并推进状态：
//   - sent 且已到退避时间：attempts 达上限转 failed，否则回到 pending 等待重投；
//   - 未终态且超过 TTL：转 expired（TTL 是硬终止，不因重试而延长）。
func ExpireAndRetryShadowMessages() (retried, failed, expired int64, err error) {
	now := time.Now().UTC()

	res := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("status = ? AND next_attempt_at IS NOT NULL AND next_attempt_at <= ? AND attempts >= ?",
			model.ShadowStatusSent, now, model.ShadowMaxAttempts).
		Updates(map[string]interface{}{"status": model.ShadowStatusFailed})
	failed = res.RowsAffected
	if res.Error != nil {
		return retried, failed, expired, res.Error
	}

	res = global.DB.Model(&model.DeviceShadowMessage{}).
		Where("status = ? AND next_attempt_at IS NOT NULL AND next_attempt_at <= ? AND attempts < ?",
			model.ShadowStatusSent, now, model.ShadowMaxAttempts).
		Updates(map[string]interface{}{"status": model.ShadowStatusPending, "next_attempt_at": nil})
	retried = res.RowsAffected
	if res.Error != nil {
		return retried, failed, expired, res.Error
	}

	res = global.DB.Model(&model.DeviceShadowMessage{}).
		Where("status IN ? AND expires_at <= ?",
			[]string{model.ShadowStatusPending, model.ShadowStatusSent}, now).
		Updates(map[string]interface{}{"status": model.ShadowStatusExpired})
	expired = res.RowsAffected
	if res.Error != nil {
		return retried, failed, expired, res.Error
	}
	return retried, failed, expired, nil
}

// ExpireDueShadowMessages 将到期的 pending 影子消息批量标记为 expired，返回受影响行数。
func ExpireDueShadowMessages() (int64, error) {
	result := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("status = ? AND expires_at <= ?", "pending", time.Now().UTC()).
		Update("status", "expired")
	return result.RowsAffected, result.Error
}

// CancelShadowMessage 取消指定设备的 pending 影子消息；目标不存在或非 pending 时返回 gorm.ErrRecordNotFound。
func CancelShadowMessage(deviceID, id string) error {
	result := global.DB.Model(&model.DeviceShadowMessage{}).
		Where("device_id = ? AND id = ? AND status = ?", deviceID, id, "pending").
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
