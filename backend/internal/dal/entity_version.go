// 文件用途：实体版本控制 DAL 层，封装 entity_versions 表的租户隔离读写。
// 核心逻辑：快照按 (tenant_id, entity_type, entity_id) 定位实体，版本号在同一实体内递增；
// 提供创建、按 id 查询、分页列表、当前最大版本号与删除能力。
// 关键注意事项：所有查询强制携带 tenant_id 作用域；版本号读取用于生成下一个序号，
// 唯一索引 (tenant_id, entity_type, entity_id, version_number) 是并发下不重号的最终保障。
// 重构建议：若需支持版本清理策略（保留最近 N 版），在这里补按实体裁剪的事务方法。
package dal

import (
	"context"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// maxEntityVersionListLimit 版本列表单次返回的行数上限，防止无界查询。
const maxEntityVersionListLimit = 500

// CreateEntityVersion 新增一条实体快照版本。
func CreateEntityVersion(version *model.EntityVersion) error {
	return global.DB.WithContext(context.Background()).Create(version).Error
}

// GetEntityVersionForScope 按 id + 租户查询单个版本。
func GetEntityVersionForScope(id, tenantID string) (*model.EntityVersion, error) {
	var version model.EntityVersion
	err := global.DB.WithContext(context.Background()).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&version).Error
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// GetEntityVersionsForScope 按实体分页查询版本历史，按版本号倒序（最新在前）。
func GetEntityVersionsForScope(tenantID, entityType, entityID string, page, pageSize int) ([]*model.EntityVersion, int64, error) {
	var list []*model.EntityVersion
	var total int64

	base := global.DB.WithContext(context.Background()).
		Model(&model.EntityVersion{}).
		Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenantID, entityType, entityID)

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if pageSize <= 0 || pageSize > maxEntityVersionListLimit {
		pageSize = maxEntityVersionListLimit
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	if err := base.Order("version_number DESC").
		Limit(pageSize).Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetMaxEntityVersionNumber 返回该实体当前最大版本号；尚无版本时返回 0。
func GetMaxEntityVersionNumber(tenantID, entityType, entityID string) (int, error) {
	var maxNumber *int
	err := global.DB.WithContext(context.Background()).
		Model(&model.EntityVersion{}).
		Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenantID, entityType, entityID).
		Select("COALESCE(MAX(version_number), 0)").
		Scan(&maxNumber).Error
	if err != nil {
		return 0, err
	}
	if maxNumber == nil {
		return 0, nil
	}
	return *maxNumber, nil
}

// DeleteEntityVersionForScope 按 id + 租户删除版本；未命中返回 gorm.ErrRecordNotFound。
func DeleteEntityVersionForScope(id, tenantID string) error {
	result := global.DB.WithContext(context.Background()).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.EntityVersion{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
