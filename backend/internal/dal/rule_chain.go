// 文件用途：规则链 DAL 层，封装 rule_chains/nodes/edges 三表的 CRUD 操作。
// 核心逻辑：提供规则链的增删改查、节点批量保存、边批量保存和按租户过滤查询。
// 关键注意事项：删除规则链时通过外键级联删除 nodes 和 edges；所有查询必须带租户隔离。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateRuleChain 创建规则链（含节点和边）。
func CreateRuleChain(chain *model.RuleChain, nodes []*model.RuleChainNode, edges []*model.RuleChainEdge) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(chain).Error; err != nil {
			return err
		}
		for _, n := range nodes {
			if err := tx.Create(n).Error; err != nil {
				return err
			}
		}
		for _, e := range edges {
			if err := tx.Create(e).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
