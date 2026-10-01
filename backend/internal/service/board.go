// board.go 负责看板增删改查、主页看板互斥、租户权限校验。
//
// 主要职责：
// 1. 维护看板实体的创建、更新、删除与旧接口兼容语义。
// 2. 统一解析写/列表/首页三条路径的租户上下文（SYS_ADMIN 指定租户、管理员校验）。
// 3. 保证同一租户最多只有一个主页看板（互斥清位）。
//
// 9-28 域拆分：本文件保留 CRUD 聚合与租户解析；发布/公开分享迁至 board_publish.go，
// 读路径（分页/单查/首页）迁至 board_query.go，首页设备统计迁至 board_device_stats.go。
// 权限校验集中在 access_helpers.go（ensureBoardWritePermission/ensureBoard*Access），
// 新增入口时应优先复用。拆分仅移动同包代码，导出符号与方法签名不变。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/authz"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type Board struct{}

// wrapBoardDBError 统一补齐数据库错误结构，保持接口层错误返回格式一致。
func wrapBoardDBError(err error) error {
	return dbError(err)
}

func boardTenantContextError(message string) error {
	return errcode.NewWithMessage(errcode.CodeParamError, message)
}

func validateBoardTenantExists(tenantID string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return boardTenantContextError("tenant context is required")
	}
	if _, err := dal.GetTenantAdmin(tenantID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.NewWithMessage(errcode.CodeNotFound, "tenant context not found")
		}
		return wrapBoardDBError(err)
	}
	return nil
}

func resolveBoardWriteTenant(requestedTenantID string, claims *utils.UserClaims) (string, error) {
	if err := ensureBoardWritePermission(claims, nil); err != nil {
		return "", err
	}
	requestedTenantID = strings.TrimSpace(requestedTenantID)
	if authz.IsSysAdmin(claims) {
		if requestedTenantID == "" {
			return "", boardTenantContextError("tenant context is required for board creation")
		}
		if err := validateBoardTenantExists(requestedTenantID); err != nil {
			return "", err
		}
		return requestedTenantID, nil
	}

	if claims.TenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify board")
	}
	if requestedTenantID != "" {
		if err := authz.CheckTenant(claims, requestedTenantID, boardModifyPermissionMessage); err != nil {
			return "", err
		}
	}
	return claims.TenantID, nil
}

func resolveBoardListTenant(requestedTenantID *string, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	requested := ""
	if requestedTenantID != nil {
		requested = strings.TrimSpace(*requestedTenantID)
	}
	if authz.IsSysAdmin(claims) {
		if requested == "" {
			// Empty is the explicit all-tenant read scope for SYS_ADMIN.
			return "", nil
		}
		if err := validateBoardTenantExists(requested); err != nil {
			return "", err
		}
		return requested, nil
	}
	if !authz.HasRole(claims, authz.TenantAdmin) || claims.TenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	if requested != "" {
		if err := authz.CheckTenant(claims, requested, "no permission to query board"); err != nil {
			return "", err
		}
	}
	return claims.TenantID, nil
}

func resolveBoardHomeTenant(requestedTenantID string, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	requestedTenantID = strings.TrimSpace(requestedTenantID)
	if authz.IsSysAdmin(claims) {
		if requestedTenantID == "" {
			return "", boardTenantContextError("tenant context is required for the board home")
		}
		if err := validateBoardTenantExists(requestedTenantID); err != nil {
			return "", err
		}
		return requestedTenantID, nil
	}
	// The home endpoint is a read-only dashboard surface. Unlike the paged
	// management list, it is intentionally available to any authenticated
	// user with a tenant context; the API contract documents that same-tenant
	// users may view the tenant's home boards. Keep the administrator-only
	// boundary in resolveBoardListTenant for CRUD/list management.
	if claims.TenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	if requestedTenantID != "" {
		if err := authz.CheckTenant(claims, requestedTenantID, "no permission to query board"); err != nil {
			return "", err
		}
	}
	return claims.TenantID, nil
}

func validateBoardConfig(config *string, invalidConfigErr func() error) error {
	if config != nil && !IsJSON(*config) {
		return invalidConfigErr()
	}
	return nil
}

func validateBoardVisType(visType *string) error {
	if visType == nil || *visType == "" || *visType == "native" || *visType == "thingsvis" {
		return nil
	}
	return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
		"field": "vis_type",
		"error": "vis_type must be native or thingsvis",
	})
}

func buildCreateBoardPayload(req *model.CreateBoardReq, tenantID string, now time.Time) model.Board {
	return model.Board{
		ID:          uuid.New(),
		Name:        req.Name,
		Config:      req.Config,
		MenuFlag:    &req.MenuFlag,
		Description: req.Description,
		Remark:      req.Remark,
		UpdatedAt:   now,
		CreatedAt:   now,
		TenantID:    tenantID,
		HomeFlag:    req.HomeFlag,
		VisType:     req.VisType,
		TypeKey:     req.TypeKey,
		Author:      req.Author,
		Version:     req.Version,
		PreviewURL:  req.PreviewURL,
	}
}

func buildUpdateBoardPayload(req *model.UpdateBoardReq, updatedAt time.Time) model.Board {
	return model.Board{
		ID:          req.Id,
		Name:        req.Name,
		Config:      req.Config,
		HomeFlag:    req.HomeFlag,
		MenuFlag:    &req.MenuFlag,
		Description: req.Description,
		Remark:      req.Remark,
		VisType:     req.VisType,
		TypeKey:     req.TypeKey,
		Author:      req.Author,
		Version:     req.Version,
		PreviewURL:  req.PreviewURL,
		UpdatedAt:   updatedAt,
	}
}

func tenantHomeBoardExists(ctx context.Context, db dal.BoardQuery, tenantID string, excludeBoardID string) (bool, error) {
	conditions := []gen.Condition{
		query.Board.TenantID.Eq(tenantID),
		query.Board.HomeFlag.Eq("Y"),
	}
	if excludeBoardID != "" {
		conditions = append(conditions, query.Board.ID.Neq(excludeBoardID))
	}

	if _, err := db.First(ctx, conditions...); err != nil {
		return false, err
	}
	return true, nil
}

// syncTenantHomeBoardForUpdate 保证同一租户最多只有一个主页看板。
// 当当前看板要切为主页时，先把该租户其他主页标记清空。
func syncTenantHomeBoardForUpdate(ctx context.Context, db dal.BoardQuery, tenantID string, boardID string, homeFlag string) error {
	if homeFlag != "Y" {
		return nil
	}

	exists, err := tenantHomeBoardExists(ctx, db, tenantID, boardID)
	if err != nil {
		logrus.Error(err)
		return nil
	}
	if !exists {
		return nil
	}

	if err := db.UpdateHomeFlagN(ctx, tenantID); err != nil {
		logrus.Error(err)
		return wrapBoardDBError(err)
	}
	return nil
}

func ensureBoardCreateDefaults(board *model.Board) error {
	if board.Name == "" {
		return fmt.Errorf("name is required")
	}
	if board.HomeFlag == "" {
		board.HomeFlag = "N"
	}
	return nil
}

func ensureNoDuplicateHomeBoardOnCreate(ctx context.Context, db dal.BoardQuery, tenantID string, homeFlag string) error {
	if homeFlag != "Y" {
		return nil
	}

	exists, err := tenantHomeBoardExists(ctx, db, tenantID, "")
	if err != nil {
		logrus.Error(err)
		return nil
	}
	if exists {
		return errcode.New(203004)
	}
	return nil
}

func prepareBoardUpdate(ctx context.Context, db dal.BoardQuery, req *model.UpdateBoardReq, claims *utils.UserClaims, board *model.Board) error {
	oldBoard, err := ensureBoardWriteAccess(ctx, req.Id, claims)
	if err != nil {
		return err
	}

	req.TenantID = oldBoard.TenantID
	return syncTenantHomeBoardForUpdate(ctx, db, req.TenantID, req.Id, board.HomeFlag)
}

func persistBoardUpdate(board *model.Board, tenantID string) error {
	if err := dal.UpdateBoard(board, tenantID); err != nil {
		logrus.Error(err)
		return wrapBoardDBError(err)
	}
	return nil
}

func createBoardFromUpdate(ctx context.Context, db dal.BoardQuery, req *model.UpdateBoardReq, claims *utils.UserClaims, board *model.Board) (*model.Board, error) {
	tenantID, err := resolveBoardWriteTenant(req.TenantID, claims)
	if err != nil {
		return nil, err
	}
	if err := ensureBoardCreateDefaults(board); err != nil {
		return nil, err
	}

	// 兼容旧接口：当 update 请求没有 id 时，按创建新看板处理。
	board.ID = uuid.New()
	board.TenantID = tenantID
	req.TenantID = tenantID

	if err := ensureNoDuplicateHomeBoardOnCreate(ctx, db, req.TenantID, board.HomeFlag); err != nil {
		return nil, err
	}

	boardInfo, err := db.Create(ctx, board)
	if err != nil {
		logrus.Error(err)
		err = wrapBoardDBError(err)
	}
	return boardInfo, err
}

func (*Board) CreateBoard(ctx context.Context, CreateBoardReq *model.CreateBoardReq, claims *utils.UserClaims) (*model.Board, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create board")
	}
	if err := validateBoardVisType(CreateBoardReq.VisType); err != nil {
		return nil, err
	}
	if err := validateBoardConfig(CreateBoardReq.Config, func() error {
		return errcode.NewWithMessage(errcode.CodeParamError, "config is not a valid JSON")
	}); err != nil {
		return nil, err
	}
	tenantID, err := resolveBoardWriteTenant(CreateBoardReq.TenantID, claims)
	if err != nil {
		return nil, err
	}

	db := dal.BoardQuery{}

	// TB-15: 实体名冲突策略消解（FAIL / RENAME / IGNORE / UPDATE）
	policy := model.NormalizeConflictPolicy(CreateBoardReq.ConflictPolicy)
	if policy != model.ConflictPolicyAllow {
		existing, err := db.GetBoardByNameAndTenant(ctx, tenantID, CreateBoardReq.Name)
		if err == nil && existing != nil {
			switch policy {
			case model.ConflictPolicyFail:
				return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("board with name '%s' already exists", CreateBoardReq.Name))
			case model.ConflictPolicyIgnore:
				return existing, nil
			case model.ConflictPolicyUpdate:
				existing.Config = CreateBoardReq.Config
				existing.Description = CreateBoardReq.Description
				existing.Remark = CreateBoardReq.Remark
				existing.VisType = CreateBoardReq.VisType
				existing.TypeKey = CreateBoardReq.TypeKey
				existing.Author = CreateBoardReq.Author
				existing.Version = CreateBoardReq.Version
				existing.PreviewURL = CreateBoardReq.PreviewURL
				existing.UpdatedAt = time.Now().UTC()
				if CreateBoardReq.MenuFlag != "" {
					existing.MenuFlag = &CreateBoardReq.MenuFlag
				}
				if CreateBoardReq.HomeFlag == "Y" {
					_ = db.UpdateHomeFlagN(ctx, tenantID)
					existing.HomeFlag = "Y"
				}
				if err := dal.UpdateBoard(existing, tenantID); err != nil {
					return nil, wrapBoardDBError(err)
				}
				return existing, nil
			case model.ConflictPolicyRename:
				names, err := db.GetBoardNamesMatchingBase(ctx, tenantID, CreateBoardReq.Name)
				if err != nil {
					return nil, wrapBoardDBError(err)
				}
				nameMap := make(map[string]bool, len(names))
				for _, n := range names {
					nameMap[n] = true
				}
				renamed := model.GenerateRenamedName(CreateBoardReq.Name, model.NameMaxLengthDefault, func(candidate string) bool {
					return nameMap[candidate]
				})
				CreateBoardReq.Name = renamed
			}
		}
	}

	board := buildCreateBoardPayload(CreateBoardReq, tenantID, time.Now().UTC())
	if CreateBoardReq.HomeFlag == "Y" {
		err := db.UpdateHomeFlagN(ctx, tenantID)
		if err != nil {
			logrus.Error(err)
			return nil, wrapBoardDBError(err)
		}
	}

	boardInfo, err := db.Create(ctx, &board)
	if err != nil {
		logrus.Error(err)
		err = wrapBoardDBError(err)
	}

	return boardInfo, err
}

func (*Board) UpdateBoard(ctx context.Context, UpdateBoardReq *model.UpdateBoardReq, claims *utils.UserClaims) (*model.Board, error) {
	if claims == nil {
		return nil, ensureBoardWritePermission(claims, nil)
	}
	if err := validateBoardVisType(UpdateBoardReq.VisType); err != nil {
		return nil, err
	}
	if err := validateBoardConfig(UpdateBoardReq.Config, func() error {
		return errcode.WithVars(100002, map[string]interface{}{
			"field": "config",
			"error": "config is not a valid JSON",
		})
	}); err != nil {
		return nil, err
	}
	if err := ensureBoardWritePermission(claims, nil); err != nil {
		return nil, err
	}

	db := dal.BoardQuery{}
	board := buildUpdateBoardPayload(UpdateBoardReq, time.Now().UTC())
	if UpdateBoardReq.Id == "" {
		return createBoardFromUpdate(ctx, db, UpdateBoardReq, claims, &board)
	}

	if err := prepareBoardUpdate(ctx, db, UpdateBoardReq, claims, &board); err != nil {
		return nil, err
	}
	if err := persistBoardUpdate(&board, UpdateBoardReq.TenantID); err != nil {
		return nil, err
	}

	return &board, nil
}

func (*Board) DeleteBoard(id string, claims *utils.UserClaims) error {
	board, err := ensureBoardWriteAccess(context.Background(), id, claims)
	if err != nil {
		return err
	}
	// 先解除项目归属（P1.x 看板项目分组）：归属删除失败则中止删除看板，
	// 避免留下指向已删看板的悬挂归属记录；顺序反过来会产生先删后失败的窗口。
	if _, merr := dal.RemoveAllBoardProjectMemberships(id, board.TenantID); merr != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": "remove board project membership before delete: " + merr.Error(),
		})
	}
	err = dal.DeleteBoard(id, board.TenantID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.NewWithMessage(errcode.CodeNotFound, "board not found")
	}
	if err != nil {
		return dbError(err)
	}
	return err
}
