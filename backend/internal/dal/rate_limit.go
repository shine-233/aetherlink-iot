package dal

import (
	"errors"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetTenantRateLimit 查询指定租户或设备的某类限流规则。
func GetTenantRateLimit(tenantID, targetType, targetID, limitType string) (*model.TenantRateLimit, error) {
	if global.DB == nil {
		return nil, errors.New("database not initialized")
	}
	var record model.TenantRateLimit
	err := global.DB.Where("tenant_id = ? AND target_type = ? AND target_id = ? AND limit_type = ?",
		tenantID, targetType, targetID, limitType).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// ListTenantRateLimits 获取某租户下的所有限流自定义规则（超管传空获取全部）。
func ListTenantRateLimits(tenantID string) ([]model.TenantRateLimit, error) {
	if global.DB == nil {
		return nil, errors.New("database not initialized")
	}
	var list []model.TenantRateLimit
	query := global.DB.Model(&model.TenantRateLimit{})
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.Order("created_at DESC").Find(&list).Error
	return list, err
}

// SaveTenantRateLimit 幂等保存限流覆盖规则（存在则更新，不存在则插入）。
func SaveTenantRateLimit(record *model.TenantRateLimit) error {
	if global.DB == nil {
		return errors.New("database not initialized")
	}
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	record.UpdatedAt = time.Now()

	return global.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "target_type"},
			{Name: "target_id"},
			{Name: "limit_type"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"rate_limits", "enabled", "description", "updated_at"}),
	}).Create(record).Error
}

// DeleteTenantRateLimit 删除自定义限流规则。
func DeleteTenantRateLimit(tenantID, targetType, targetID, limitType string) error {
	if global.DB == nil {
		return errors.New("database not initialized")
	}
	query := global.DB.Where("target_type = ? AND target_id = ? AND limit_type = ?", targetType, targetID, limitType)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	result := query.Delete(&model.TenantRateLimit{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
