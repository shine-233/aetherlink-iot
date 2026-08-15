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
	var req model.CreateDeviceGroupReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceGroup.CreateDeviceGroup(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// DeleteDeviceGroup removes a device group.
// @Router   /api/v1/device/group/{id} [delete]
func (*DeviceApi) DeleteDeviceGroup(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceGroup.DeleteDeviceGroup(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// UpdateDeviceGroup updates a device group.
// @Router   /api/v1/device/group [put]
func (*DeviceApi) UpdateDeviceGroup(c *gin.Context) {
	var req model.UpdateDeviceGroupReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceGroup.UpdateDeviceGroup(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleDeviceGroupByPage returns paginated device groups.
// @Router   /api/v1/device/group [get]
func (*DeviceApi) HandleDeviceGroupByPage(c *gin.Context) {
	var req model.GetDeviceGroupsListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceGroup.GetDeviceGroupListByPage(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleDeviceGroupByTree returns the device-group tree.
// @Router   /api/v1/device/group/tree [get]
func (*DeviceApi) HandleDeviceGroupByTree(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceGroup.GetDeviceGroupByTree(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleDeviceGroupByDetail returns one device-group detail record.
// @Router   /api/v1/device/group/detail/{id} [get]
func (*DeviceApi) HandleDeviceGroupByDetail(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceGroup.GetDeviceGroupDetail(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateDeviceGroupRelation creates a device-to-group relation.
// @Router   /api/v1/device/group/relation [post]
func (*DeviceApi) CreateDeviceGroupRelation(c *gin.Context) {
	var req model.CreateDeviceGroupRelationReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceGroup.CreateDeviceGroupRelation(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// DeleteDeviceGroupRelation removes a device-to-group relation.
// @Router   /api/v1/device/group/relation [delete]
func (*DeviceApi) DeleteDeviceGroupRelation(c *gin.Context) {
	var req model.DeleteDeviceGroupRelationReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.DeviceGroup.DeleteDeviceGroupRelation(req.GroupId, req.DeviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleDeviceGroupRelation returns devices from a group relation query.
// @Router   /api/v1/device/group/relation/list [get]
func (*DeviceApi) HandleDeviceGroupRelation(c *gin.Context) {
	var req model.GetDeviceListByGroup
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceGroup.GetDeviceGroupRelation(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleDeviceGroupListByDeviceId returns groups for one device.
// @Router   /api/v1/device/group/relation [get]
func (*DeviceApi) HandleDeviceGroupListByDeviceId(c *gin.Context) {
	var req model.GetDeviceGroupListByDeviceIdReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.DeviceGroup.GetDeviceGroupByDeviceId(req.DeviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
