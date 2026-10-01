// 文件用途：维护仪表盘菜单与前端导航配置服务。
// 核心逻辑：按租户和角色读取、创建、更新菜单项，并生成前端可消费的菜单结构。
// 关键注意事项：菜单权限会影响页面可见性，跨租户菜单和隐藏项变更需要谨慎处理。
// 重构建议：拆分菜单树构建与权限过滤，补齐角色边界、排序稳定性和事务写入测试。
package service

import (
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service/kit"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type DashboardMenu struct{}

const maxBatchDashboardMenuIDs = 100

// dashboardMenuTarget keeps the menu service independent from the concrete
// visualization storage. Legacy ThingsVis dashboards live in vis_dashboard;
// native boards live in boards, but both expose the same tenant-scoped menu
// contract to the caller.
type dashboardMenuTarget struct {
	DashboardName *string
}

// dashboardMenuScope 菜单仅对租户用户开放：nil claims 与空白租户分别报无权限。
var dashboardMenuScope = kit.TenantScope{
	NilMsg:   "no permission to manage dashboard menu",
	BlankMsg: "tenant dashboard menu is only available for tenant users",
}

// dashboardIDRequired 每次新建错误值（*errcode.Error 可被调用方修改，不共享单例）。
func dashboardIDRequired() error {
	return errcode.NewWithMessage(errcode.CodeParamError, "dashboard_id is required")
}

func validateDashboardMenuAccess(claims *utils.UserClaims, dashboardID string) (string, string, error) {
	tenantID, err := dashboardMenuScope.Tenant(claims)
	if err != nil {
		return "", "", err
	}
	normalizedDashboardID := strings.TrimSpace(dashboardID)
	if normalizedDashboardID == "" {
		return "", "", dashboardIDRequired()
	}
	return tenantID, normalizedDashboardID, nil
}

// dashboardTargetDBErr 看板查询失败：在 operation/error 之外额外携带 dashboard_id（契约键）。
func dashboardTargetDBErr(op, dashboardID string, err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"operation":    op,
		"dashboard_id": dashboardID,
		"error":        err.Error(),
	})
}

func ensureTenantDashboardTarget(tenantID string, dashboardID string) (*dashboardMenuTarget, error) {
	dashboard, err := dal.GetVisDashboardByID(dashboardID)
	if err != nil {
		return nil, dashboardTargetDBErr("get_vis_dashboard_before_menu", dashboardID, err)
	}
	if dashboard != nil {
		if dashboard.TenantID == nil || strings.TrimSpace(*dashboard.TenantID) != tenantID {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "dashboard not found or no permission")
		}
		return &dashboardMenuTarget{DashboardName: dashboard.DashboardName}, nil
	}

	nativeBoard, err := dal.GetNativeBoardByID(dashboardID)
	if err != nil {
		return nil, dashboardTargetDBErr("get_native_board_before_menu", dashboardID, err)
	}
	if nativeBoard == nil || strings.TrimSpace(nativeBoard.TenantID) != tenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "dashboard not found or no permission")
	}

	return &dashboardMenuTarget{DashboardName: &nativeBoard.Name}, nil
}

func (*DashboardMenu) GetTenantDashboardMenu(claims *utils.UserClaims, dashboardID string) (*model.TenantDashboardMenuRsp, error) {
	tenantID, normalizedDashboardID, err := validateDashboardMenuAccess(claims, dashboardID)
	if err != nil {
		return nil, err
	}

	menu, err := dal.GetTenantDashboardMenu(tenantID, normalizedDashboardID)
	if err != nil {
		return nil, kit.DBErrOp("get_dashboard_menu", err)
	}

	if menu == nil {
		return nil, nil
	}

	return menu.ToRsp(), nil
}

func (*DashboardMenu) GetTenantDashboardMenus(claims *utils.UserClaims, dashboardIDs []string) (map[string]*model.TenantDashboardMenuRsp, error) {
	tenantID, normalizedDashboardIDs, err := validateDashboardMenuBatchAccess(claims, dashboardIDs)
	if err != nil {
		return nil, err
	}

	menus, err := dal.ListTenantDashboardMenusByDashboardIDs(tenantID, normalizedDashboardIDs)
	if err != nil {
		return nil, kit.DBErrOp("batch_get_dashboard_menu", err)
	}

	result := make(map[string]*model.TenantDashboardMenuRsp, len(normalizedDashboardIDs))
	for _, dashboardID := range normalizedDashboardIDs {
		result[dashboardID] = nil
	}
	for i := range menus {
		menu := menus[i]
		result[menu.DashboardID] = menu.ToRsp()
	}
	return result, nil
}

func validateDashboardMenuBatchAccess(claims *utils.UserClaims, dashboardIDs []string) (string, []string, error) {
	tenantID, err := dashboardMenuScope.Tenant(claims)
	if err != nil {
		return "", nil, err
	}
	if len(dashboardIDs) == 0 {
		return "", nil, errcode.NewWithMessage(errcode.CodeParamError, "dashboard_ids are required")
	}

	normalizedIDs := make([]string, 0, len(dashboardIDs))
	seen := make(map[string]struct{}, len(dashboardIDs))
	for _, rawID := range dashboardIDs {
		dashboardID := strings.TrimSpace(rawID)
		if dashboardID == "" {
			return "", nil, dashboardIDRequired()
		}
		if _, ok := seen[dashboardID]; ok {
			continue
		}
		seen[dashboardID] = struct{}{}
		normalizedIDs = append(normalizedIDs, dashboardID)
		if len(normalizedIDs) > maxBatchDashboardMenuIDs {
			return "", nil, errcode.NewWithMessage(errcode.CodeParamError, "dashboard_ids cannot exceed 100")
		}
	}
	return tenantID, normalizedIDs, nil
}

func (*DashboardMenu) UpsertTenantDashboardMenu(claims *utils.UserClaims, dashboardID string, req *model.UpsertTenantDashboardMenuReq) (*model.TenantDashboardMenuRsp, error) {
	tenantID, normalizedDashboardID, err := validateDashboardMenuAccess(claims, dashboardID)
	if err != nil {
		return nil, err
	}
	dashboard, err := ensureTenantDashboardTarget(tenantID, normalizedDashboardID)
	if err != nil {
		return nil, err
	}

	sortValue := int16(1)
	if req.Sort != nil {
		sortValue = *req.Sort
	}

	enabledValue := true
	if req.Enabled != nil {
		enabledValue = *req.Enabled
	}

	dashboardName := req.MenuName
	if req.DashboardName != nil && *req.DashboardName != "" {
		dashboardName = *req.DashboardName
	} else if dashboard.DashboardName != nil && *dashboard.DashboardName != "" {
		dashboardName = *dashboard.DashboardName
	}

	existing, err := dal.GetTenantDashboardMenu(tenantID, normalizedDashboardID)
	if err != nil {
		return nil, kit.DBErrOp("get_dashboard_menu_before_upsert", err)
	}

	now := kit.NowUTC()
	menu := model.TenantDashboardMenu{
		ID:            kit.NewID(),
		TenantID:      tenantID,
		DashboardID:   normalizedDashboardID,
		DashboardName: dashboardName,
		MenuName:      req.MenuName,
		ParentCode:    "home",
		Sort:          sortValue,
		Enabled:       enabledValue,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if existing != nil {
		menu.ID = existing.ID
		menu.CreatedAt = existing.CreatedAt
	}

	err = dal.UpsertTenantDashboardMenu(&menu)
	if err != nil {
		logrus.Error(err)
		return nil, kit.DBErrOp("upsert_dashboard_menu", err)
	}

	return menu.ToRsp(), nil
}

func (*DashboardMenu) DeleteTenantDashboardMenu(claims *utils.UserClaims, dashboardID string) error {
	tenantID, normalizedDashboardID, err := validateDashboardMenuAccess(claims, dashboardID)
	if err != nil {
		return err
	}

	err = dal.DeleteTenantDashboardMenu(tenantID, normalizedDashboardID)
	if err != nil {
		return kit.DBErrOp("delete_dashboard_menu", err)
	}
	return nil
}
