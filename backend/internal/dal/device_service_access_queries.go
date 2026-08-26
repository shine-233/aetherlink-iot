// 文件用途：集中服务接入点与设备关联的读取、计数和批量分组查询。
//
// device_query_reads.go 保留通用设备读取；本文件维持既有导出函数和 SQL 条件，
// 不新增租户语义。调用方仍须先完成服务接入点的租户与权限校验。
package dal

import (
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

// GetServiceDeviceList 查询服务接入点关联的完整设备记录。
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetServiceDeviceList(serviceAccessID string) ([]model.Device, error) {
	var devices []model.Device
	err := query.Device.Where(query.Device.ServiceAccessID.Eq(serviceAccessID)).Scan(&devices)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return devices, err
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func CountServiceDevicesByAccessID(serviceAccessID string) (int64, error) {
	if serviceAccessID == "" {
		return 0, nil
	}

	device := query.Device
	count, err := device.Where(device.ServiceAccessID.Eq(serviceAccessID)).Count()
	if err != nil {
		logrus.Error(err)
		return 0, err
	}
	return count, nil
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetServiceDeviceListByNumbers(serviceAccessID string, deviceNumbers []string) ([]model.Device, error) {
	normalizedNumbers := normalizeServiceDeviceNumbers(deviceNumbers)
	if serviceAccessID == "" || len(normalizedNumbers) == 0 {
		return []model.Device{}, nil
	}

	device := query.Device
	var devices []model.Device
	err := device.Where(
		device.ServiceAccessID.Eq(serviceAccessID),
		device.DeviceNumber.In(normalizedNumbers...),
	).Select(device.DeviceNumber, device.DeviceConfigID).Scan(&devices)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return devices, nil
}

func normalizeServiceDeviceNumbers(deviceNumbers []string) []string {
	normalized := make([]string, 0, len(deviceNumbers))
	seen := make(map[string]struct{}, len(deviceNumbers))
	for _, deviceNumber := range deviceNumbers {
		deviceNumber = strings.TrimSpace(deviceNumber)
		if deviceNumber == "" {
			continue
		}
		if _, ok := seen[deviceNumber]; ok {
			continue
		}
		seen[deviceNumber] = struct{}{}
		normalized = append(normalized, deviceNumber)
	}
	return normalized
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetServiceDevicesByAccessIDs(serviceAccessIDs []string) (map[string][]model.Device, error) {
	result := make(map[string][]model.Device, len(serviceAccessIDs))
	normalizedIDs := normalizeDeviceIDs(serviceAccessIDs)
	if len(normalizedIDs) == 0 {
		return result, nil
	}

	var devices []model.Device
	err := query.Device.Where(query.Device.ServiceAccessID.In(normalizedIDs...)).Scan(&devices)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	for _, device := range devices {
		if device.ServiceAccessID == nil || strings.TrimSpace(*device.ServiceAccessID) == "" {
			continue
		}
		accessID := strings.TrimSpace(*device.ServiceAccessID)
		result[accessID] = append(result[accessID], device)
	}
	return result, nil
}
