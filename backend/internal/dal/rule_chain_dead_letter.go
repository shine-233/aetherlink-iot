package dal

import (
	"context"
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// SaveRuleChainDeadLetter 插入一条死信记录。
func SaveRuleChainDeadLetter(ctx context.Context, row *model.RuleChainDeadLetter) error {
	if row == nil || global.DB == nil {
		return nil
	}
	return global.DB.WithContext(ctx).Table(model.TableNameRuleChainDeadLetter).Create(row).Error
}

// ListRuleChainDeadLetters 查询死信列表，支持租户、规则链、批次过滤。
func ListRuleChainDeadLetters(ctx context.Context, tenantID, chainID, execID string, limit, offset int) ([]model.RuleChainDeadLetter, int64, error) {
	rows := make([]model.RuleChainDeadLetter, 0)
	if global.DB == nil {
		return rows, 0, nil
	}
	query := global.DB.WithContext(ctx).
		Table(model.TableNameRuleChainDeadLetter).
		Where("tenant_id = ?", tenantID)

	if strings.TrimSpace(chainID) != "" {
		query = query.Where("chain_id = ?", strings.TrimSpace(chainID))
	}
	if strings.TrimSpace(execID) != "" {
		query = query.Where("exec_id = ?", strings.TrimSpace(execID))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}
