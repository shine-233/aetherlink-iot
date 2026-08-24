// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。
// 收敛说明: 设备写路径与身份预检已按 gen 继承面收敛批次一改为 raw global.DB / Session NewDB 起点（clone==1 根），
// 杜绝高并发下跨请求 Statement 残留注入陈旧条件（见 references/gen-inheritance-audit.md）。

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
	"gorm.io/gorm"

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
		// 凭证哈希存储 Phase 2b（references/backend-hardening-plan.md 车道1）：devices.voucher
		// 列停写明文——插入行以空串落列（列 NOT NULL DEFAULT ''），调用方入参 struct 保持原值，
		// 让创建方（device_auth.go / gateway_register 等一次性响应）仍能拿到生成时明文。
		rows := make([]*model.Device, len(devices))
		for i, device := range devices {
			if device == nil {
				rows[i] = device
				continue
			}
			blanked := *device
			blanked.Voucher = ""
			rows[i] = &blanked
		}
		if err := tx.Device.Create(rows...); err != nil {
			return err
		}

		// 凭证哈希存储 Phase 1/2b（references/backend-hardening-plan.md 车道1）：gen 模型无
		// VoucherHash 字段，raw UPDATE 收口——同事务内按入参 struct 的明文计算并补写
		// voucher_hash，同时把 voucher 列置空串兜底、逐设备写入网页测试缓存。
		// 此写点覆盖全部创建路径（device_create.go、device_batch_create.go、
		// device_auth.go、device_gateway_register.go 网关与子设备注册）；
		// 停写明文后 voucher_hash 列是唯一匹配依据。
		if err := WriteVoucherHashInQueryTx(tx, devices); err != nil {
			return err
		}

		return autoBindDevicesToDefaultRootGroup(tx, tenantID, devices)
	})
}

// isolatedDevice 返回从全新 gorm Statement 出发的 devices 链起点。
// 与 isolatedDeviceConfig 同理：Session{NewDB:true} 强制每次操作都使用零起点的
// 全新语句，切断 gen 包级单例在高负载下的跨请求 Model/Dest 状态继承。
// 仅用于必须保留 gen 类型化字段表达式的链路；其余写路径统一走 raw global.DB。
func isolatedDevice() query.IDeviceDo {
	return query.Device.Session(&gorm.Session{NewDB: true})
}

// UpdateDevice 更新设备非零字段。批次一收敛（2026-08-24，见
// references/gen-inheritance-audit.md）：改走 raw global.DB 链（clone==1 根，
// 每次起点全新 Statement），杜绝继承链残留 Model/Dest 注入陈旧主键 WHERE；
// struct 非零更新语义与"未命中行报错"契约保持不变。
func UpdateDevice(device *model.Device) (*model.Device, error) {
	info := global.DB.Model(&model.Device{}).
		Where("id = ?", device.ID).
		Updates(device)
	if err := info.Error; err != nil {
		logrus.Error(err)
		return nil, err
	} else if info.RowsAffected == 0 {
		return nil, fmt.Errorf("update device failed, no rows affected")
	}
	return device, nil
}

func UpdateDeviceByMap(deviceID string, deviceMap map[string]interface{}) (*model.Device, error) {
	// 批次一收敛（2026-08-24）：写与回读均改走 raw global.DB 链，杜绝 UPDATE 后
	// SELECT 继承残留 Statement 读到旧快照（CI 实锤根因）；RowsAffected 契约不变。
	info := global.DB.Model(&model.Device{}).
		Where("id = ?", deviceID).
		Updates(deviceMap)
	if err := info.Error; err != nil {
		logrus.Error(err)
		return nil, err
	} else if info.RowsAffected == 0 {
		return nil, fmt.Errorf("update device failed, no rows affected")
	}
	// Return the row after the update so callers see DB-side changes.
	device := &model.Device{}
	if err := global.DB.Model(&model.Device{}).
		Where("id = ?", deviceID).
		First(device).Error; err != nil {
		logrus.Error(err)
		return nil, err
	}
	return device, nil
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

// getDeviceTenantID 查询设备归属租户。批次一收敛（2026-08-24）：模拟器上下线
// 心跳热路径读侧改走 raw global.DB 链（clone==1 根），杜绝继承链残留导致
// INSERT 后 SELECT 读到旧快照（CI 实锤根因）。
func getDeviceTenantID(deviceId string) (string, error) {
	device := &model.Device{}
	if err := global.DB.Model(&model.Device{}).
		Where("id = ?", deviceId).
		First(device).Error; err != nil {
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

// updateDeviceOnlineStatusColumns 更新在线状态列。批次一收敛（2026-08-24）：
// 心跳热路径写侧改走 raw global.DB 链（clone==1 根），杜绝继承链残留注入陈旧
// 主键条件；is_online <> status 幂等守卫、UpdateColumns/Update 分支与
// RowsAffected 语义（未命中行静默成功）保持完全一致。
func updateDeviceOnlineStatusColumns(deviceId string, status int16) (gen.ResultInfo, error) {
	if status == 0 {
		now := time.Now().UTC()
		info := global.DB.Model(&model.Device{}).
			Where("id = ?", deviceId).
			Where("is_online <> ?", status).
			UpdateColumns(map[string]interface{}{
				"is_online":         status,
				"last_offline_time": now,
			})
		return gen.ResultInfo{RowsAffected: info.RowsAffected}, info.Error
	}

	info := global.DB.Model(&model.Device{}).
		Where("id = ?", deviceId).
		Where("is_online <> ?", status).
		Update("is_online", status)
	return gen.ResultInfo{RowsAffected: info.RowsAffected}, info.Error
}

// DeleteDevice 删除设备。批次一收敛（2026-08-24）：改走 raw global.DB 链
// （clone==1 根），杜绝继承链残留注入陈旧主键 WHERE 导致假成功删除；
// "必须恰好命中 1 行"的契约保持不变。
func DeleteDevice(id string, tenantID string) error {
	info := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.Device{})
	if err := info.Error; err != nil {
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
// 批次一收敛（2026-08-24）：直链读起点改走 raw global.DB 链，杜绝继承链残留。
func GetParentDeviceBySubDeviceID(subDeviceID string) (info *model.Device, err error) {
	device := &model.Device{}
	if err = global.DB.Model(&model.Device{}).
		Where("id = ?", subDeviceID).
		First(device).Error; err != nil {
		logrus.Error(err)
		return nil, err
	}
	return device, nil
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
// 批次一收敛（2026-08-24）：改走 raw global.DB 链（clone==1 根）；置 NULL 语义
// 与 RowsAffected 契约保持不变。
func RemoveSubDevice(deviceId string, tenant_id string) error {
	info := global.DB.Model(&model.Device{}).
		Where("id = ? AND tenant_id = ?", deviceId, tenant_id).
		UpdateColumns(map[string]interface{}{
			"parent_id":       nil,
			"sub_device_addr": nil,
		})
	if err := info.Error; err != nil {
		logrus.Error(err)
		return err
	} else if info.RowsAffected == 0 {
		return fmt.Errorf("remove sub device failed, device not found")
	}
	return nil
}

type DeviceQuery struct{}

// 更新指定字段
func (DeviceQuery) Update(ctx context.Context, info *model.Device, option ...field.Expr) error {
	// 批次一收敛（2026-08-24）：需保留类型化 Select(field.Expr)，改走 Session NewDB
	// 起点，切断跨请求 Statement 继承。
	_, err := isolatedDevice().WithContext(ctx).
		Where(query.Device.ID.Eq(info.ID)).
		Select(option...).
		UpdateColumns(info)
	if err != nil {
		logrus.Error(ctx, err)
	}
	return err
}

// 更新设备配置
func (DeviceQuery) ChangeDeviceConfig(deviceID string, deviceConfigID *string) error {
	// 批次一收敛（2026-08-24）：改走 raw global.DB 链（clone==1 根）。
	info := global.DB.Model(&model.Device{}).
		Where("id = ?", deviceID).
		Update("device_config_id", deviceConfigID)
	if err := info.Error; err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("update device config failed, no rows affected")
	}
	return nil
}
