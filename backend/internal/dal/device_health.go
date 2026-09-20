// 文件用途：设备综合健康评分（Device Health Score）数据访问层（DAL）。
// 核心逻辑：设备健康评分的 Upsert、租户作用域查询、设备活跃告警关联检索。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
	"fmt"
	"time"

	"gorm.io/gorm/clause"
)

// UpsertDeviceHealthScore 插入或更新单设备健康评分
func UpsertDeviceHealthScore(item *model.DeviceHealthScore) error {
	if item == nil {
		return fmt.Errorf("device health score item is nil")
	}
	now := time.Now()
	item.UpdatedAt = now
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.EvaluatedAt.IsZero() {
		item.EvaluatedAt = now
	}

	return global.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "device_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"score",
			"health_status",
			"alarm_penalty",
			"offline_penalty",
			"anomaly_penalty",
			"details",
			"evaluated_at",
			"updated_at",
		}),
	}).Create(item).Error
}

// GetDeviceHealthScoreByDeviceID 查询单设备健康评分
func GetDeviceHealthScoreByDeviceID(deviceID, tenantID string) (*model.DeviceHealthScore, error) {
	var item model.DeviceHealthScore
	err := global.DB.Where("tenant_id = ? AND device_id = ?", tenantID, deviceID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetDeviceHealthScoresByTenant 查询租户全部设备健康评分列表
func GetDeviceHealthScoresByTenant(tenantID string) ([]*model.DeviceHealthScore, error) {
	var list []*model.DeviceHealthScore
	err := global.DB.Where("tenant_id = ?", tenantID).Order("score ASC").Find(&list).Error
	return list, err
}

// GetDeviceActiveAlarms 查询指定设备当前活跃的告警历史（未恢复：H/M/L）
func GetDeviceActiveAlarms(tenantID, deviceID string) ([]*model.AlarmHistory, error) {
	var list []*model.AlarmHistory
	// 匹配 JSONB 数组中包含 deviceID
	err := global.DB.Where(
		"tenant_id = ? AND alarm_status IN ('H', 'M', 'L') AND (jsonb_exists(COALESCE(alarm_device_list::jsonb, '[]'::jsonb), ?) OR alarm_device_list::text LIKE ?)",
		tenantID,
		deviceID,
		"%"+deviceID+"%",
	).Order("create_at DESC").Find(&list).Error
	return list, err
}

// GetAllTenantActiveAlarms 查询租户全部活跃告警记录
func GetAllTenantActiveAlarms(tenantID string) ([]*model.AlarmHistory, error) {
	var list []*model.AlarmHistory
	err := global.DB.Where("tenant_id = ? AND alarm_status IN ('H', 'M', 'L')", tenantID).Order("create_at DESC").Find(&list).Error
	return list, err
}

// GetTenantDevicesForHealthEvaluation 查询租户下所有已录入设备
func GetTenantDevicesForHealthEvaluation(tenantID string) ([]*model.Device, error) {
	var list []*model.Device
	err := global.DB.Where("tenant_id = ?", tenantID).Find(&list).Error
	return list, err
}

// GetTenantDeviceByID 查询租户下的指定设备
func GetTenantDeviceByID(deviceID, tenantID string) (*model.Device, error) {
	var dev model.Device
	err := global.DB.Where("id = ? AND tenant_id = ?", deviceID, tenantID).First(&dev).Error
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

