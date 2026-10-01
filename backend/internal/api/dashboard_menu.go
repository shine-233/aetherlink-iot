package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DashboardMenuApi struct{}

func (*DashboardMenuApi) GetDashboardMenu(c *gin.Context) {
	HandlePath(c, "dashboardId", func(dashboardID string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DashboardMenu.GetTenantDashboardMenu(userClaims, dashboardID)
	})
}

func (*DashboardMenuApi) BatchGetDashboardMenus(c *gin.Context) {
	Handle(c, func(req *model.BatchTenantDashboardMenuReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DashboardMenu.GetTenantDashboardMenus(userClaims, req.DashboardIDs)
	})
}

func (*DashboardMenuApi) SaveDashboardMenu(c *gin.Context) {
	HandlePathBody(c, "dashboardId", func(dashboardID string, req *model.UpsertTenantDashboardMenuReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DashboardMenu.UpsertTenantDashboardMenu(userClaims, dashboardID, req)
	})
}

func (*DashboardMenuApi) DeleteDashboardMenu(c *gin.Context) {
	HandlePathAction(c, "dashboardId", func(dashboardID string, userClaims *utils.UserClaims) error {
		return service.GroupApp.DashboardMenu.DeleteTenantDashboardMenu(userClaims, dashboardID)
	})
}
