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

// deviceActiveAlarmsWhereSQL 设备健康面板"设备当前活跃告警"过滤条件（142.sql 关联表形态）。
//
// 旧写法 jsonb_exists(alarm_device_list::jsonb, ?) OR alarm_device_list::text LIKE '%id%'
// 需要对租户内每一行 alarm_history 做 jsonb 展开 + 子串匹配，无法走索引；LIKE 分支还会
// 在设备 id 互为子串时误命中（dev-1 命中 dev-10 的告警），把别的设备的告警算进健康扣分。
//
// 现改为在 alarm_history_devices 上做等值 EXISTS。与 alarm_history.go 的
// alarmHistoryDeviceExistsByIDUnqualified 相比，这里额外带上 ahd.tenant_id 等值条件：
//   - 让子查询完整命中 idx_alarm_history_devices_device_tenant(device_id, tenant_id)，
//     PG 可把 EXISTS 规划为以关联表为驱动的半连接，再按主键回表 alarm_history；
//   - 关联表的 tenant_id 由触发器从告警行冗余而来，与外层 tenant_id 恒等，语义不变。
//
// 关联表由 142.sql 的 trg_alarm_history_devices_sync 触发器与 alarm_device_list 同事务
// 同步，并对存量行做过回填，因此读侧切换无需应用写路径配合。
const deviceActiveAlarmsWhereSQL = `alarm_history.tenant_id = ?
  AND alarm_history.alarm_status IN ('H', 'M', 'L')
  AND EXISTS (
    SELECT 1
    FROM alarm_history_devices ahd
    WHERE ahd.alarm_history_id = alarm_history.id
      AND ahd.device_id = ?
      AND ahd.tenant_id = ?
)`

// GetDeviceActiveAlarms 查询指定设备当前活跃的告警历史（未恢复：H/M/L），按创建时间倒序。
// 签名与返回形态保持不变；空 deviceID 不可能命中关联表（device_id 为 NOT NULL 的真实 id），
// 直接短路返回空列表，避免一次无意义的查询。
// tenant-scope: sql-filtered——alarm_history.tenant_id = ? 与 ahd.tenant_id = ?（deviceActiveAlarmsWhereSQL 首条件）。
func GetDeviceActiveAlarms(tenantID, deviceID string) ([]*model.AlarmHistory, error) {
	list := make([]*model.AlarmHistory, 0)
	if deviceID == "" {
		return list, nil
	}
	err := global.DB.Model(&model.AlarmHistory{}).
		Where(deviceActiveAlarmsWhereSQL, tenantID, deviceID, tenantID).
		Order("alarm_history.create_at DESC").
		Find(&list).Error
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
