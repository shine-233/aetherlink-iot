package dal

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListPermissions 查询系统所有权限点，支持按模块筛选
// tenant-scope: system-wide dictionary, read-only
func ListPermissions(ctx context.Context, module string) ([]model.SysPermission, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	var list []model.SysPermission
	db := global.DB.WithContext(ctx).Table(model.TableNameSysPermission)
	if strings.TrimSpace(module) != "" {
		db = db.Where("module = ?", strings.TrimSpace(module))
	}
	err := db.Order("module ASC, code ASC").Find(&list).Error
	return list, err
}

// GetPermissionsByCodes 批量按 Code 获取权限定义
// tenant-scope: system-wide dictionary, read-only
func GetPermissionsByCodes(ctx context.Context, codes []string) ([]model.SysPermission, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if len(codes) == 0 {
		return []model.SysPermission{}, nil
	}
	var list []model.SysPermission
	err := global.DB.WithContext(ctx).Table(model.TableNameSysPermission).
		Where("code IN ?", codes).
		Find(&list).Error
	return list, err
}

// GetRolePermissions 查询某角色绑定的权限记录
// tenant-scope: filtered by role_id and tenant_id
func GetRolePermissions(ctx context.Context, roleID, tenantID string) ([]model.SysRolePermission, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	var list []model.SysRolePermission
	db := global.DB.WithContext(ctx).Table(model.TableNameSysRolePermission).Where("role_id = ?", roleID)
	if strings.TrimSpace(tenantID) != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	err := db.Order("created_at ASC").Find(&list).Error
	return list, err
}

// ReplaceRolePermissions 在同一事务中原子替换角色绑定的权限点
// tenant-scope: atomic replace by role_id and tenant_id
func ReplaceRolePermissions(ctx context.Context, roleID, tenantID string, codes []string) error {
	if global.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除既有
		delQuery := tx.Table(model.TableNameSysRolePermission).Where("role_id = ?", roleID)
		if strings.TrimSpace(tenantID) != "" {
			delQuery = delQuery.Where("tenant_id = ?", tenantID)
		}
		if err := delQuery.Delete(&model.SysRolePermission{}).Error; err != nil {
			return err
		}

		if len(codes) == 0 {
			return nil
		}

		now := time.Now().UTC()
		var toInsert []model.SysRolePermission
		for _, code := range codes {
			code = strings.TrimSpace(code)
			if code == "" {
				continue
			}
			toInsert = append(toInsert, model.SysRolePermission{
				ID:             uuid.New().String(),
				RoleID:         roleID,
				PermissionCode: code,
				TenantID:       tenantID,
				CreatedAt:      now,
			})
		}
		if len(toInsert) > 0 {
			if err := tx.Table(model.TableNameSysRolePermission).Create(&toInsert).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetUsersByRoleID 查询绑定了该角色的用户列表
// tenant-scope: filtered by tenant_id and role g-binding
func GetUsersByRoleID(ctx context.Context, roleID, tenantID string) ([]model.RoleUserSummary, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	// 用户通过 casbin_rule (ptype='g', v1=roleID) 关联角色
	var users []model.RoleUserSummary
	query := global.DB.WithContext(ctx).Table("users u").
		Select("u.id, u.email, u.name, u.phone_number, u.authority").
		Joins("JOIN casbin_rule cr ON cr.ptype = 'g' AND cr.v0 = u.id").
		Where("cr.v1 = ?", roleID)

	if strings.TrimSpace(tenantID) != "" {
		query = query.Where("u.tenant_id = ?", tenantID)
	}

	err := query.Order("u.created_at DESC").Find(&users).Error
	return users, err
}
