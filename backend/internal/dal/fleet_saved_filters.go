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

// ListFleetSavedFiltersVisibleToUser 返回作用域内本人拥有的全部筛选器，加上
// 同作用域其他成员显式共享（shared = true）的筛选器（ROADMAP C2 自上而下读）。
// scopes 语义：0→fail-closed 空结果、1→tenant_id =（与旧单租户等价）、>1→tenant_id IN；
// user 维度恒为 user_id = ? OR shared = true，跨成员私有行即使落在作用域内也不可见。
// tenant-scope: scopes 由 service 层展开并校验（TENANT_ADMIN self∪子孙；调用方为
// SYS_ADMIN 且 tenant 为空时由 service 映射为 [""] 保持平台空租户旧行为）。
func ListFleetSavedFiltersVisibleToUser(scopes []string, userID string, limit int) ([]*model.FleetSavedFilter, error) {
	var filters []*model.FleetSavedFilter
	query := global.DB.Model(&model.FleetSavedFilter{})
	switch len(scopes) {
	case 0:
		return []*model.FleetSavedFilter{}, nil
	case 1:
		query = query.Where("tenant_id = ? AND (user_id = ? OR shared = ?)", scopes[0], userID, true)
	default:
		query = query.Where("tenant_id IN ? AND (user_id = ? OR shared = ?)", scopes, userID, true)
	}
	err := query.
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
