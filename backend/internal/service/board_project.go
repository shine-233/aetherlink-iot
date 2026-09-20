// 文件用途：看板项目分组的服务层——native-board-provider 项目增删改的业务边界。
// 核心逻辑：租户内项目 CRUD + 看板归属管理（一板一项目）。
// 关键注意事项：
//   - 归属变更必须校验"项目在看板都在本租户内"：归属表以租户为作用域，
//     跨租户组合出来的归属等于把别人的看板挂进自己的项目列表。
//   - "看板已在别的项目"由 member 表 (tenant_id, board_id) 唯一索引拒绝，
//     服务层负责把该拒绝转成可读的参数错误——并发换项目只有一个赢家。
//   - 删除项目只解除分组，不动看板本身；删除前如实报告成员数。
package service

import (
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BoardProjectService 看板项目分组服务。
type BoardProjectService struct{}

// CreateProject 创建项目（租户内名称唯一）。
func (*BoardProjectService) CreateProject(req model.CreateBoardProjectReq, claims *utils.UserClaims) (*model.BoardProject, error) {
	if err := ensureTenantScopedWriteClaims(claims, "create board project"); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "project name is required")
	}
	now := time.Now()
	project := &model.BoardProject{
		ID:          uuid.New().String(),
		TenantID:    claims.TenantID,
		Name:        name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := dal.CreateBoardProject(project); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "board project name already exists in tenant")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return project, nil
}

// ListProjects 列出项目；board_id 非空时反查包含该看板的项目。
func (*BoardProjectService) ListProjects(req model.BoardProjectListReq, claims *utils.UserClaims) ([]*model.BoardProject, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	boardID := strings.TrimSpace(req.BoardID)
	if boardID != "" {
		// 反查前确认看板在租户内：不存在的看板ID不该返回"空项目列表"这种模糊信号。
		if _, err := dal.GetBoardInTenant(boardID, claims.TenantID); err != nil {
			if err.Error() == "record not found" {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "board not found in tenant")
			}
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
	}
	rows, err := dal.ListBoardProjectsInTenant(claims.TenantID, boardID, 0)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return rows, nil
}

// GetProject 单个项目详情。
func (*BoardProjectService) GetProject(id string, claims *utils.UserClaims) (*model.BoardProject, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	project, err := dal.GetBoardProjectInTenant(id, claims.TenantID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "board project not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return project, nil
}

// UpdateProject 更新项目。
func (*BoardProjectService) UpdateProject(id string, req model.UpdateBoardProjectReq, claims *utils.UserClaims) (*model.BoardProject, error) {
	if err := ensureTenantScopedWriteClaims(claims, "update board project"); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "project name is required")
	}
	affected, err := dal.UpdateBoardProject(id, claims.TenantID, name, req.Description)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "board project name already exists in tenant")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "board project not found")
	}
	return dal.GetBoardProjectInTenant(id, claims.TenantID)
}

// DeleteProject 删除项目（只解除分组，不动看板）。
func (*BoardProjectService) DeleteProject(id string, claims *utils.UserClaims) error {
	if err := ensureTenantScopedWriteClaims(claims, "delete board project"); err != nil {
		return err
	}
	if _, err := dal.GetBoardProjectInTenant(id, claims.TenantID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errcode.NewWithMessage(errcode.CodeParamError, "board project not found")
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if err := dal.DeleteBoardProject(id, claims.TenantID); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// AddBoard 把租户内看板加入项目。
func (*BoardProjectService) AddBoard(projectID, boardID string, claims *utils.UserClaims) error {
	if err := ensureTenantScopedWriteClaims(claims, "add board to project"); err != nil {
		return err
	}
	if _, err := dal.GetBoardProjectInTenant(projectID, claims.TenantID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errcode.NewWithMessage(errcode.CodeParamError, "board project not found")
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if _, err := dal.GetBoardInTenant(boardID, claims.TenantID); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "board not found in tenant")
	}
	if err := dal.AddBoardToProject(projectID, boardID, claims.TenantID); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return errcode.NewWithMessage(errcode.CodeParamError, "board already belongs to a project")
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// RemoveBoard 把看板移出项目。本来就不在时视为幂等成功。
func (*BoardProjectService) RemoveBoard(projectID, boardID string, claims *utils.UserClaims) error {
	if err := ensureTenantScopedWriteClaims(claims, "remove board from project"); err != nil {
		return err
	}
	if _, err := dal.GetBoardProjectInTenant(projectID, claims.TenantID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errcode.NewWithMessage(errcode.CodeParamError, "board project not found")
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if _, err := dal.RemoveBoardFromProject(projectID, boardID, claims.TenantID); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// MembershipOf 反查看板所属项目（不在任何项目返回 nil）。
func (*BoardProjectService) MembershipOf(boardID string, claims *utils.UserClaims) (*model.BoardProject, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if _, err := dal.GetBoardInTenant(boardID, claims.TenantID); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "board not found in tenant")
	}
	project, err := dal.GetBoardProjectMembership(boardID, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return project, nil
}
