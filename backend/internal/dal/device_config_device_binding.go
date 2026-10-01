// 文件用途: device_config ↔ device 绑定子聚合（9-28 DAL 拆分，自 device_config.go 按聚合迁出）。
// 核心逻辑: 跨档案/设备两表的绑定写入（devices.device_config_id 批量/单个改绑）与
//   档案维度的活跃设备计数（列表页 DeviceCount 数据源）。
// 关键注意事项: 改绑写的是 devices 表，租户校验由 service 层前置完成；活跃设备计数按
//   activate_flag='active' 过滤。导出符号与签名与拆分前完全一致。
// 重构建议: 绑定批量更新可考虑事务化包裹，避免分批中途失败留下半改绑状态。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

func deviceConfigIDs(deviceconfigList []*model.DeviceConfig) []string {
	ids := make([]string, 0, len(deviceconfigList))
	for _, deviceConfig := range deviceconfigList {
		if deviceConfig == nil || deviceConfig.ID == "" {
			continue
		}
		ids = append(ids, deviceConfig.ID)
	}
	return ids
}

func countActiveDevicesByConfigIDs(deviceConfigIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(deviceConfigIDs))
	if len(deviceConfigIDs) == 0 {
		return counts, nil
	}
	type row struct {
		DeviceConfigID string `gorm:"column:device_config_id"`
		Count          int64  `gorm:"column:count"`
	}
	rows := make([]row, 0, len(deviceConfigIDs))
	err := global.DB.Model(&model.Device{}).
		Select("device_config_id, count(*) as count").
		Where("activate_flag = ? AND device_config_id IN ?", "active", deviceConfigIDs).
		Group("device_config_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		counts[item.DeviceConfigID] = item.Count
	}
	return counts, nil
}

const updateDeviceConfigBatchSize = 500

func UpdateDeviceDeviceConfigIDs(deviceIDs []string, deviceConfigID *string) error {
	normalizedIDs := normalizeDeviceIDs(deviceIDs)
	for start := 0; start < len(normalizedIDs); start += updateDeviceConfigBatchSize {
		end := start + updateDeviceConfigBatchSize
		if end > len(normalizedIDs) {
			end = len(normalizedIDs)
		}
		_, err := query.Device.
			Where(query.Device.ID.In(normalizedIDs[start:end]...)).
			Update(query.Device.DeviceConfigID, deviceConfigID)
		if err != nil {
			logrus.Error(err)
			return err
		}
	}
	return nil
}
