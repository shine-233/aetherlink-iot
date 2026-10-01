package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// CreateDeviceGroup creates a tenant-scoped device group.
// @Router   /api/v1/device/group [post]
func (*DeviceApi) CreateDeviceGroup(c *gin.Context) {
	HandleAction(c, func(req *model.CreateDeviceGroupReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.DeviceGroup.CreateDeviceGroup(*req, userClaims)
	})
}

// DeleteDeviceGroup removes a device group.
// @Router   /api/v1/device/group/{id} [delete]
func (*DeviceApi) DeleteDeviceGroup(c *gin.Context) {
	HandlePathAction(c, "id", func(id string, userClaims *utils.UserClaims) error {
		return service.GroupApp.DeviceGroup.DeleteDeviceGroup(id, userClaims)
	})
}

// UpdateDeviceGroup updates a device group.
// @Router   /api/v1/device/group [put]
func (*DeviceApi) UpdateDeviceGroup(c *gin.Context) {
	HandleAction(c, func(req *model.UpdateDeviceGroupReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.DeviceGroup.UpdateDeviceGroup(*req, userClaims)
	})
}

// HandleDeviceGroupByPage returns paginated device groups.
// @Router   /api/v1/device/group [get]
func (*DeviceApi) HandleDeviceGroupByPage(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceGroupsListByPageReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceGroup.GetDeviceGroupListByPage(*req, userClaims)
	})
}

// HandleDeviceGroupByTree returns the device-group tree.
// @Router   /api/v1/device/group/tree [get]
func (*DeviceApi) HandleDeviceGroupByTree(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceGroup.GetDeviceGroupByTree(userClaims)
	})
}

// HandleDeviceGroupByDetail returns one device-group detail record.
// @Router   /api/v1/device/group/detail/{id} [get]
func (*DeviceApi) HandleDeviceGroupByDetail(c *gin.Context) {
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceGroup.GetDeviceGroupDetail(id, userClaims)
	})
}

// CreateDeviceGroupRelation creates a device-to-group relation.
// @Router   /api/v1/device/group/relation [post]
func (*DeviceApi) CreateDeviceGroupRelation(c *gin.Context) {
	HandleAction(c, func(req *model.CreateDeviceGroupRelationReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.DeviceGroup.CreateDeviceGroupRelation(*req, userClaims)
	})
}

// DeleteDeviceGroupRelation removes a device-to-group relation.
// @Router   /api/v1/device/group/relation [delete]
func (*DeviceApi) DeleteDeviceGroupRelation(c *gin.Context) {
	HandleAction(c, func(req *model.DeleteDeviceGroupRelationReq, userClaims *utils.UserClaims) error {
		return service.GroupApp.DeviceGroup.DeleteDeviceGroupRelation(req.GroupId, req.DeviceId, userClaims)
	})
}

// HandleDeviceGroupRelation returns devices from a group relation query.
// @Router   /api/v1/device/group/relation/list [get]
func (*DeviceApi) HandleDeviceGroupRelation(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceListByGroup, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceGroup.GetDeviceGroupRelation(*req, userClaims)
	})
}

// HandleDeviceGroupListByDeviceId returns groups for one device.
// @Router   /api/v1/device/group/relation [get]
func (*DeviceApi) HandleDeviceGroupListByDeviceId(c *gin.Context) {
	Handle(c, func(req *model.GetDeviceGroupListByDeviceIdReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceGroup.GetDeviceGroupByDeviceId(req.DeviceId, userClaims)
	})
}
