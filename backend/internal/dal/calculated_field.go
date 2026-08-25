// 文件用途：计算字段 DAL 层，封装 calculated_fields 表的租户隔离 CRUD 与引擎热路径查询。
// 核心逻辑：管理操作全部携带 tenant_id 作用域；引擎侧提供按模板拉取启用字段和设备→模板归属查询。
// 关键注意事项：更新/删除必须用 RowsAffected 守卫识别"不存在"，避免静默成功；列表查询强制 LIMIT 封顶。
// 重构建议：若后续模板删除需要级联停用计算字段，在这里补事务方法，而不是让 service 拼装多条语句。
package dal

import (
	"context"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// maxCalculatedFieldListLimit 计算字段列表单次返回的行数上限，防止无界查询。
const maxCalculatedFieldListLimit = 500

// CreateCalculatedField 新增一条计算字段。
func CreateCalculatedField(field *model.CalculatedField) error {
	return global.DB.WithContext(context.Background()).Create(field).Error
}

// GetCalculatedFieldForScope 按 id + 租户查询单条计算字段。
func GetCalculatedFieldForScope(id, tenantID string) (*model.CalculatedField, error) {
	var field model.CalculatedField
	err := global.DB.WithContext(context.Background()).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&field).Error
	if err != nil {
		return nil, err
	}
	return &field, nil
}

// UpdateCalculatedFieldForScope 按 id + 租户更新指定列；未命中返回 gorm.ErrRecordNotFound。
func UpdateCalculatedFieldForScope(id, tenantID string, updates map[string]interface{}) error {
	result := global.DB.WithContext(context.Background()).
		Model(&model.CalculatedField{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteCalculatedFieldForScope 按 id + 租户删除；未命中返回 gorm.ErrRecordNotFound。
func DeleteCalculatedFieldForScope(id, tenantID string) error {
	result := global.DB.WithContext(context.Background()).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.CalculatedField{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListCalculatedFieldsByPage 分页返回当前租户的计算字段，可按模板/名称过滤。
// pageSize 非正或超过上限时收敛到具名上限，避免负值取消 LIMIT 造成全表扫描。
func ListCalculatedFieldsByPage(tenantID string, req *model.CalculatedFieldListReq) (int64, []*model.CalculatedField, error) {
	query := global.DB.WithContext(context.Background()).
		Model(&model.CalculatedField{}).
		Where("tenant_id = ?", tenantID)
	if req != nil {
		if req.DeviceTemplateID != nil && *req.DeviceTemplateID != "" {
			query = query.Where("device_template_id = ?", *req.DeviceTemplateID)
		}
		if req.Name != nil && *req.Name != "" {
			query = query.Where("name LIKE ?", "%"+*req.Name+"%")
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	page, pageSize := 1, maxCalculatedFieldListLimit
	if req != nil && req.Page > 0 {
		page = req.Page
	}
	if req != nil && req.PageSize > 0 && req.PageSize <= maxCalculatedFieldListLimit {
		pageSize = req.PageSize
	}

	list := make([]*model.CalculatedField, 0)
	err := query.
		Order("updated_at DESC, id ASC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&list).Error
	return total, list, err
}

// ListEnabledCalculatedFieldsByTemplate 返回某设备模板下全部启用字段（引擎热路径）。
func ListEnabledCalculatedFieldsByTemplate(templateID string) ([]*model.CalculatedField, error) {
	fields := make([]*model.CalculatedField, 0)
	err := global.DB.WithContext(context.Background()).
		Where("device_template_id = ? AND enabled = TRUE", templateID).
		Order("id ASC").
		Limit(maxCalculatedFieldListLimit).
		Find(&fields).Error
	return fields, err
}

// CountDeviceTemplatesInTenant 返回当前租户下指定设备模板的行数（0 表示不存在或越权）。
func CountDeviceTemplatesInTenant(templateID, tenantID string) (int64, error) {
	var count int64
	err := global.DB.WithContext(context.Background()).
		Model(&model.DeviceTemplate{}).
		Where("id = ? AND tenant_id = ?", templateID, tenantID).
		Count(&count).Error
	return count, err
}

// GetDeviceTemplateIDByDeviceID 解析设备归属的设备模板 id；未绑定配置/模板时返回空串。
func GetDeviceTemplateIDByDeviceID(deviceID string) (string, error) {
	var row struct {
		TemplateID *string `gorm:"column:template_id"`
	}
	err := global.DB.WithContext(context.Background()).
		Table("devices d").
		Select("dc.device_template_id AS template_id").
		Joins("LEFT JOIN device_configs dc ON dc.id = d.device_config_id").
		Where("d.id = ?", deviceID).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "", err
	}
	if row.TemplateID == nil {
		return "", nil
	}
	return *row.TemplateID, nil
}
