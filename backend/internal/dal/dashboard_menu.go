// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"errors"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetTenantDashboardMenu(tenantID string, dashboardID string) (*model.TenantDashboardMenu, error) {
	var menu model.TenantDashboardMenu
	err := global.DB.Where("tenant_id = ? AND dashboard_id = ?", tenantID, dashboardID).First(&menu).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logrus.Error(err)
		return nil, err
	}
	return &menu, nil
}

func ListTenantDashboardMenusByDashboardIDs(tenantID string, dashboardIDs []string) ([]model.TenantDashboardMenu, error) {
	if len(dashboardIDs) == 0 {
		return []model.TenantDashboardMenu{}, nil
	}
	var menus []model.TenantDashboardMenu
	err := global.DB.
		Where("tenant_id = ? AND dashboard_id IN ?", tenantID, dashboardIDs).
		Find(&menus).Error
	if err != nil {
		logrus.Error(err)
	}
	return menus, err
}

func UpsertTenantDashboardMenu(menu *model.TenantDashboardMenu) error {
	err := global.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "dashboard_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"dashboard_name",
			"menu_name",
			"parent_code",
			"sort",
			"enabled",
			"updated_at",
		}),
	}).Create(menu).Error
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func DeleteTenantDashboardMenu(tenantID string, dashboardID string) error {
	err := global.DB.Where("tenant_id = ? AND dashboard_id = ?", tenantID, dashboardID).
		Delete(&model.TenantDashboardMenu{}).Error
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func ListTenantDashboardMenus(tenantID string, parentCode string) ([]model.TenantDashboardMenu, error) {
	var menus []model.TenantDashboardMenu
	err := global.DB.
		Where("tenant_id = ? AND parent_code = ? AND enabled = ?", tenantID, parentCode, true).
		Order(`sort asc, created_at asc`).
		Find(&menus).Error
	if err != nil {
		logrus.Error(err)
	}
	return menus, err
}

func GetVisDashboardByID(dashboardID string) (*model.VisDashboard, error) {
	dashboard, err := query.VisDashboard.Where(query.VisDashboard.ID.Eq(dashboardID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logrus.Error(err)
		return nil, err
	}
	return dashboard, nil
}

func GetNativeBoardByID(boardID string) (*model.Board, error) {
	board, err := query.Board.Where(query.Board.ID.Eq(boardID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logrus.Error(err)
		return nil, err
	}
	return board, nil
}
