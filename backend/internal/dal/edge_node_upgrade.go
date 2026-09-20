// 文件用途：P1.5 边缘节点升级与版本变更历史数据访问层（DAL）。
package dal

import (
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// CreateEdgeNodeUpgradeHistory 记录升级或回滚历史流水。
func CreateEdgeNodeUpgradeHistory(h *model.EdgeNodeUpgradeHistory) error {
	return global.DB.Create(h).Error
}

// GetEdgeNodeUpgradeHistoryByID 租户作用域内按 ID 查询单条升级历史。
func GetEdgeNodeUpgradeHistoryByID(id, tenantID string) (*model.EdgeNodeUpgradeHistory, error) {
	var h model.EdgeNodeUpgradeHistory
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&h).Error
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// ListEdgeNodeUpgradeHistory 查询租户内某边缘节点的版本历史记录（按 created_at 倒序）。
func ListEdgeNodeUpgradeHistory(tenantID, nodeID string, limit int) ([]*model.EdgeNodeUpgradeHistory, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var list []*model.EdgeNodeUpgradeHistory
	err := global.DB.Where("tenant_id = ? AND node_id = ?", tenantID, nodeID).
		Order("created_at DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// UpdateEdgeNodeVersion 更新边缘节点当前版本（条件更新：必须处于 active 且租户一致）。
func UpdateEdgeNodeVersion(nodeID, tenantID, version string, updatedAt time.Time) (int64, error) {
	res := global.DB.Model(&model.EdgeNode{}).
		Where("id = ? AND tenant_id = ? AND status = ?", nodeID, tenantID, model.EdgeNodeStatusActive).
		Updates(map[string]interface{}{
			"version":    version,
			"updated_at": updatedAt,
		})
	return res.RowsAffected, res.Error
}
