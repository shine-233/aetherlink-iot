// 文件用途：设备 Modbus 点表 DAL（ROADMAP B1）。
// 核心逻辑：按 device_id 一机一行 upsert / 读取点表 JSON。
// 关键注意事项：租户边界由 service 层的设备访问守卫保证，本层只按主键读写；
//   profile 为任意 JSON 文本，结构校验在 service 层完成。
package dal

import (
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// UpsertDeviceModbusProfile 写入（或更新）设备点表，返回更新时间。
func UpsertDeviceModbusProfile(deviceId, profileJSON, updatedBy string) (time.Time, error) {
	now := time.Now().UTC()
	row := model.DeviceModbusProfile{
		DeviceID:  deviceId,
		Profile:   &profileJSON,
		UpdatedAt: &now,
	}
	if updatedBy != "" {
		row.UpdatedBy = &updatedBy
	}
	err := global.DB.
		Where("device_id = ?", deviceId).
		Assign(row).
		FirstOrCreate(&row).Error
	return now, err
}

// GetDeviceModbusProfile 读取设备点表；不存在时返回 nil 而非错误。
// GetDeviceModbusProfile 按 设备+租户 双条件读取点表（tenant-scope 棘轮要求，
// 不再依赖 service 层 check-then-act 单独兜底）；不存在时返回 (nil, nil)。
func GetDeviceModbusProfile(deviceId, tenantID string) (*model.DeviceModbusProfile, error) {
	var row model.DeviceModbusProfile
	err := global.DB.
		Table("device_modbus_profiles p").
		Select("p.*").
		Joins("JOIN devices d ON d.id = p.device_id AND d.tenant_id = ?", tenantID).
		Where("p.device_id = ?", deviceId).
		First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
