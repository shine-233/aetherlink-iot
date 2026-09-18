// 文件用途：产品（Product）数据访问层（DAL）。
// 核心逻辑：提供 Product 的创建、租户按名检索、模糊前缀更名匹配（TB-15）、ID 检索、按租户更新、按租户删除及关联设备引用计数。
// 关键注意事项：所有查询与写操作必须严格遵循租户隔离（tenant_id），杜绝越权与跨租户信息泄漏。
package dal

import (
	"context"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
)

// CreateProduct 创建产品
func CreateProduct(p *model.Product) error {
	return query.Product.WithContext(context.Background()).Create(p)
}

// GetProductByNameAndTenant 查询指定租户下指定名称的产品（TB-15）。
func GetProductByNameAndTenant(tenantID, name string) (*model.Product, error) {
	var p model.Product
	err := global.DB.Where("tenant_id = ? AND name = ?", tenantID, name).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProductByIDAndTenant 根据 ID 和租户查询产品。
func GetProductByIDAndTenant(tenantID, id string) (*model.Product, error) {
	var p model.Product
	err := global.DB.Where("tenant_id = ? AND id = ?", tenantID, id).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProductNamesMatchingBase 查询指定租户下 baseName 或 baseName (N) 形式的产品名称列表（TB-15）。
func GetProductNamesMatchingBase(tenantID, baseName string) ([]string, error) {
	var names []string
	err := global.DB.Model(&model.Product{}).
		Where("tenant_id = ? AND (name = ? OR name LIKE ?)", tenantID, baseName, baseName+" (%)").
		Pluck("name", &names).Error
	return names, err
}

// UpdateProduct 更新指定租户下的产品信息。
func UpdateProduct(id, tenantID string, updates map[string]interface{}) error {
	return global.DB.Model(&model.Product{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates).Error
}

// DeleteProduct 删除指定租户下的产品。
func DeleteProduct(id, tenantID string) error {
	return global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.Product{}).Error
}

// CountDevicesByProductID 统计指定租户下关联该产品的设备数量。
// 显式携带 tenant_id：调用点虽已先验证产品归属，设备表再过滤一次是纵深防御，
// 也满足租户作用域审计对每条查询的显式标记要求。
func CountDevicesByProductID(tenantID, productID string) (int64, error) {
	var count int64
	err := global.DB.Model(&model.Device{}).Where("product_id = ? AND tenant_id = ?", productID, tenantID).Count(&count).Error
	return count, err
}

// GetProductListByPageWithDetail 分页多维度查询产品列表并关联设备配置名称。
func GetProductListByPageWithDetail(req *model.GetProductListByPageReq, tenantID string) (int64, []model.ProductList, error) {
	var list []model.ProductList
	db := global.DB.Table("products").
		Select("products.*, device_configs.name as device_config_name").
		Joins("LEFT JOIN device_configs ON device_configs.id = products.device_config_id").
		Where("products.tenant_id = ?", tenantID)

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		db = db.Where("products.name ILIKE ?", "%"+strings.TrimSpace(*req.Name)+"%")
	}
	if req.ProductModel != nil && strings.TrimSpace(*req.ProductModel) != "" {
		db = db.Where("products.product_model ILIKE ?", "%"+strings.TrimSpace(*req.ProductModel)+"%")
	}
	if req.ProductType != nil && strings.TrimSpace(*req.ProductType) != "" {
		db = db.Where("products.product_type = ?", strings.TrimSpace(*req.ProductType))
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	err := db.Order("products.created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	if err != nil {
		return 0, nil, err
	}
	return total, list, nil
}
