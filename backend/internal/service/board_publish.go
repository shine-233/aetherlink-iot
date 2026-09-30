// board_publish.go 负责看板发布/公开分享聚合。
//
// 9-28 域拆分：自 board.go 按聚合迁出，仅移动代码，函数签名与语义不变。
// 发布走独立的稳定 token 契约，与通用看板更新隔离，避免改渲染数据时误公开。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	secureuuid "github.com/google/uuid"
	"gorm.io/gorm"
)

// PublishBoard creates a stable public token for a native board. Publishing
// is deliberately separate from the generic board update contract so a
// caller cannot accidentally make a board public by changing renderer data.
func (*Board) PublishBoard(id string, claims *utils.UserClaims) (*model.Board, error) {
	board, err := ensureBoardWriteAccess(context.Background(), id, claims)
	if err != nil {
		return nil, err
	}
	if board.VisType == nil || strings.TrimSpace(*board.VisType) != "native" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "only native boards can be published locally")
	}

	shareToken := strings.TrimSpace(pointerStringValue(board.ShareToken))
	if shareToken == "" {
		shareToken = secureuuid.NewString()
	}
	publishedAt := time.Now().UTC()
	published, err := dal.PublishBoard(board.ID, board.TenantID, shareToken, publishedAt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "native board not found")
		}
		return nil, wrapBoardDBError(err)
	}
	return published, nil
}

func pointerStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// GetPublishedBoardByShareToken is used by the public preview route and does
// not accept tenant or user identifiers from the caller.
func (*Board) GetPublishedBoardByShareToken(token string) (*model.Board, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 64 {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "dashboard not found")
	}
	board, err := dal.GetPublishedBoardByShareToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "dashboard not found")
		}
		return nil, wrapBoardDBError(err)
	}
	return board, nil
}
