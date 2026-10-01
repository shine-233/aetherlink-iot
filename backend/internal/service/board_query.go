// board_query.go 负责看板读路径聚合：分页列表、单查、按租户列表与首页看板。
//
// 9-28 域拆分：自 board.go 按聚合迁出，仅移动代码，函数签名与语义不变。
// 作用域解析（resolveBoardListTenant / resolveBoardHomeTenant）仍在 board.go，
// 与写路径的租户解析集中维护。
package service

import (
	"context"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func (*Board) GetBoardListByPage(Params *model.GetBoardListByPageReq, U *utils.UserClaims) (map[string]interface{}, error) {
	if U == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	if err := validateBoardVisType(Params.VisType); err != nil {
		return nil, err
	}
	tenantID, err := resolveBoardListTenant(Params.TenantID, U)
	if err != nil {
		return nil, err
	}
	// C2：tenantID 为空=管理员全量（scopes=nil），否则展开 self∪子孙（自上而下）层级作用域。
	var boardScopes []string
	if strings.TrimSpace(tenantID) != "" {
		boardScopes = expandTenantIDScope(tenantID)
	}
	// TB-46 组共享可见性：普通租户用户隐藏"组绑定且非组内成员"的看板；
	// 管理员不受限（hidden=nil）。映射错误上抛（fail-closed），不得降级放行。
	hiddenBoardIDs, err := groupHiddenResourceIDs(boardScopes, U, model.GroupElementKindBoard)
	if err != nil {
		return nil, wrapBoardDBError(err)
	}
	total, list, err := dal.GetBoardListByPageForScopesWithGroupScope(Params, boardScopes, hiddenBoardIDs)
	if err != nil {
		return nil, dbError(err)
	}
	boardListRsp := make(map[string]interface{})
	boardListRsp["total"] = total
	boardListRsp["list"] = list

	return boardListRsp, err
}

func (*Board) GetBoard(id string, U *utils.UserClaims) (interface{}, error) {
	board, err := ensureBoardReadAccess(context.Background(), id, U)
	if err != nil {
		return nil, err
	}

	return board, err
}

func (*Board) GetBoardHomeForClaims(tenantID string, claims *utils.UserClaims) (interface{}, error) {
	resolvedTenantID, err := resolveBoardHomeTenant(tenantID, claims)
	if err != nil {
		return nil, err
	}
	// TB-46 组共享可见性：首页看板被绑定到用户组时，对组外成员 fail-closed 隐藏
	// （返回 nil = 无首页看板）。管理员不受限；映射错误上抛，不得降级放行。
	hiddenBoardIDs, err := groupHiddenResourceIDs([]string{resolvedTenantID}, claims, model.GroupElementKindBoard)
	if err != nil {
		return nil, wrapBoardDBError(err)
	}
	_, data, err := dal.GetBoardListByTenantId(resolvedTenantID)
	if err != nil {
		return nil, dbError(err)
	}
	if board, ok := data.(*model.Board); ok && board != nil && len(hiddenBoardIDs) > 0 {
		for _, id := range hiddenBoardIDs {
			if id == board.ID {
				return nil, nil
			}
		}
	}
	return data, nil
}
