package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceTwinApi struct{}

// HandleDeviceTwin returns a read-only desired-vs-reported twin aggregation for
// one device. It is intentionally thin and delegates permission checks plus data
// shaping to the service layer.
func (*DeviceTwinApi) HandleDeviceTwin(c *gin.Context) {
	deviceID := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.DeviceTwin.GetDeviceTwin(deviceID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleDeviceTwinDriftIndex returns a read-only fleet-level drift index that
// enumerates a bounded set of tenant devices, reuses the single-device twin
// classification, and aggregates it into a severity-ranked queryable index.
func (*DeviceTwinApi) HandleDeviceTwinDriftIndex(c *gin.Context) {
	var req model.DeviceTwinDriftIndexReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.DeviceTwin.GetDeviceTwinDriftIndex(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

func (*DeviceTwinApi) UpsertDeviceTwinDesired(c *gin.Context) {
	deviceID := c.Param("id")
	var req model.UpsertDeviceTwinDesiredReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	data, err := service.GroupApp.DeviceTwin.UpsertDesired(deviceID, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}
