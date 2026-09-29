package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type FleetSavedFilterApi struct{}

func (*FleetSavedFilterApi) CreateFleetSavedFilter(c *gin.Context) {
	Handle(c, func(req *model.FleetSavedFilterReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.FleetSavedFilter.Create(req, userClaims)
	})
}

func (*FleetSavedFilterApi) ListFleetSavedFilters(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.FleetSavedFilter.List(userClaims)
	})
}

func (*FleetSavedFilterApi) UpdateFleetSavedFilter(c *gin.Context) {
	Handle(c, func(req *model.FleetSavedFilterReq, userClaims *utils.UserClaims) (interface{}, error) {
		req.ID = c.Param("filter_id")
		return service.GroupApp.FleetSavedFilter.Update(req, userClaims)
	})
}

func (*FleetSavedFilterApi) DeleteFleetSavedFilter(c *gin.Context) {
	HandleNoBodyAction(c, func(userClaims *utils.UserClaims) error {
		return service.GroupApp.FleetSavedFilter.Delete(c.Param("filter_id"), userClaims)
	})
}
