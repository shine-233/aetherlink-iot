// 文件用途：大屏看板模板导出与导入（TP-5 资源中心）。
// 核心逻辑：
// 1. 导出：将租户看板转化为跨租户可移植的描述符（BoardTemplateExport），剔除租户与敏感实例标识；
// 2. 导入：租户鉴权后，解析模板并校验配置合法性，在目标租户内幂等落库。
package service

import (
	"context"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

// ExportBoard 将指定看板导出为便携模板描述符。
func (b *Board) ExportBoard(id string, claims *utils.UserClaims) (*model.BoardTemplateExport, error) {
	ctx := context.Background()
	board, err := ensureBoardReadAccess(ctx, id, claims)
	if err != nil {
		return nil, err
	}

	author := ""
	if board.Author != nil && *board.Author != "" {
		author = *board.Author
	} else if claims != nil && claims.Email != "" {
		author = claims.Email
	}

	version := "1.0.0"
	if board.Version != nil && *board.Version != "" {
		version = *board.Version
	}

	path := ""
	if board.PreviewURL != nil {
		path = *board.PreviewURL
	}

	visType := "native"
	if board.VisType != nil && *board.VisType != "" {
		visType = *board.VisType
	}

	typeKey := ""
	if board.TypeKey != nil {
		typeKey = *board.TypeKey
	}

	menuFlag := "N"
	if board.MenuFlag != nil {
		menuFlag = *board.MenuFlag
	}

	// 累计导出计数
	_ = dal.IncrementBoardDownloadCounts(ctx, []string{id})

	return &model.BoardTemplateExport{
		Kind:        "aetherlink-board-template",
		Name:        board.Name,
		Author:      &author,
		Version:     &version,
		Description: board.Description,
		Remark:      board.Remark,
		Path:        &path,
		VisType:     &visType,
		TypeKey:     &typeKey,
		Config:      board.Config,
		HomeFlag:    "N",
		MenuFlag:    menuFlag,
		ExportedAt:  time.Now().Format(time.RFC3339),
	}, nil
}

// ImportBoard 将看板模板导入至当前租户。
func (b *Board) ImportBoard(req *model.BoardTemplateExport, claims *utils.UserClaims) (*model.Board, error) {
	if err := ensureTenantScopedWriteClaims(claims, "import board template"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "board template is required")
	}

	kind := strings.TrimSpace(req.Kind)
	if kind != "" && kind != "aetherlink-board-template" {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "invalid template kind, expected aetherlink-board-template",
		})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "board name is required")
	}

	if err := validateBoardConfig(req.Config, func() error {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"field": "config",
			"error": "config must be valid JSON",
		})
	}); err != nil {
		return nil, err
	}

	if err := validateBoardVisType(req.VisType); err != nil {
		return nil, err
	}

	ctx := context.Background()
	tenantID := claims.TenantID

	// 检查当前租户是否已存在同名看板
	existing, err := dal.GetBoardByNameInTenant(ctx, tenantID, name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, wrapBoardDBError(err)
	}

	visType := "native"
	if req.VisType != nil && *req.VisType != "" {
		visType = *req.VisType
	}

	version := "1.0.0"
	if req.Version != nil && *req.Version != "" {
		version = *req.Version
	}

	author := ""
	if req.Author != nil {
		author = *req.Author
	}

	path := ""
	if req.Path != nil {
		path = *req.Path
	}

	typeKey := ""
	if req.TypeKey != nil {
		typeKey = *req.TypeKey
	}

	now := time.Now()

	if existing != nil {
		// 更新已存在看板
		existing.Config = req.Config
		existing.Description = req.Description
		existing.Remark = req.Remark
		existing.VisType = &visType
		existing.Author = &author
		existing.Version = &version
		existing.PreviewURL = &path
		existing.TypeKey = &typeKey
		existing.UpdatedAt = now
		if err := dal.UpdateBoard(existing, tenantID); err != nil {
			return nil, wrapBoardDBError(err)
		}
		return existing, nil
	}

	// 新建看板
	homeFlag := "N"
	menuFlag := "N"
	if req.MenuFlag != "" {
		menuFlag = req.MenuFlag
	}

	newBoard := &model.Board{
		ID:            uuid.New(),
		Name:          name,
		Config:        req.Config,
		TenantID:      tenantID,
		CreatedAt:     now,
		UpdatedAt:     now,
		HomeFlag:      homeFlag,
		Description:   req.Description,
		Remark:        req.Remark,
		MenuFlag:      &menuFlag,
		VisType:       &visType,
		TypeKey:       &typeKey,
		Author:        &author,
		Version:       &version,
		PreviewURL:    &path,
		DownloadCount: 0,
	}

	if err := query.Board.WithContext(ctx).Create(newBoard); err != nil {
		return nil, wrapBoardDBError(err)
	}

	return newBoard, nil
}
