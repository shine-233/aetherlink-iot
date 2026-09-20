// 文件用途：看板项目分组（native-board-provider 项目增删改）的数据访问。
// 核心逻辑：项目 CRUD、看板归属（一板一项目）、按看板反查项目、按项目过滤看板 ID。
// 关键注意事项：
//  1. 所有查询强制 tenant_id；跨租户表现为未命中而不是"存在但无权限"。
//  2. 归属变更用条件更新语义（先删后插在同一事务语义里不必要——
//     member 表主键 (project_id, board_id) 冲突即失败，把"已属于本项目"原样暴露）。
//  3. 换项目 = RemoveBoardFromProject + AddBoardToProject 由服务层编排；
//     board_id 唯一索引保证并发换项目时只有一个赢家，不靠应用层先查。
package dal

import (
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// CreateBoardProject 写入一个项目。租户内同名由唯一索引拒绝，错误原样返回。
func CreateBoardProject(project *model.BoardProject) error {
	return global.DB.Create(project).Error
}

// GetBoardProjectInTenant 读取租户内项目；未命中返回 gorm.ErrRecordNotFound。
func GetBoardProjectInTenant(id, tenantID string) (*model.BoardProject, error) {
	var row model.BoardProject
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListBoardProjectsInTenant 列出租户内项目；boardID 非空时只返回包含该看板的项目。
func ListBoardProjectsInTenant(tenantID, boardID string, limit int) ([]*model.BoardProject, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows := make([]*model.BoardProject, 0, limit)
	q := global.DB.Where("tenant_id = ?", tenantID)
	if boardID != "" {
		q = q.Where("id IN (SELECT project_id FROM board_project_members WHERE tenant_id = ? AND board_id = ?)",
			tenantID, boardID)
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// UpdateBoardProject 更新项目可变字段；RowsAffected=0 表示不存在。
func UpdateBoardProject(id, tenantID, name string, description *string) (int64, error) {
	updates := map[string]interface{}{"name": name}
	if description != nil {
		updates["description"] = *description
	} else {
		updates["description"] = nil
	}
	res := global.DB.Model(&model.BoardProject{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// DeleteBoardProject 删除项目及其归属关系（不是删除看板）。
func DeleteBoardProject(id, tenantID string) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND tenant_id = ?", id, tenantID).
			Delete(&model.BoardProject{}).Error; err != nil {
			return err
		}
		return tx.Where("project_id = ? AND tenant_id = ?", id, tenantID).
			Delete(&model.BoardProjectMember{}).Error
	})
}

// AddBoardToProject 把看板加入项目。看板已在（任何）项目时由 board_id 唯一索引拒绝。
// 校验看板确实在租户内是服务层职责（这里只管归属事实本身）。
func AddBoardToProject(projectID, boardID, tenantID string) error {
	member := &model.BoardProjectMember{ProjectID: projectID, BoardID: boardID, TenantID: tenantID}
	return global.DB.Create(member).Error
}

// RemoveBoardFromProject 把看板移出项目（仅当该项目属于本租户）。
// RowsAffected=0 表示本来就不在这个项目里，调用方自行决定是否视为幂等成功。
func RemoveBoardFromProject(projectID, boardID, tenantID string) (int64, error) {
	res := global.DB.
		Where("project_id = ? AND board_id = ? AND tenant_id = ?", projectID, boardID, tenantID).
		Delete(&model.BoardProjectMember{})
	return res.RowsAffected, res.Error
}

// GetBoardProjectMembership 反查一块看板当前所属的项目（不在任何项目返回 nil, nil）。
func GetBoardProjectMembership(boardID, tenantID string) (*model.BoardProject, error) {
	var member model.BoardProjectMember
	err := global.DB.Where("board_id = ? AND tenant_id = ?", boardID, tenantID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return GetBoardProjectInTenant(member.ProjectID, tenantID)
}

// ListBoardIDsByProject 项目内全部看板 ID（供看板列表按项目过滤）。
func ListBoardIDsByProject(projectID, tenantID string) ([]string, error) {
	rows := make([]model.BoardProjectMember, 0, 64)
	err := global.DB.Where("project_id = ? AND tenant_id = ?", projectID, tenantID).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.BoardID)
	}
	return ids, nil
}

// CountBoardProjectMembers 项目内看板数（删除前的提示与校验用）。
func CountBoardProjectMembers(projectID, tenantID string) (int64, error) {
	var count int64
	err := global.DB.Model(&model.BoardProjectMember{}).
		Where("project_id = ? AND tenant_id = ?", projectID, tenantID).
		Count(&count).Error
	return count, err
}

// ListAllProjectMemberBoardIDs 全部已归属看板的 ID（内置项目过滤用）。
// tenantID 非空时限定租户（TENANT 作用域）；空表示 SYS_ADMIN 全量视图。
func ListAllProjectMemberBoardIDs(tenantID string) ([]string, error) {
	q := global.DB.Model(&model.BoardProjectMember{})
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	rows := make([]model.BoardProjectMember, 0, 128)
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.BoardID)
	}
	return ids, nil
}

// RemoveAllBoardProjectMemberships 解除一块看板的全部项目归属（删除看板前的清理）。
func RemoveAllBoardProjectMemberships(boardID, tenantID string) (int64, error) {
	res := global.DB.
		Where("board_id = ? AND tenant_id = ?", boardID, tenantID).
		Delete(&model.BoardProjectMember{})
	return res.RowsAffected, res.Error
}
