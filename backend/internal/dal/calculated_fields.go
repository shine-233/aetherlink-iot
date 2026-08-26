// 文件用途：计算字段 DAL 层，封装 calculated_fields 表的 CRUD 操作。
// 核心逻辑：按设备配置查询启用的计算字段，支持增删改查和批量启用/禁用。
// 关键注意事项：output_key 在同一 device_config_id 内唯一；禁用字段不参与遥测计算。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// tenant-scope: parent-owned?2026-08-26 ?????
func GetCalculatedFieldsByConfigId(configId string) ([]*model.CalculatedField, error) {
	var fields []*model.CalculatedField
	err := global.DB.Where("device_config_id = ? AND enabled = true", configId).Find(&fields).Error
	return fields, err
}

func CreateCalculatedField(field *model.CalculatedField) error {
	return global.DB.Create(field).Error
}

func UpdateCalculatedField(id string, updates map[string]interface{}) error {
	result := global.DB.Model(&model.CalculatedField{}).Where("id = ?", id).Updates(updates)
	if result.RowsAffected == 0 {
		return global.DB.Error
	}
	return result.Error
}

func DeleteCalculatedField(id string) error {
	result := global.DB.Where("id = ?", id).Delete(&model.CalculatedField{})
	if result.RowsAffected == 0 {
		return global.DB.Error
	}
	return result.Error
}
