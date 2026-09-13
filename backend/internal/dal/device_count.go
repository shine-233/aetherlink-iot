// 文件用途：P3 商业许可证设备配额的计数支持。
// 核心逻辑：全库设备总数（跨租户）——配额是部署级边界，不是租户级。
// 关键注意事项：
//   - 计数包含全部租户：max_devices 声明的是这套部署允许的设备总量。
//   - 用轻量 COUNT(*)，不取实体；设备表无软删除列，物理行数即事实。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// CountAllDevices 统计全库设备总数（部署级，跨租户）。
func CountAllDevices() (int64, error) {
	var count int64
	err := global.DB.Model(&model.Device{}).Count(&count).Error
	return count, err
}
