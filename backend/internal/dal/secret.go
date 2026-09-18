// 文件用途：通用 Secrets Storage 数据访问层（DAL）。
// 核心逻辑：封装 sys_secrets 表的 CRUD、租户隔离、分页检索与唯一性检测。
// 关键注意事项：所有查询和修改必须严格遵循租户隔离原则（多租户安全边界）。
package dal

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// CreateSecret 新增密钥记录。
func CreateSecret(ctx context.Context, secret *model.SysSecret) error {
	if secret == nil {
		return errors.New("secret is nil")
	}
	if secret.CreatedAt.IsZero() {
		secret.CreatedAt = time.Now()
	}
	if secret.UpdatedAt.IsZero() {
		secret.UpdatedAt = time.Now()
	}
	return global.DB.WithContext(ctx).Table("sys_secrets").Create(secret).Error
}

// GetSecretByID 根据 ID 获取密钥详情（附带租户隔离）。
func GetSecretByID(ctx context.Context, tenantID, id string) (*model.SysSecret, error) {
	var s model.SysSecret
	db := global.DB.WithContext(ctx).Table("sys_secrets").Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if err := db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSecretByKey 根据租户 ID 与 key 获取密钥记录。
func GetSecretByKey(ctx context.Context, tenantID, key string) (*model.SysSecret, error) {
	var s model.SysSecret
	db := global.DB.WithContext(ctx).Table("sys_secrets").Where("tenant_id = ? AND key = ?", tenantID, key)
	if err := db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSecrets 分页与条件查询密钥列表。
func ListSecrets(ctx context.Context, tenantID string, req *model.SecretListReq) ([]model.SysSecret, int64, error) {
	var (
		list  []model.SysSecret
		total int64
	)
	db := global.DB.WithContext(ctx).Table("sys_secrets")
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if req.SecretType != "" {
		db = db.Where("secret_type = ?", req.SecretType)
	}
	if q := strings.TrimSpace(req.Query); q != "" {
		pattern := "%" + q + "%"
		db = db.Where("key ILIKE ? OR name ILIKE ?", pattern, pattern)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	err := db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateSecret 更新密钥记录。
func UpdateSecret(ctx context.Context, secret *model.SysSecret) error {
	if secret == nil {
		return errors.New("secret is nil")
	}
	secret.UpdatedAt = time.Now()
	updates := map[string]interface{}{
		"name":            secret.Name,
		"secret_type":     secret.SecretType,
		"description":     secret.Description,
		"encrypted_value": secret.EncryptedValue,
		"mask_preview":    secret.MaskPreview,
		"key_id":          secret.KeyID,
		"updated_at":      secret.UpdatedAt,
	}
	return global.DB.WithContext(ctx).Table("sys_secrets").Where("id = ?", secret.ID).Updates(updates).Error
}

// DeleteSecret 删除密钥（严格校验租户边界）。
func DeleteSecret(ctx context.Context, tenantID, id string) error {
	var s model.SysSecret
	db := global.DB.WithContext(ctx).Table("sys_secrets").Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if err := db.First(&s).Error; err != nil {
		return err
	}
	return global.DB.WithContext(ctx).Table("sys_secrets").Where("id = ?", id).Delete(&model.SysSecret{}).Error
}

// CheckKeyExists 检查特定租户下是否存在同名 key（排除指定 ID）。
func CheckKeyExists(ctx context.Context, tenantID, key, excludeID string) (bool, error) {
	var count int64
	db := global.DB.WithContext(ctx).Table("sys_secrets").Where("tenant_id = ? AND key = ?", tenantID, key)
	if excludeID != "" {
		db = db.Where("id != ?", excludeID)
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
