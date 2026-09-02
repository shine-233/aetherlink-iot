// board.go 负责看板增删改查、主页看板互斥、租户权限校验，
// 以及看板首页依赖的设备统计聚合能力。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	common "aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	secureuuid "github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type Board struct{}

// wrapBoardDBError 统一补齐数据库错误结构，保持接口层错误返回格式一致。
func wrapBoardDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func ensureBoardWritePermission(claims *utils.UserClaims, targetTenantID *string) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify board")
	}
	if claims.Authority != constant.SYS_ADMIN && claims.Authority != constant.TENANT_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify board")
	}
	if claims.Authority == constant.TENANT_ADMIN && claims.TenantID == "" {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify board")
	}
	if targetTenantID != nil {
		if *targetTenantID == "" || (claims.Authority == constant.TENANT_ADMIN && *targetTenantID != claims.TenantID) {
			return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify board")
		}
	}
	return nil
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
	if claims.Authority == constant.SYS_ADMIN {
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
	if requestedTenantID != "" && requestedTenantID != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify board")
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
	if claims.Authority == constant.SYS_ADMIN {
		if requested == "" {
			// Empty is the explicit all-tenant read scope for SYS_ADMIN.
			return "", nil
		}
		if err := validateBoardTenantExists(requested); err != nil {
			return "", err
		}
		return requested, nil
	}
	if claims.Authority != constant.TENANT_ADMIN || claims.TenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	if requested != "" && requested != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	return claims.TenantID, nil
}

func resolveBoardHomeTenant(requestedTenantID string, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	requestedTenantID = strings.TrimSpace(requestedTenantID)
	if claims.Authority == constant.SYS_ADMIN {
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
	if requestedTenantID != "" && requestedTenantID != claims.TenantID {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
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

// ensureBoardReadAccess 既校验看板存在，也把跨租户读取拦在 service 层。
func ensureBoardReadAccess(ctx context.Context, boardID string, claims *utils.UserClaims) (*model.Board, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	board, err := dal.BoardQuery{}.First(ctx, query.Board.ID.Eq(boardID))
	if err != nil {
		return nil, wrapBoardDBError(err)
	}
	if claims.Authority != constant.SYS_ADMIN && board.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	return board, nil
}

func ensureBoardWriteAccess(ctx context.Context, boardID string, claims *utils.UserClaims) (*model.Board, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query board")
	}
	if err := ensureBoardWritePermission(claims, nil); err != nil {
		return nil, err
	}
	board, err := ensureBoardReadAccess(ctx, boardID, claims)
	if err != nil {
		return nil, err
	}
	if err := ensureBoardWritePermission(claims, &board.TenantID); err != nil {
		return nil, err
	}
	return board, nil
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
	err = dal.DeleteBoard(id, board.TenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}

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
	// C2：tenantID 为空=管理员全量（scopes=nil），否则展开 self∪祖先层级作用域。
	var boardScopes []string
	if strings.TrimSpace(tenantID) != "" {
		boardScopes = expandTenantIDScope(tenantID)
	}
	total, list, err := dal.GetBoardListByPageForScopes(Params, boardScopes)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
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

func (*Board) GetBoardListByTenantId(tenantid string) (interface{}, error) {
	_, data, err := dal.GetBoardListByTenantId(tenantid)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, err
}

func (*Board) GetBoardHomeForClaims(tenantID string, claims *utils.UserClaims) (interface{}, error) {
	resolvedTenantID, err := resolveBoardHomeTenant(tenantID, claims)
	if err != nil {
		return nil, err
	}
	return (&Board{}).GetBoardListByTenantId(resolvedTenantID)
}

func (*Board) GetDeviceTotal(ctx context.Context, claims *utils.UserClaims) (int64, error) {
	var (
		total int64
		err   error
		db    = dal.DeviceQuery{}
	)
	if scopeErr := requireSupportedScopeAuthority(claims, "no permission to query device total"); scopeErr != nil {
		return 0, scopeErr
	}
	if common.CheckUserIsAdmin(claims.Authority) {
		total, err = db.Count(ctx)
	} else {
		tenantID, tenantErr := requireDeviceTenantClaims(claims, "no permission to query device total")
		if tenantErr != nil {
			return 0, tenantErr
		}
		device := query.Device
		conditions := []gen.Condition{device.TenantID.Eq(tenantID)}
		if ownerUserID := deviceOwnerUserIDFilterForClaims(claims); ownerUserID != nil {
			conditions = append(conditions, device.OwnerUserID.Eq(*ownerUserID))
		}
		total, err = db.CountByWhere(ctx, conditions...)
	}
	if err != nil {
		return 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return total, err
}

func (*Board) GetDevice(ctx context.Context, U *utils.UserClaims) (data *model.GetBoardDeviceRes, err error) {
	var (
		total, on int64
		device    = query.Device
		db        = dal.DeviceQuery{}
	)
	if scopeErr := requireSupportedScopeAuthority(U, "no permission to query device overview"); scopeErr != nil {
		return nil, scopeErr
	}
	if !common.CheckUserIsAdmin(U.Authority) {
		if _, scopeErr := requireDeviceTenantClaims(U, "no permission to query device overview"); scopeErr != nil {
			return nil, scopeErr
		}
	}
	ownerUserID := deviceOwnerUserIDFilterForClaims(U)

	// 非管理员只能统计本租户设备；管理员需要排除 inactive 设备，保持首页数字语义一致。
	if !common.CheckUserIsAdmin(U.Authority) {
		total, err = countVisibleBoardDevices(ctx, db, U.TenantID, ownerUserID, false)
	} else {
		total, err = db.CountByWhere(ctx, device.ActivateFlag.Neq("inactive"))
	}
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count failed:", err)
		err = errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
		return
	}
	// 在线数只统计 active 且在线的设备，离线数通过 total-on 反推。
	if !common.CheckUserIsAdmin(U.Authority) {
		on, err = countVisibleBoardDevices(ctx, db, U.TenantID, ownerUserID, true)
	} else {
		on, err = db.CountByWhere(ctx, device.ActivateFlag.Eq("active"), device.IsOnline.Eq(1))
	}
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count/on failed:", err)
		err = errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
		return
	}
	data = &model.GetBoardDeviceRes{
		DeviceTotal:   total,
		DeviceOn:      on,
		DeviceOffline: total - on,
	}
	return
}

func countVisibleBoardDevices(ctx context.Context, db dal.DeviceQuery, tenantID string, ownerUserID *string, activeOnly bool) (int64, error) {
	return countBoardDevicesByScope(ctx, db, tenantID, ownerUserID, activeOnly, false)
}

func countBoardDevicesByScope(ctx context.Context, db dal.DeviceQuery, tenantID string, ownerUserID *string, activeOnly bool, allTenants bool) (int64, error) {
	device := query.Device
	conditions := make([]gen.Condition, 0, 3)
	if !allTenants {
		conditions = append(conditions, device.TenantID.Eq(tenantID))
	}
	if activeOnly {
		conditions = append(conditions, device.ActivateFlag.Eq("active"), device.IsOnline.Eq(1))
	} else {
		conditions = append(conditions, device.ActivateFlag.Neq("inactive"))
	}
	if ownerUserID != nil && *ownerUserID != "" {
		conditions = append(conditions, device.OwnerUserID.Eq(*ownerUserID))
	}
	return db.CountByWhere(ctx, conditions...)
}

func (b *Board) GetDeviceByTenantID(ctx context.Context, claims *utils.UserClaims) (data *model.GetBoardDeviceRes, err error) {
	return b.GetDeviceOverview(ctx, &model.GetBoardDeviceReq{}, claims)
}

func (*Board) GetDeviceOverview(ctx context.Context, req *model.GetBoardDeviceReq, claims *utils.UserClaims) (data *model.GetBoardDeviceRes, err error) {
	var (
		total, on int64
		db        = dal.DeviceQuery{}
	)
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device overview request is required")
	}
	if err := requireSystemAdminAllTenantsScope(
		req.AllTenants,
		claims,
		"all-tenants device overview is only available to system administrators",
	); err != nil {
		return nil, err
	}
	tenantID := ""
	if !req.AllTenants {
		tenantID, err = requireDeviceTenantClaims(claims, "no permission to query tenant device overview")
		if err != nil {
			return nil, err
		}
	}
	ownerUserID := deviceOwnerUserIDFilterForClaims(claims)

	total, err = countBoardDevicesByScope(ctx, db, tenantID, ownerUserID, false, req.AllTenants)
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count failed:", err)
		return
	}
	on, err = countBoardDevicesByScope(ctx, db, tenantID, ownerUserID, true, req.AllTenants)
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count/on failed:", err)
		return
	}
	data = &model.GetBoardDeviceRes{
		DeviceTotal:   total,
		DeviceOn:      on,
		DeviceOffline: total - on,
	}
	return
}

func (*Device) GetDeviceTrend(ctx context.Context, claims *utils.UserClaims, tenantID string, startTime, endTime *int64) (*model.DeviceTrendRes, error) {
	var points []model.DeviceTrendPoint
	if err := requireSupportedScopeAuthority(claims, "no permission to query device trend"); err != nil {
		return nil, err
	}
	if claims.Authority != constant.SYS_ADMIN {
		claimsTenantID, err := requireDeviceTenantClaims(claims, "no permission to query device trend")
		if err != nil {
			return nil, err
		}
		if tenantID != claimsTenantID {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device trend")
		}
	}

	points, err := dal.GetDeviceTrend(tenantID, deviceOwnerUserIDFilterForClaims(claims), startTime, endTime)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return &model.DeviceTrendRes{
		Points: points,
	}, nil
}
