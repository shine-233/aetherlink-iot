// 文件用途：集中设备编号与凭证的唯一性预检查询。
//
// 这些函数保持原有全局精确匹配语义，不自行加入 tenant 条件、trim、大小写
// 归一化、事务或锁；它们只是写入前预检，真正的并发唯一性仍由数据库约束负责。
//
// 批次一收敛（2026-08-24，见 references/gen-inheritance-audit.md）：预检位于
// 设备创建/激活高频路径，全部改走 raw global.DB 链（clone==1 根，每次链式起点
// 均为全新 Statement），杜绝高并发下 gen 继承链残留 Model/Dest 导致预检读到
// 旧快照（INSERT 后 SELECT 查重漏判/误判的 CI 实锤根因）。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// CheckDeviceNumberExists checks if a device number already exists in the database.
func CheckDeviceNumberExists(deviceNumber string) (bool, error) {
	var count int64
	err := global.DB.Model(&model.Device{}).
		Where("device_number = ?", deviceNumber).
		Count(&count).Error
	if err != nil {
		logrus.Error(err)
		return false, err
	}
	return count > 0, nil
}

// CheckDeviceNumbersExists returns the subset of exact device numbers already stored.
func CheckDeviceNumbersExists(deviceNumbers []string) (map[string]bool, error) {
	existing := make(map[string]bool, len(deviceNumbers))
	normalized := make([]string, 0, len(deviceNumbers))
	seen := make(map[string]struct{}, len(deviceNumbers))
	for _, deviceNumber := range deviceNumbers {
		if deviceNumber == "" {
			continue
		}
		if _, ok := seen[deviceNumber]; ok {
			continue
		}
		seen[deviceNumber] = struct{}{}
		normalized = append(normalized, deviceNumber)
	}
	if len(normalized) == 0 {
		return existing, nil
	}

	var devices []*model.Device
	err := global.DB.Model(&model.Device{}).
		Where("device_number IN ?", normalized).
		Select("device_number").
		Find(&devices).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	for _, device := range devices {
		existing[device.DeviceNumber] = true
	}
	return existing, nil
}

// CheckVoucherExists checks whether a voucher belongs to another device.
func CheckVoucherExists(voucher string, excludeDeviceID string) (bool, error) {
	var count int64
	err := global.DB.Model(&model.Device{}).
		Where("voucher = ?", voucher).
		Where("id <> ?", excludeDeviceID).
		Count(&count).Error
	if err != nil {
		logrus.Error(err)
		return false, err
	}
	return count > 0, nil
}
