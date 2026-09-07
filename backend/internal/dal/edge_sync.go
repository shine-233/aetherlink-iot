// 文件用途：边缘计算 2.0（ROADMAP D6）数据访问层。
// 核心逻辑：边缘同步任务 CRUD 与状态流转落库；资源快照源（boards/rule_chains/ota_upgrade_packages）
// 与设备定位均带 tenant_id 过滤保证租户隔离。
package dal

import (
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

func CreateEdgeSyncTask(t *model.EdgeSyncTask) error {
	return global.DB.Create(t).Error
}

// GetEdgeSyncTaskInTenant 按租户定位单条任务；未命中返回 gorm.ErrRecordNotFound。
func GetEdgeSyncTaskInTenant(id, tenantID string) (*model.EdgeSyncTask, error) {
	var t model.EdgeSyncTask
	err := global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&t).Error
	return &t, err
}

// ListEdgeSyncTasks 租户内列任务，可按资源类型/网关设备/状态过滤。
func ListEdgeSyncTasks(tenantID, resourceType, gatewayDeviceID, status string, limit int) ([]*model.EdgeSyncTask, error) {
	var list []*model.EdgeSyncTask
	q := global.DB.Where("tenant_id = ?", tenantID)
	if resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	if gatewayDeviceID != "" {
		q = q.Where("gateway_device_id = ?", gatewayDeviceID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&list).Error
	return list, err
}

// UpdateEdgeSyncTaskResult 落库投递结果（成功 synced_at；失败 error 与 attempts）。
func UpdateEdgeSyncTaskResult(id string, status string, taskErr *string, syncedAt interface{}, attempts int) error {
	return global.DB.Model(&model.EdgeSyncTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":    status,
			"error":     taskErr,
			"synced_at": syncedAt,
			"attempts":  attempts,
		}).Error
}

// GetDeviceInTenant 租户内取设备（用于解析网关设备编号与 OTA 目标设备）。
func GetDeviceInTenant(deviceID, tenantID string) (*model.Device, error) {
	var d model.Device
	err := global.DB.
		Where("id = ? AND tenant_id = ?", deviceID, tenantID).
		First(&d).Error
	return &d, err
}

// GetBoardInTenant 租户内取看板（快照源）。
func GetBoardInTenant(boardID, tenantID string) (*model.Board, error) {
	var b model.Board
	err := global.DB.
		Where("id = ? AND tenant_id = ?", boardID, tenantID).
		First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &b, err
}

// GetRuleChainInTenant 租户内取规则链（快照源）。
func GetRuleChainInTenant(chainID, tenantID string) (*model.RuleChain, error) {
	var r model.RuleChain
	err := global.DB.
		Where("id = ? AND tenant_id = ?", chainID, tenantID).
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &r, err
}

// GetOtaPackageInTenant 租户内取升级包（OTA 经边分发源）。
func GetOtaPackageInTenant(packageID, tenantID string) (*model.OtaUpgradePackage, error) {
	var p model.OtaUpgradePackage
	q := global.DB.Where("id = ?", packageID)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &p, err
}
