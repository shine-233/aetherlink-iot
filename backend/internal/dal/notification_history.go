// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const notificationHistoryOwnerExistsSQL = `EXISTS (
	SELECT 1
	FROM notification_history_devices scoped_history_device
	INNER JOIN devices scoped_device
		ON scoped_device.id = scoped_history_device.device_id
		AND scoped_device.tenant_id = scoped_history_device.tenant_id
	WHERE scoped_history_device.notification_history_id = nh.id
		AND scoped_history_device.tenant_id = nh.tenant_id
		AND scoped_device.owner_user_id = ?
)`

const notificationHistoryForeignOwnerExistsSQL = `EXISTS (
	SELECT 1
	FROM notification_history_devices scoped_history_device
	LEFT JOIN devices scoped_device
		ON scoped_device.id = scoped_history_device.device_id
		AND scoped_device.tenant_id = nh.tenant_id
	WHERE scoped_history_device.notification_history_id = nh.id
		AND (
			scoped_history_device.tenant_id <> nh.tenant_id
			OR scoped_device.id IS NULL
			OR scoped_device.owner_user_id IS NULL
			OR scoped_device.owner_user_id <> ?
		)
)`

func GetNotificationHisoryListByPage(notifications *model.GetNotificationHistoryListByPageReq, ownerUserID *string) (int64, []*model.NotificationHistory, error) {
	var count int64
	queryBuilder := global.DB.WithContext(context.Background()).
		Table(model.TableNameNotificationHistory+" AS nh").
		Where("nh.tenant_id = ?", notifications.TenantID)
	if ownerUserID != nil && strings.TrimSpace(*ownerUserID) != "" {
		ownerID := strings.TrimSpace(*ownerUserID)
		queryBuilder = queryBuilder.
			Where(notificationHistoryOwnerExistsSQL, ownerID).
			Where("NOT "+notificationHistoryForeignOwnerExistsSQL, ownerID)
	}
	if notifications.NotificationType != nil && *notifications.NotificationType != "" {
		queryBuilder = queryBuilder.Where("nh.notification_type LIKE ?", fmt.Sprintf("%%%s%%", *notifications.NotificationType))
	}

	if notifications.SendTarget != nil && *notifications.SendTarget != "" {
		queryBuilder = queryBuilder.Where("nh.send_target = ?", *notifications.SendTarget)
	}

	if notifications.SendTimeStart != nil && notifications.SendTimeStop != nil {
		queryBuilder = queryBuilder.Where("nh.send_time BETWEEN ? AND ?", *notifications.SendTimeStart, *notifications.SendTimeStop)
	}

	if err := queryBuilder.Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	queryBuilder = queryBuilder.Session(&gorm.Session{}).
		Order("nh.send_time DESC")
	if notifications.Page != 0 && notifications.PageSize != 0 {
		queryBuilder = queryBuilder.
			Limit(notifications.PageSize).
			Offset((notifications.Page - 1) * notifications.PageSize)
	}

	notificationList := make([]*model.NotificationHistory, 0)
	if err := queryBuilder.Find(&notificationList).Error; err != nil {
		logrus.Error("queryBuilder.Find error: ", err)
		return count, nil, err
	}
	return count, notificationList, nil
}

func normalizeNotificationHistoryDeviceIDs(deviceIDs []string) []string {
	normalized := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, rawDeviceID := range deviceIDs {
		deviceID := strings.TrimSpace(rawDeviceID)
		if deviceID == "" {
			continue
		}
		if _, exists := seen[deviceID]; exists {
			continue
		}
		seen[deviceID] = struct{}{}
		normalized = append(normalized, deviceID)
	}
	return normalized
}

func CreateNotificationHistory(notificationHistory *model.NotificationHistory, deviceIDs ...string) error {
	normalizedDeviceIDs := normalizeNotificationHistoryDeviceIDs(deviceIDs)
	err := global.DB.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(notificationHistory).Error; err != nil {
			return err
		}
		if len(normalizedDeviceIDs) == 0 {
			return nil
		}

		var tenantDeviceCount int64
		if err := tx.Model(&model.Device{}).
			Where("id IN ? AND tenant_id = ?", normalizedDeviceIDs, notificationHistory.TenantID).
			Count(&tenantDeviceCount).Error; err != nil {
			return err
		}
		if tenantDeviceCount != int64(len(normalizedDeviceIDs)) {
			return fmt.Errorf("notification history device scope does not belong to tenant %s", notificationHistory.TenantID)
		}

		links := make([]*model.NotificationHistoryDevice, 0, len(normalizedDeviceIDs))
		for _, deviceID := range normalizedDeviceIDs {
			links = append(links, &model.NotificationHistoryDevice{
				NotificationHistoryID: notificationHistory.ID,
				DeviceID:              deviceID,
				TenantID:              notificationHistory.TenantID,
			})
		}
		return tx.Create(&links).Error
	})
	if err != nil {
		logrus.Error(err)
		return err
	}
	return nil
}

func UpdateNotificationHistory(id string, status *string, remark *string) (int64, error) {
	q := query.NotificationHistory
	updates := make(map[string]interface{})

	if status != nil {
		updates["send_result"] = *status
	}
	if remark != nil {
		updates["remark"] = *remark
	}

	result, err := q.Where(q.ID.Eq(id)).Updates(updates)
	if err != nil {
		logrus.Error("update notification history failed:", err)
		return 0, err
	}

	return result.RowsAffected, nil
}

func UpdateNotificationHistoryWithContent(id string, status *string, remark *string, content *string) (int64, error) {
	q := query.NotificationHistory
	updates := make(map[string]interface{})

	if status != nil {
		updates["send_result"] = *status
	}
	if remark != nil {
		updates["remark"] = *remark
	}
	if content != nil {
		updates["send_content"] = *content
	}

	result, err := q.Where(q.ID.Eq(id)).Updates(updates)
	if err != nil {
		logrus.Error("update notification history failed:", err)
		return 0, err
	}

	return result.RowsAffected, nil
}
