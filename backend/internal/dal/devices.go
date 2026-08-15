// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm/clause"

	"gorm.io/gen"
	"gorm.io/gen/field"

	"github.com/sirupsen/logrus"
)

// CreateDevice inserts one device and, when the tenant has an auto-bind root
// group, creates the device-group relation in the same transaction.
func CreateDevice(device *model.Device) error {
	return createDevicesWithDefaultRootGroup([]*model.Device{device})
}

// CreateDeviceBatch inserts devices and their default root-group relations in
// one transaction. An empty slice is a no-op.
func CreateDeviceBatch(devices []*model.Device) error {
	return createDevicesWithDefaultRootGroup(devices)
}

func createDevicesWithDefaultRootGroup(devices []*model.Device) error {
	if len(devices) == 0 {
		return nil
	}

	tenantID := devices[0].TenantID
	return query.Q.Transaction(func(tx *query.Query) error {
		if err := tx.Device.Create(devices...); err != nil {
			return err
		}

		return autoBindDevicesToDefaultRootGroup(tx, tenantID, devices)
	})
}

func UpdateDevice(device *model.Device) (*model.Device, error) {
	info, err := query.Device.Where(query.Device.ID.Eq(device.ID)).Updates(device)
	if err != nil {
		logrus.Error(err)
		return nil, err
	} else if info.RowsAffected == 0 {
		return nil, fmt.Errorf("update device failed, no rows affected")
	}
	return device, err
}

func UpdateDeviceByMap(deviceID string, deviceMap map[string]interface{}) (*model.Device, error) {
	info, err := query.Device.Where(query.Device.ID.Eq(deviceID)).Updates(deviceMap)
	if err != nil {
		logrus.Error(err)
		return nil, err
	} else if info.RowsAffected == 0 {
		return nil, fmt.Errorf("update device failed, no rows affected")
	}
	// Return the row after the update so callers see DB-side changes.
	device, err := query.Device.Where(query.Device.ID.Eq(deviceID)).First()
	if err != nil {
		logrus.Error(err)
	}
	return device, err
}

// UpdateDeviceStatus updates the latest online status synchronously and returns
// true only when the stored status actually changed.
func UpdateDeviceStatus(deviceId string, status int16) (bool, error) {
	tenantID, err := getDeviceTenantID(deviceId)
	if err != nil {
		return false, err
	}

	statusChanged, err := persistDeviceOnlineStatus(deviceId, status)
	if err != nil {
		return false, err
	}
	if !statusChanged {
		return false, nil
	}

	deleteDeviceCacheAfterStatusUpdate(deviceId)
	saveDeviceStatusHistoryAsync(tenantID, deviceId, status)

	return true, nil
}

func getDeviceTenantID(deviceId string) (string, error) {
	device, err := query.Device.Where(query.Device.ID.Eq(deviceId)).First()
	if err != nil {
		logrus.WithError(err).WithField("device_id", deviceId).Error("Failed to get device info")
		return "", err
	}

	return device.TenantID, nil
}

func persistDeviceOnlineStatus(deviceId string, status int16) (bool, error) {
	info, err := updateDeviceOnlineStatusColumns(deviceId, status)
	if err != nil {
		logrus.Error(err)
		return false, err
	}

	return info.RowsAffected > 0, nil
}

func deleteDeviceCacheAfterStatusUpdate(deviceId string) {
	if global.REDIS == nil {
		return
	}
	if err := global.REDIS.Del(context.Background(), deviceId).Err(); err != nil {
		logrus.WithError(err).WithField("device_id", deviceId).Warn("Failed to delete device cache after status update")
	}
}

func saveDeviceStatusHistoryAsync(tenantID string, deviceId string, status int16) {
	// Status history is best-effort and should not block the live-state update.
	go func() {
		if err := SaveDeviceStatusHistory(tenantID, deviceId, status); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"device_id": deviceId,
				"status":    status,
			}).Warn("Failed to save device status history")
		}
	}()
}

func updateDeviceOnlineStatusColumns(deviceId string, status int16) (gen.ResultInfo, error) {
	if status == 0 {
		now := time.Now().UTC()
		return query.Device.Where(query.Device.ID.Eq(deviceId), query.Device.IsOnline.Neq(status)).
			UpdateColumns(map[string]interface{}{
				"is_online":         status,
				"last_offline_time": now,
			})
	}

	return query.Device.Where(query.Device.ID.Eq(deviceId), query.Device.IsOnline.Neq(status)).
		Update(query.Device.IsOnline, status)
}

func DeleteDevice(id string, tenantID string) error {
	info, err := query.Device.Where(query.Device.ID.Eq(id), query.Device.TenantID.Eq(tenantID)).Delete()
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected != 1 {
		return fmt.Errorf("delete device failed, affected rows: %d", info.RowsAffected)
	}
	return nil
}

// DeleteDeviceWithTx deletes a device inside an existing transaction.
func DeleteDeviceWithTx(id string, tenantID string, tx *query.QueryTx) error {
	info, err := tx.Device.Where(query.Device.ID.Eq(id), query.Device.TenantID.Eq(tenantID)).Delete()
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected != 1 {
		return fmt.Errorf("delete device failed, affected rows: %d", info.RowsAffected)
	}
	return nil
}

// GetParentDeviceBySubDeviceID returns the parent/gateway record for a child device ID.
func GetParentDeviceBySubDeviceID(subDeviceID string) (info *model.Device, err error) {
	device := query.Device
	info, err = device.Where(device.ID.Eq(subDeviceID)).First()
	if err != nil {
		logrus.Error(err)
	}
	return
}

// GetDeviceByIDForUpdate locks the device row inside a transaction for
// read-modify-write updates such as additional_info JSON changes.
func GetDeviceByIDForUpdate(tx *query.QueryTx, id string) (*model.Device, error) {
	device, err := tx.Device.Where(tx.Device.ID.Eq(id)).
		Clauses(clause.Locking{Strength: "UPDATE"}).First()
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("device is nil for id: %s", id)
	}
	return device, nil
}

func GetDeviceByIDForUpdateWithTenant(tx *query.QueryTx, id string, tenantID string) (*model.Device, error) {
	device, err := tx.Device.Where(tx.Device.ID.Eq(id), tx.Device.TenantID.Eq(tenantID)).
		Clauses(clause.Locking{Strength: "UPDATE"}).First()
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("device is nil for id: %s", id)
	}
	return device, nil
}

func BindChildDeviceWithTx(tx *query.QueryTx, childID string, tenantID string, parentID string, subDeviceAddr string) error {
	info, err := tx.Device.Where(
		tx.Device.ID.Eq(childID),
		tx.Device.TenantID.Eq(tenantID),
		tx.Device.ParentID.IsNull(),
		tx.Device.DeviceConfigID.IsNotNull(),
	).Updates(map[string]interface{}{
		"parent_id":       parentID,
		"sub_device_addr": subDeviceAddr,
	})
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected != 1 {
		return fmt.Errorf("bind child device failed, affected rows: %d", info.RowsAffected)
	}
	return nil
}

// UpdateDeviceAdditionalInfoWithTx updates additional_info after the caller has
// locked the row with GetDeviceByIDForUpdate.
func UpdateDeviceAdditionalInfoWithTx(tx *query.QueryTx, deviceID string, additionalInfo string) error {
	info, err := tx.Device.Where(tx.Device.ID.Eq(deviceID)).
		UpdateColumn(tx.Device.AdditionalInfo, additionalInfo)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("update device additional_info failed, no rows affected for id: %s", deviceID)
	}
	return nil
}

// UpdateDeviceOnlineStatus updates the persisted online status only.
func UpdateDeviceOnlineStatus(deviceId string, status int16) error {
	_, err := updateDeviceOnlineStatusColumns(deviceId, status)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

// 移除子设备：将设备的parent_id置为空
func RemoveSubDevice(deviceId string, tenant_id string) error {
	info, err := query.Device.Where(query.Device.ID.Eq(deviceId), query.Device.TenantID.Eq(tenant_id)).UpdateSimple(query.Device.ParentID.Null(), query.Device.SubDeviceAddr.Null())
	if err != nil {
		logrus.Error(err)
	} else if info.RowsAffected == 0 {
		return fmt.Errorf("remove sub device failed, device not found")
	}
	return err
}

type DeviceQuery struct{}

// 更新指定字段
func (DeviceQuery) Update(ctx context.Context, info *model.Device, option ...field.Expr) error {
	device := query.Device
	_, err := query.Device.WithContext(ctx).Where(device.ID.Eq(info.ID)).Select(option...).UpdateColumns(info)
	if err != nil {
		logrus.Error(ctx, err)
	}
	return err
}

// 更新设备配置
func (DeviceQuery) ChangeDeviceConfig(deviceID string, deviceConfigID *string) error {
	device := query.Device
	info, err := device.Where(device.ID.Eq(deviceID)).Update(device.DeviceConfigID, deviceConfigID)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("update device config failed, no rows affected")
	}
	return err
}
