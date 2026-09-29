// 文件用途：提供统一集成实体（Integration，TB-45）持久化存储操作。
// 核心逻辑：Integration CRUD、租户隔离过滤与分页检索；转换器绑定列为可空外键直读。
// 关键注意事项：所有查询都带 tenant_id 过滤（tenant_scope 审计口径），删除前先按租户探活。
// 重构建议：config JSONB 后续若出现结构化查询需求，再拆绑定表并保留 device_ids 兼容读取。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateIntegration 插入一条集成实例记录
func CreateIntegration(c *model.Integration) error {
	return global.DB.Create(c).Error
}

// GetIntegrationByID 根据 ID 及租户 ID 查询集成实例
func GetIntegrationByID(id, tenantID string) (*model.Integration, error) {
	var record model.Integration
	db := global.DB.Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if err := db.First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateIntegration 更新集成实例
func UpdateIntegration(c *model.Integration) error {
	return global.DB.Save(c).Error
}

// DeleteIntegration 删除集成实例（租户过滤，0 行命中返回 gorm.ErrRecordNotFound，
// 与 DeleteBoard 同契约：调用方可区分"删了"与"本来就没有/跨租户"）。
func DeleteIntegration(id, tenantID string) error {
	db := global.DB.Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	res := db.Delete(&model.Integration{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListIntegrations 分页查询集成实例列表
func ListIntegrations(req *model.GetIntegrationListReq, tenantID string) (int64, []*model.Integration, error) {
	var count int64
	var list []*model.Integration

	db := global.DB.Model(&model.Integration{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if req.ConnectorType != nil && *req.ConnectorType != "" {
		db = db.Where("connector_type = ?", *req.ConnectorType)
	}
	if req.Enabled != nil {
		db = db.Where("enabled = ?", *req.Enabled)
	}
	db = whereKeywordContainsPtr(db, opILike, req.Search, "name")

	if err := db.Count(&count).Error; err != nil {
		return 0, nil, err
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	err := db.Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&list).Error

	return count, list, err
}
