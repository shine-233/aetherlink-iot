// 文件用途：封装告警邮件模板的作用域查询、默认模板切换和事务写入。
// 核心逻辑：所有管理操作都带 tenant_id；运行时先取租户默认模板，再回退系统默认模板。
// 关键注意事项：默认模板切换必须在事务内先清除同作用域旧默认，避免并发后出现多个有效默认。
package dal

import (
	"context"
	"errors"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// maxEmailTemplateListLimit 邮件模板列表单次返回的行数上限，防止无界查询。
const maxEmailTemplateListLimit = 500

// ListEmailTemplates 分页返回作用域内的告警邮件模板（ROADMAP C2 自上而下读）。
// scopes 语义：0→fail-closed 空结果、1→tenant_id =（与旧单租户等价）、>1→tenant_id IN。
// tenant-scope: scopes 由 service 层展开并校验（TENANT_ADMIN self∪子孙；平台默认模板由
// SYS_ADMIN 以 [""] 作用域管理）。
func ListEmailTemplates(scopes []string, page, pageSize int) (int64, []*model.EmailTemplate, error) {
	query := global.DB.WithContext(context.Background()).
		Model(&model.EmailTemplate{})
	switch len(scopes) {
	case 0:
		return 0, []*model.EmailTemplate{}, nil
	case 1:
		query = query.Where("tenant_id = ? AND purpose = ?", scopes[0], model.EmailTemplatePurposeAlarm)
	default:
		query = query.Where("tenant_id IN ? AND purpose = ?", scopes, model.EmailTemplatePurposeAlarm)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	if page > 0 && pageSize > 0 {
		query = query.Limit(clampEmailTemplateListLimit(pageSize)).Offset((page - 1) * pageSize)
	} else {
		// 未分页的调用同样强制封顶，避免一次性拉取全表。
		query = query.Limit(maxEmailTemplateListLimit)
	}
	list := make([]*model.EmailTemplate, 0)
	err := query.Order("is_default DESC, updated_at DESC, id ASC").Find(&list).Error
	return total, list, err
}

// clampEmailTemplateListLimit 把分页大小收敛到具名上限内。
func clampEmailTemplateListLimit(pageSize int) int {
	if pageSize > maxEmailTemplateListLimit {
		return maxEmailTemplateListLimit
	}
	return pageSize
}

func GetEmailTemplateByIDForScope(id, tenantID string) (*model.EmailTemplate, error) {
	var template model.EmailTemplate
	err := global.DB.WithContext(context.Background()).
		Where("id = ? AND tenant_id = ? AND purpose = ?", id, tenantID, model.EmailTemplatePurposeAlarm).
		First(&template).Error
	return &template, err
}

func SaveEmailTemplate(template *model.EmailTemplate) error {
	return global.DB.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		if template.IsDefault {
			if err := tx.Model(&model.EmailTemplate{}).
				Where("tenant_id = ? AND purpose = ? AND id <> ?", template.TenantID, template.Purpose, template.ID).
				Updates(map[string]interface{}{"is_default": false, "updated_at": template.UpdatedAt}).Error; err != nil {
				return err
			}
		}
		return tx.Save(template).Error
	})
}

func DeleteEmailTemplateForScope(id, tenantID string) error {
	result := global.DB.WithContext(context.Background()).
		Where("id = ? AND tenant_id = ? AND purpose = ?", id, tenantID, model.EmailTemplatePurposeAlarm).
		Delete(&model.EmailTemplate{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func SetDefaultEmailTemplateForScope(id, tenantID string, now time.Time) error {
	return global.DB.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		var template model.EmailTemplate
		if err := tx.Where("id = ? AND tenant_id = ? AND purpose = ?", id, tenantID, model.EmailTemplatePurposeAlarm).
			First(&template).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.EmailTemplate{}).
			Where("tenant_id = ? AND purpose = ?", tenantID, model.EmailTemplatePurposeAlarm).
			Updates(map[string]interface{}{"is_default": false, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.EmailTemplate{}).
			Where("id = ? AND tenant_id = ? AND purpose = ?", id, tenantID, model.EmailTemplatePurposeAlarm).
			Updates(map[string]interface{}{"enabled": true, "is_default": true, "updated_at": now}).Error
	})
}

func GetEffectiveAlarmEmailTemplate(tenantID string) (*model.EmailTemplate, error) {
	var template model.EmailTemplate
	if tenantID != "" {
		err := newEffectiveAlarmEmailTemplateQuery().Where("tenant_id = ?", tenantID).First(&template).Error
		if err == nil {
			return &template, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	// Build a fresh query for the fallback. Reusing the tenant query can retain its
	// tenant_id predicate in GORM and make the global lookup impossible.
	err := newEffectiveAlarmEmailTemplateQuery().Where("tenant_id = ?", "").First(&template).Error
	return &template, err
}

func newEffectiveAlarmEmailTemplateQuery() *gorm.DB {
	return global.DB.WithContext(context.Background()).
		Where("purpose = ? AND enabled = ? AND is_default = ?", model.EmailTemplatePurposeAlarm, true, true)
}
