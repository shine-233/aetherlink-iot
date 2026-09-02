// 文件用途：资产 DAL 层（ROADMAP C2），封装 assets 表 CRUD。
// 核心逻辑：所有查询强制携带 tenantScopes（由 Service 层按 hierarchy.Scope 展开，
//
//	即 self∪子孙（自上而下）），杜绝跨租户读；删除前校验无子节点。
//
// 关键注意事项：时间/字符串语义与 device_shadow 一致，仅用参数化值，保证 PG/sqlite 同行为。
package dal

import (
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateAsset 新建资产（parent_id/环校验由 Service 层负责）。
func CreateAsset(a *model.Asset) error {
	return global.DB.Create(a).Error
}

// GetAsset 按 id 在指定租户作用域内读取资产。
func GetAsset(id string, scopes []string) (*model.Asset, error) {
	var a model.Asset
	err := global.DB.Where("id = ? AND tenant_id IN ?", id, scopes).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListAssetNodes 返回租户作用域内全部资产节点（用于构建树/校验环，量级受分页约束）。
func ListAssetNodes(scopes []string) ([]*model.Asset, error) {
	var list []*model.Asset
	err := global.DB.Where("tenant_id IN ?", scopes).Order("created_at ASC").Find(&list).Error
	return list, err
}

// ListAssetsByPage 分页查询；parentID 传 "" 时只查根；keyword 非空时对名称做模糊匹配。
// 返回 (列表, 总数, 错误)。
func ListAssetsByPage(scopes []string, parentID, keyword string, page, pageSize int) ([]*model.Asset, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := global.DB.Model(&model.Asset{}).Where("tenant_id IN ?", scopes)
	kw := strings.TrimSpace(keyword)
	if parentID != "" {
		q = q.Where("parent_id = ?", parentID)
	} else if kw == "" {
		// 未给父节点且未给关键词时，默认列出根节点。
		q = q.Where("parent_id = ?", "")
	}
	if kw != "" {
		q = q.Where("name LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.Asset
	if err := q.Order("created_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// CountAssetChildren 统计指定资产直接子节点数（删除守卫）。
func CountAssetChildren(id string, scopes []string) (int64, error) {
	var n int64
	err := global.DB.Model(&model.Asset{}).
		Where("parent_id = ? AND tenant_id IN ?", id, scopes).
		Count(&n).Error
	return n, err
}

// UpdateAsset 更新资产（名称/类型/meta/父节点）；仅允许作用域内记录。
// 返回是否命中（0 行视为未找到或无权限）。
func UpdateAsset(a *model.Asset) (bool, error) {
	now := time.Now().UTC()
	res := global.DB.Model(&model.Asset{}).
		Where("id = ? AND tenant_id = ?", a.ID, a.TenantID).
		Updates(map[string]interface{}{
			"parent_id":  a.ParentID,
			"name":       a.Name,
			"asset_type": a.AssetType,
			"meta":       a.Meta,
			"updated_at": &now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeleteAsset 删除资产（调用方需先保证无子节点）。
func DeleteAsset(id, tenantID string) error {
	res := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.Asset{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
