package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

func CreatePayloadSchema(record *model.PayloadSchemaRecord) error {
	return global.DB.Create(record).Error
}

func UpdatePayloadSchema(record *model.PayloadSchemaRecord) error {
	return global.DB.Save(record).Error
}

func DeletePayloadSchema(id, tenantID string) error {
	return global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.PayloadSchemaRecord{}).Error
}

func GetPayloadSchemaByID(id, tenantID string) (*model.PayloadSchemaRecord, error) {
	var record model.PayloadSchemaRecord
	err := global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&record).Error
	return &record, err
}

func ListPayloadSchemas(tenantID string, limit int) ([]*model.PayloadSchemaRecord, error) {
	var records []*model.PayloadSchemaRecord
	err := global.DB.
		Where("tenant_id = ?", tenantID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}
