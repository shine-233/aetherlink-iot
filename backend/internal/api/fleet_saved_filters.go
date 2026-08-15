package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type FleetSavedFilterApi struct{}

func (*FleetSavedFilterApi) CreateFleetSavedFilter(c *gin.Context) {
	var req model.FleetSavedFilterReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.FleetSavedFilter.Create(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*FleetSavedFilterApi) ListFleetSavedFilters(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.FleetSavedFilter.List(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*FleetSavedFilterApi) UpdateFleetSavedFilter(c *gin.Context) {
	var req model.FleetSavedFilterReq
	if !BindAndValidate(c, &req) {
		return
	}
	req.ID = c.Param("filter_id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.FleetSavedFilter.Update(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

func (*FleetSavedFilterApi) DeleteFleetSavedFilter(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.FleetSavedFilter.Delete(c.Param("filter_id"), userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}
