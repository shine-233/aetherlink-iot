// 文件用途：P1.6 模板升级/回滚的数据访问——历史记录与"最新版本行"定位。
// 关键注意事项：
//  1. GetLatestDeviceTemplateByName 取"最新"以 created_at DESC 为准（与列表排序语义一致），
//     版本号不参与排序——版本是点分字符串，数据库排序会 1.10 < 1.2 排错。
//  2. 历史查询强制 tenant_id；跨租户表现为未命中。
package dal

import (
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// GetLatestDeviceTemplateByName 取租户内该名称的最新模板行；不存在返回 (nil, nil)。
func GetLatestDeviceTemplateByName(tenantID, name string) (*model.DeviceTemplate, error) {
	var row model.DeviceTemplate
	err := global.DB.
		Where("tenant_id = ? AND name = ?", tenantID, name).
		Order("created_at DESC").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// InsertTemplateUpgradeHistory 落一条升级历史。
func InsertTemplateUpgradeHistory(history *model.TemplateUpgradeHistory) error {
	return global.DB.Create(history).Error
}

// GetTemplateUpgradeHistoryInTenant 读取租户内一条升级历史；未命中返回 ErrRecordNotFound。
func GetTemplateUpgradeHistoryInTenant(id, tenantID string) (*model.TemplateUpgradeHistory, error) {
	var row model.TemplateUpgradeHistory
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListTemplateUpgradeHistoryInTenant 列出某模板的升级历史（新→旧），供前端展示回滚点。
func ListTemplateUpgradeHistoryInTenant(tenantID, templateName string, limit int) ([]*model.TemplateUpgradeHistory, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows := make([]*model.TemplateUpgradeHistory, 0, limit)
	q := global.DB.Where("tenant_id = ?", tenantID)
	if templateName != "" {
		q = q.Where("template_name = ?", templateName)
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
