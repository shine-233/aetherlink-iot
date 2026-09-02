// 文件用途：规则链 DAL（ROADMAP B2）。
// 核心逻辑：租户内 CRUD 与按触发类型的启用链查询。
// 关键注意事项：所有列表查询必须携带非空 tenant_id（空租户守卫）；
//   图结构合法性（DAG 无环、节点类型已知）在 service 层校验。
package dal

import (
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateRuleChain 新建规则链。
func CreateRuleChain(chain *model.RuleChain) error {
	return global.DB.Create(chain).Error
}

// UpdateRuleChain 按主键更新非零字段，返回是否命中。
func UpdateRuleChain(chain *model.RuleChain) (bool, error) {
	result := global.DB.Model(chain).
		Where("id = ? AND tenant_id = ?", chain.ID, chain.TenantID).
		Updates(map[string]interface{}{
			"name":        chain.Name,
			"description": chain.Description,
			"enabled":     chain.Enabled,
			"graph":       chain.Graph,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// DeleteRuleChain 删除规则链。
func DeleteRuleChain(id, tenantID string) (bool, error) {
	result := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.RuleChain{})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// GetRuleChainByID 按 ID 读取（租户过滤）。
func GetRuleChainByID(id, tenantID string) (*model.RuleChain, error) {
	var chain model.RuleChain
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&chain).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &chain, nil
}

// ListRuleChainsByTenant 租户内分页列表；keyword 可选按名称模糊。
func ListRuleChainsByTenant(tenantID, keyword string, page, pageSize int) (int64, []model.RuleChain, error) {
	if strings.TrimSpace(tenantID) == "" {
		return 0, nil, fmt.Errorf("tenant id is required")
	}
	query := global.DB.Model(&model.RuleChain{}).Where("tenant_id = ?", tenantID)
	if kw := strings.TrimSpace(keyword); kw != "" {
		query = query.Where("name LIKE ?", fmt.Sprintf("%%%s%%", kw))
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	chains := make([]model.RuleChain, 0)
	err := query.Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&chains).Error
	return count, chains, err
}

// ListEnabledRuleChainGraphs 返回租户内启用链的原始 graph 文本（执行热路径用）。
func ListEnabledRuleChainGraphs(tenantID string) ([]string, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	var graphs []string
	err := global.DB.Model(&model.RuleChain{}).
		Where("tenant_id = ? AND enabled = ?", tenantID, true).
		Pluck("graph", &graphs).Error
	return graphs, err
}
