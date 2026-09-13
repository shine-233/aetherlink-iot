// 文件用途：P1.5 边缘节点注册表的数据访问。
// 核心逻辑：注册（插入/更新）、按 ID 全局读取（跨租户抢注判定）、租户内列表、心跳触碰。
// 关键注意事项：
//  1. **GetEdgeNodeByID 故意不带租户过滤**：同一 NodeID 被另一个租户注册是安全事件，
//     必须先全局看到这个 ID 才能拒绝；租户内读取走 GetEdgeNodeInTenant。
//  2. 心跳触碰用条件更新（WHERE id AND tenant_id AND status='active'）：
//     revoked 节点的心跳必须失败，而不是把 revoked 行的 last_seen 悄悄刷新。
//  3. 注册更新同样是条件更新并检查 RowsAffected：先查后写在并发注册下会互相覆盖。
package dal

import (
	"errors"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// GetEdgeNodeByID 按 ID 全局读取节点（**不带租户过滤**，供跨租户抢注判定）。
// 未命中返回 (nil, nil)。
func GetEdgeNodeByID(nodeID string) (*model.EdgeNode, error) {
	var row model.EdgeNode
	err := global.DB.Where("id = ?", nodeID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// GetEdgeNodeInTenant 读取租户内的节点；未命中返回 gorm.ErrRecordNotFound。
// 跨租户表现为未命中，而不是"存在但无权限"——后者等于告诉调用方节点在别的租户里。
func GetEdgeNodeInTenant(nodeID, tenantID string) (*model.EdgeNode, error) {
	var row model.EdgeNode
	err := global.DB.Where("id = ? AND tenant_id = ?", nodeID, tenantID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// InsertEdgeNode 写入一条新注册。
func InsertEdgeNode(node *model.EdgeNode) error {
	return global.DB.Create(node).Error
}

// UpdateEdgeNodeRegistration 更新既有节点的版本/能力/心跳（幂等重注册与心跳共用）。
// 条件更新 + RowsAffected 判定：0 行 = 节点不存在或已 revoked，调用方必须如实上报。
func UpdateEdgeNodeRegistration(nodeID, tenantID, version, capabilities string, seenAt time.Time) (int64, error) {
	res := global.DB.Model(&model.EdgeNode{}).
		Where("id = ? AND tenant_id = ? AND status = ?", nodeID, tenantID, model.EdgeNodeStatusActive).
		Updates(map[string]interface{}{
			"version":      version,
			"capabilities": capabilities,
			"last_seen_at": seenAt,
		})
	return res.RowsAffected, res.Error
}

// TouchEdgeNodeHeartbeat 只刷新心跳时间（版本/能力未变的心跳路径）。
// 同样以 status='active' 为条件：revoked 节点的心跳不得续命。
func TouchEdgeNodeHeartbeat(nodeID, tenantID string, seenAt time.Time) (int64, error) {
	res := global.DB.Model(&model.EdgeNode{}).
		Where("id = ? AND tenant_id = ? AND status = ?", nodeID, tenantID, model.EdgeNodeStatusActive).
		Update("last_seen_at", seenAt)
	return res.RowsAffected, res.Error
}

// ListEdgeNodesInTenant 列出租户内节点（按最后心跳倒序，运维视角先看最久没响的）。
func ListEdgeNodesInTenant(tenantID string, limit int) ([]*model.EdgeNode, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows := make([]*model.EdgeNode, 0, limit)
	err := global.DB.Where("tenant_id = ?", tenantID).
		Order("last_seen_at DESC NULLS LAST, id ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
