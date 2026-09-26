// 文件用途：用户组与组权限（TB-46 GPE v1，131.sql）的 DAL 层数据访问与组共享可见性映射。
// 核心逻辑：用户组 CRUD / 成员关联 / 组权限元素绑定的持久化操作，以及
//
//	「用户所属组→组权限→资源可见映射」的两个可见性集合查询（绑定集合、组内可见集合）。
//
// 关键注意事项：所有查询强制租户条件（fail-closed：scopes 为空返回空集不报错）；
//
//	element_code 多态引用无外键，资源存在性与同租户归属由 service 层绑定前校验；
//	组删除在同事务内显式清理成员与权限行（FK CASCADE 之外的方言兜底，sqlite 测试环境
//	默认不启用外键，依赖 CASCADE 会在测试夹具里留下悬空行）。
//
// 重构建议：组规模增大后，可见性映射可改为 SQL 端 NOT EXISTS 过滤直连列表查询，
//
//	避免"绑定集合−可见集合"两次往返（当前绑定行数量级很小，两次查询更可读）。
package dal

import (
	"context"
	"errors"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CreateUserGroup 新建用户组（名称同租户唯一性由 service 层预检 + 表 UNIQUE 兜底）。
func CreateUserGroup(g *model.UserGroup) error {
	return global.DB.Create(g).Error
}

// GetUserGroupForTenant 按 id+tenant 双条件读取用户组（租户隔离由 DAL 强制）。
func GetUserGroupForTenant(id, tenantID string) (*model.UserGroup, error) {
	var g model.UserGroup
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&g).Error
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// GetUserGroupByID 按 id 读取用户组（不带租户条件）。
// tenant-scope: caller-enforced（service 层 ensureUserGroupWriteAccess 校验组归属租户；
// SYS_ADMIN 全租户读取的唯一入口，TENANT_ADMIN 越权在 service 层返回 404）。
func GetUserGroupByID(id string) (*model.UserGroup, error) {
	var g model.UserGroup
	err := global.DB.Where("id = ?", id).First(&g).Error
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// GetUserGroupListByPage 分页查询用户组：scopes 语义与看板列表一致
// （nil=SYS_ADMIN 显式全租户视图；空切片/单值/多值=IN 过滤；空 scopes→fail-closed 空集）。
func GetUserGroupListByPage(req *model.GetUserGroupListReq, scopes []string) (int64, []*model.UserGroup, error) {
	if req == nil {
		req = &model.GetUserGroupListReq{}
	}
	q := global.DB.Model(&model.UserGroup{})
	// nil 作用域保留给 SYS_ADMIN 的显式全租户列表口径（与 boardListByScopes 同语义）；
	// 显式空切片按 fail-closed 处理：直接返回空集。
	if scopes != nil {
		if len(scopes) == 0 {
			return 0, []*model.UserGroup{}, nil
		}
		q = q.Where("tenant_id IN ?", scopes)
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		q = q.Where("name LIKE ?", "%"+EscapeLikePattern(strings.TrimSpace(*req.Name))+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		logrus.Error(err)
		return 0, nil, err
	}
	var list []*model.UserGroup
	err := applyListPagination(q, req.Page, req.PageSize).Order("created_at DESC").Find(&list).Error
	if err != nil {
		logrus.Error(err)
		return total, nil, err
	}
	return total, list, nil
}

// UpdateUserGroupForTenant 更新组名称/描述；id+tenant 双条件，0 行视为未命中。
func UpdateUserGroupForTenant(g *model.UserGroup) (bool, error) {
	updates := map[string]interface{}{"updated_at": time.Now().UTC()}
	if strings.TrimSpace(g.Name) != "" {
		updates["name"] = g.Name
	}
	if g.Description != nil {
		updates["description"] = *g.Description
	}
	res := global.DB.Model(&model.UserGroup{}).
		Where("id = ? AND tenant_id = ?", g.ID, g.TenantID).
		Updates(updates)
	if res.Error != nil {
		logrus.Error(res.Error)
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeleteUserGroupForTenant 删除用户组并在同事务内清理成员与权限绑定行。
// 迁移层的 FK CASCADE 是完整性兜底；这里显式删除保证 sqlite（外键默认关闭）行为一致。
func DeleteUserGroupForTenant(id, tenantID string) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&model.GroupPermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&model.UserGroupMember{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.UserGroup{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// GetUserGroupNameExists 检查同租户下同名组（排除自身 id，用于更新场景）。
func GetUserGroupNameExists(name, tenantID, excludeID string) (bool, error) {
	var count int64
	q := global.DB.Model(&model.UserGroup{}).Where("name = ? AND tenant_id = ?", name, tenantID)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

// ListUserGroupMembers 列出组成员（users 表内账号；customer 客户不在 users 表，天然不接入）。
func ListUserGroupMembers(groupID, tenantID string) ([]model.UserGroupMemberSummary, error) {
	var users []model.UserGroupMemberSummary
	err := global.DB.Table("users u").
		Select("u.id, u.email, u.name, u.authority").
		Joins("INNER JOIN "+model.TableNameUserGroupMember+" rgu ON rgu.user_id = u.id").
		Where("rgu.group_id = ? AND rgu.tenant_id = ?", groupID, tenantID).
		Order("u.created_at DESC").
		Find(&users).Error
	return users, err
}

// CountUsersByIDAndTenant 统计给定 ID 中真实存在且属于指定租户的 users 账号数
// （成员校验：customer 客户无 users 行，天然被拒）。
func CountUsersByIDAndTenant(userIDs []string, tenantID string) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}
	var count int64
	err := global.DB.Model(&model.User{}).
		Where("id IN ? AND tenant_id = ?", userIDs, tenantID).
		Count(&count).Error
	return count, err
}

// ReplaceUserGroupMembers 全量替换组成员（同事务先删后插，空列表=清空成员）。
func ReplaceUserGroupMembers(ctx context.Context, groupID, tenantID string, userIDs []string) error {
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.UserGroupMember{}).Error; err != nil {
			return err
		}
		if len(userIDs) == 0 {
			return nil
		}
		now := time.Now().UTC()
		rows := make([]model.UserGroupMember, 0, len(userIDs))
		seen := make(map[string]struct{}, len(userIDs))
		for _, uid := range userIDs {
			uid = strings.TrimSpace(uid)
			if uid == "" {
				continue
			}
			if _, dup := seen[uid]; dup {
				continue
			}
			seen[uid] = struct{}{}
			rows = append(rows, model.UserGroupMember{GroupID: groupID, UserID: uid, TenantID: tenantID, CreatedAt: &now})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

// ListGroupPermissions 列出组绑定的权限元素行。
func ListGroupPermissions(groupID, tenantID string) ([]model.GroupPermission, error) {
	var rows []model.GroupPermission
	err := global.DB.Where("group_id = ? AND tenant_id = ?", groupID, tenantID).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

// ReplaceGroupPermissions 全量替换组权限元素绑定（同事务先删后插，空列表=清空绑定）。
func ReplaceGroupPermissions(ctx context.Context, groupID, tenantID string, codes []string) error {
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.GroupPermission{}).Error; err != nil {
			return err
		}
		if len(codes) == 0 {
			return nil
		}
		now := time.Now().UTC()
		rows := make([]model.GroupPermission, 0, len(codes))
		seen := make(map[string]struct{}, len(codes))
		for _, code := range codes {
			code = strings.TrimSpace(code)
			if code == "" {
				continue
			}
			if _, dup := seen[code]; dup {
				continue
			}
			seen[code] = struct{}{}
			rows = append(rows, model.GroupPermission{
				ID:          uuid.New().String(),
				GroupID:     groupID,
				ElementCode: code,
				TenantID:    tenantID,
				CreatedAt:   &now,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

// CountBoardsByIDAndTenant 校验看板存在且属于指定租户（组权限绑定前的资源校验）。
func CountBoardsByIDAndTenant(boardID, tenantID string) (int64, error) {
	var count int64
	err := global.DB.Model(&model.Board{}).Where("id = ? AND tenant_id = ?", boardID, tenantID).Count(&count).Error
	return count, err
}

// CountAssetsByIDAndTenant 校验资产存在且属于指定租户（组权限绑定前的资源校验）。
func CountAssetsByIDAndTenant(assetID, tenantID string) (int64, error) {
	var count int64
	err := global.DB.Model(&model.Asset{}).Where("id = ? AND tenant_id = ?", assetID, tenantID).Count(&count).Error
	return count, err
}

// stripGroupElementPrefix 从元素码中剥离命名空间前缀，返回资源 ID。
// 前缀由调用方保证为 "kind:"（kind ∈ board/asset），非匹配行返回空串由调用方跳过。
func stripGroupElementPrefix(code, kind string) string {
	return strings.TrimPrefix(code, kind+":")
}

// GetGroupBoundResourceIDs 返回作用域内被任一用户组绑定的资源 ID 集合（组共享受限集合）。
// 这些资源默认对组外成员不可见（fail-closed）；scopes 为空时 fail-closed 返回空集。
// tenant-scope: caller-enforced（scopes 由 service 层展开并校验，元素行按租户 IN 过滤）。
func GetGroupBoundResourceIDs(scopes []string, kind string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{}, nil
	}
	var codes []string
	err := global.DB.Model(&model.GroupPermission{}).
		Where("tenant_id IN ?", scopes).
		Where("element_code LIKE ?", kind+":%").
		Distinct().
		Pluck("element_code", &codes).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	ids := make([]string, 0, len(codes))
	for _, code := range codes {
		if id := stripGroupElementPrefix(code, kind); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// GetGroupSharedResourceIDs 返回用户经「所属组→组权限→资源可见映射」可见的资源 ID 集合。
// 用户不属于任何组 / scopes 为空 / userID 为空时 fail-closed 返回空集（默认不可见）。
// 租户边界：组成员行与权限元素行均按 scopes IN 过滤，组共享不放大跨租户可见性。
// tenant-scope: caller-enforced（scopes 由 service 层展开并校验）。
func GetGroupSharedResourceIDs(scopes []string, userID, kind string) ([]string, error) {
	if len(scopes) == 0 || strings.TrimSpace(userID) == "" {
		return []string{}, nil
	}
	var codes []string
	err := global.DB.Table(model.TableNameGroupPermission+" gp").
		Select("DISTINCT gp.element_code").
		Joins("INNER JOIN "+model.TableNameUserGroupMember+" rgu ON rgu.group_id = gp.group_id").
		Where("rgu.user_id = ?", userID).
		Where("rgu.tenant_id IN ?", scopes).
		Where("gp.tenant_id IN ?", scopes).
		Where("gp.element_code LIKE ?", kind+":%").
		Pluck("element_code", &codes).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []string{}, nil
		}
		logrus.Error(err)
		return nil, err
	}
	ids := make([]string, 0, len(codes))
	for _, code := range codes {
		if id := stripGroupElementPrefix(code, kind); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
