// 文件用途：按设备读取 OTA 升级进度（ROADMAP P1.4 移动端）。
// 核心逻辑：取指定设备最近一条 OTA 升级明细，用于移动端展示"这台设备升级到哪一步"。
// 关键注意事项：
//  1. **租户过滤必须走包路径**。`ota_upgrade_task_details` 自身没有 tenant_id 列，
//     租户信息挂在 `ota_upgrade_tasks.ota_upgrade_package_id -> ota_upgrade_packages.tenant_id`
//     上。只按 device_id 查会跨租户返回明细：设备 ID 是全局唯一的 UUID，
//     猜到/拿到别租户的设备 ID 就能读出对方的升级进度。
//  2. 只取最近一条（按 updated_at 倒序 LIMIT 1）：同一台设备可能参加过多次升级任务，
//     返回多条再让调用方挑，等于把"哪条是最新的"这个判断推给上层，各端会挑出不同答案。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// LatestOTAUpgradeDetailForDevice 返回设备最近一条 OTA 升级明细。
// 无记录时返回 (nil, nil)——"没参加过升级"是正常状态，不是错误，
// 调用方据此返回 none 而不是报错。
func LatestOTAUpgradeDetailForDevice(tenantID, deviceID string) (*model.OtaUpgradeTaskDetail, error) {
	if global.DB == nil {
		return nil, gorm.ErrInvalidDB
	}
	var detail model.OtaUpgradeTaskDetail
	err := global.DB.
		Table(model.TableNameOtaUpgradeTaskDetail+" AS d").
		Select("d.*").
		Joins("JOIN "+model.TableNameOtaUpgradeTask+" AS t ON t.id = d.ota_upgrade_task_id").
		Joins("JOIN "+model.TableNameOtaUpgradePackage+" AS p ON p.id = t.ota_upgrade_package_id").
		Where("d.device_id = ? AND p.tenant_id = ?", deviceID, tenantID).
		Order("d.updated_at DESC").
		Limit(1).
		Scan(&detail).Error
	if err != nil {
		return nil, err
	}
	if detail.ID == "" {
		return nil, nil
	}
	return &detail, nil
}
