// 文件用途：AI 2.0（ROADMAP D7）模型中心数据访问层。
// 核心逻辑：租户内模型档案 CRUD；全部查询带 tenant_id 过滤保证租户隔离。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

func CreateAiModel(m *model.AiModel) error {
	return global.DB.Create(m).Error
}

func UpdateAiModel(m *model.AiModel) error {
	return global.DB.Save(m).Error
}

// DeleteAiModelInTenant 租户内删除模型档案，返回受影响行数（0=未命中）。
func DeleteAiModelInTenant(id, tenantID string) (int64, error) {
	res := global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.AiModel{})
	return res.RowsAffected, res.Error
}

// GetAiModelInTenant 按租户定位单条模型档案；未命中返回 gorm.ErrRecordNotFound。
func GetAiModelInTenant(id, tenantID string) (*model.AiModel, error) {
	var m model.AiModel
	err := global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&m).Error
	return &m, err
}

// ListAiModels 租户内列模型档案，可按用途/启用过滤。
func ListAiModels(tenantID, purpose string, limit int) ([]*model.AiModel, error) {
	var list []*model.AiModel
	q := global.DB.Where("tenant_id = ?", tenantID)
	if purpose != "" {
		q = q.Where("purpose = ?", purpose)
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&list).Error
	return list, err
}
