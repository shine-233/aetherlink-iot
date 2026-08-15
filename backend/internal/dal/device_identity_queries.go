// 文件用途：集中设备编号与凭证的唯一性预检查询。
//
// 这些函数保持原有全局精确匹配语义，不自行加入 tenant 条件、trim、大小写
// 归一化、事务或锁；它们只是写入前预检，真正的并发唯一性仍由数据库约束负责。
package dal

import (
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

// CheckDeviceNumberExists checks if a device number already exists in the database.
func CheckDeviceNumberExists(deviceNumber string) (bool, error) {
	count, err := query.Device.Where(query.Device.DeviceNumber.Eq(deviceNumber)).Count()
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

	devices, err := query.Device.
		Where(query.Device.DeviceNumber.In(normalized...)).
		Select(query.Device.DeviceNumber).
		Find()
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
	count, err := query.Device.Where(query.Device.Voucher.Eq(voucher)).
		Where(query.Device.ID.Neq(excludeDeviceID)).Count()
	if err != nil {
		logrus.Error(err)
		return false, err
	}
	return count > 0, nil
}
