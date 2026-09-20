// 文件用途：P1.5 边缘节点证书数据访问层（DAL）。
// 边界说明：所有查询必须携带 tenant_id，保障多租户强隔离。
package dal

import (
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// CreateEdgeNodeCertificate 保存边缘节点证书。
func CreateEdgeNodeCertificate(cert *model.EdgeNodeCertificate) error {
	return global.DB.Create(cert).Error
}

// GetActiveEdgeNodeCertificate 获取节点当前生效的证书（按 issued_at 倒序取最新一条 active）。
func GetActiveEdgeNodeCertificate(tenantID, nodeID string) (*model.EdgeNodeCertificate, error) {
	var cert model.EdgeNodeCertificate
	err := global.DB.Where("tenant_id = ? AND node_id = ? AND status = ?", tenantID, nodeID, "active").
		Order("issued_at DESC").
		First(&cert).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// RevokeEdgeNodeCertificates 将该节点的所有 active 证书置为 revoked。
func RevokeEdgeNodeCertificates(tenantID, nodeID string, revokedAt time.Time, reason string) (int64, error) {
	updates := map[string]interface{}{
		"status":        "revoked",
		"revoked_at":    revokedAt,
		"updated_at":    revokedAt,
		"revoke_reason": reason,
	}
	res := global.DB.Model(&model.EdgeNodeCertificate{}).
		Where("tenant_id = ? AND node_id = ? AND status = ?", tenantID, nodeID, "active").
		Updates(updates)
	return res.RowsAffected, res.Error
}

// ListEdgeNodeCertificates 获取节点的所有历史证书记录。
func ListEdgeNodeCertificates(tenantID, nodeID string) ([]*model.EdgeNodeCertificate, error) {
	var certs []*model.EdgeNodeCertificate
	err := global.DB.Where("tenant_id = ? AND node_id = ?", tenantID, nodeID).
		Order("issued_at DESC").
		Find(&certs).Error
	return certs, err
}
