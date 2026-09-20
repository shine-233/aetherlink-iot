// 文件用途：规则链版本数据访问层（ROADMAP P1.2）。
// 核心逻辑：租户内版本写入、列表、按版本号定位与已发布版本定位。
// 关键注意事项：
//  1. 每一个查询都强制带 tenant_id。跨租户数据对本层而言是"不存在"，
//     不是"存在但没权限"，因此一律表现为未命中。
//  2. 版本冲突（(chain_id, version) 重复、同一链第二个 published）由数据库约束拒绝，
//     本层不做"先查再插"——那在高并发下会漏判。
//  3. 发布是带状态的更新：只有 status=draft 的行能被更新为 published，
//     用 WHERE 条件把状态机条件交给数据库，避免并发下重复发布。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// CreateRuleChainVersion 写入一个新版本（通常为 draft）。冲突交给数据库唯一约束。
func CreateRuleChainVersion(version *model.RuleChainVersion) error {
	return global.DB.Model(&model.RuleChainVersion{}).Create(map[string]interface{}{
		"id":               version.ID,
		"tenant_id":        version.TenantID,
		"chain_id":         version.ChainID,
		"version":          version.Version,
		"status":           version.Status,
		"graph_hash":       version.GraphHash,
		"graph":            version.Graph,
		"rolled_back_from": version.RolledBackFrom,
		"created_at":       version.CreatedAt,
		"published_at":     version.PublishedAt,
	}).Error
}

// ListRuleChainVersions 列出某条链在当前租户下的全部版本，按版本号升序。
func ListRuleChainVersions(tenantID, chainID string) ([]*model.RuleChainVersion, error) {
	rows := make([]*model.RuleChainVersion, 0)
	err := global.DB.Model(&model.RuleChainVersion{}).
		Where("tenant_id = ? AND chain_id = ?", tenantID, chainID).
		Order("version ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetRuleChainVersion 按版本号定位某条链的单个版本。
func GetRuleChainVersion(tenantID, chainID string, version int) (*model.RuleChainVersion, error) {
	row := &model.RuleChainVersion{}
	err := global.DB.Model(&model.RuleChainVersion{}).
		Where("tenant_id = ? AND chain_id = ? AND version = ?", tenantID, chainID, version).
		First(row).Error
	if err != nil {
		return nil, err
	}
	return row, nil
}

// PublishRuleChainVersion 把 draft 置为 published。
// 条件里带 status='draft'，因此并发重复发布时第二个请求影响行数为 0，
// 由调用方据此判定为"已发布"而不是静默成功。
func PublishRuleChainVersion(tenantID, chainID string, version int, publishedAt interface{}) (int64, error) {
	result := global.DB.Model(&model.RuleChainVersion{}).
		Where("tenant_id = ? AND chain_id = ? AND version = ? AND status = ?",
			tenantID, chainID, version, "draft").
		Updates(map[string]interface{}{
			"status":       "published",
			"published_at": publishedAt,
		})
	return result.RowsAffected, result.Error
}
