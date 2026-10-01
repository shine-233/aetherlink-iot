// 文件用途：提供数据转换器持久化存储操作（ThingsBoard Data Converter 载荷解析）。
// 核心逻辑：数据转换器 CRUD、租户隔离过滤与分页检索。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
)

// CreateDataConverter 插入一条数据转换器记录
func CreateDataConverter(c *model.DataConverter) error {
	return global.DB.Create(c).Error
}

// GetDataConverterByID 根据 ID 及租户 ID 查询
func GetDataConverterByID(id, tenantID string) (*model.DataConverter, error) {
	var record model.DataConverter
	db := global.DB.Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if err := db.First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateDataConverter 更新转换器
func UpdateDataConverter(c *model.DataConverter) error {
	return global.DB.Save(c).Error
}

// DeleteDataConverter 删除转换器
func DeleteDataConverter(id, tenantID string) error {
	db := global.DB.Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	return db.Delete(&model.DataConverter{}).Error
}

// ListDataConverters 分页查询数据转换器列表
func ListDataConverters(req *model.GetDataConverterListReq, tenantID string) (int64, []*model.DataConverter, error) {
	var list []*model.DataConverter

	db := global.DB.Model(&model.DataConverter{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if req.Type != nil && *req.Type != "" {
		db = db.Where("type = ?", *req.Type)
	}
	db = whereKeywordContainsPtr(db, opILike, req.Search, "name", "description")

	count, err := countAndFindLegacyPage(db, "created_at DESC", req.Page, req.PageSize, &list)
	return count, list, err
}
