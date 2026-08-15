package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

func CreateFleetSavedFilter(filter *model.FleetSavedFilter) error {
	return global.DB.Create(filter).Error
}

func UpdateFleetSavedFilter(filter *model.FleetSavedFilter) error {
	return global.DB.Save(filter).Error
}

func DeleteFleetSavedFilter(id, tenantID, userID string) error {
	return global.DB.
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
		Delete(&model.FleetSavedFilter{}).Error
}

// GetFleetSavedFilterInTenant 只按租户定位记录，让 service 层能区分
// "记录不存在" 与 "记录属于同租户其他成员"，后者必须返回无权限而不是 404。
func GetFleetSavedFilterInTenant(id, tenantID string) (*model.FleetSavedFilter, error) {
	var filter model.FleetSavedFilter
	err := global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&filter).Error
	return &filter, err
}

// ListFleetSavedFiltersVisibleToUser 返回本人拥有的全部筛选器，加上同租户
// 其他成员显式共享（shared = true）的筛选器。跨租户记录永不可见。
func ListFleetSavedFiltersVisibleToUser(tenantID, userID string, limit int) ([]*model.FleetSavedFilter, error) {
	var filters []*model.FleetSavedFilter
	err := global.DB.
		Where("tenant_id = ? AND (user_id = ? OR shared = ?)", tenantID, userID, true).
		Order("updated_at DESC").
		Limit(limit).
		Find(&filters).Error
	return filters, err
}

// CountFleetSavedFiltersOwnedByUser 只统计本人拥有的行，保证配额不会被别人
// 共享进来的筛选器挤占。
func CountFleetSavedFiltersOwnedByUser(tenantID, userID string) (int64, error) {
	var total int64
	err := global.DB.
		Model(&model.FleetSavedFilter{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Count(&total).Error
	return total, err
}
