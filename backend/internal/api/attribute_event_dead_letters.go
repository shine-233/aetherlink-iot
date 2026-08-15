package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ServeAttributeEventDeadLetterList returns metadata-only attribute/event
// storage failures. Canonical raw payload is deliberately unavailable here.
func (*TelemetryDataApi) ServeAttributeEventDeadLetterList(c *gin.Context) {
	var req model.GetAttributeEventDeadLetterListReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.TelemetryData.GetAttributeEventDeadLetterList(
		c.Request.Context(),
		&req,
		claims,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateAttributeEventDeadLetterStatus retries, replays, resolves or ignores
// one row through the storage-owned claim/fencing seam.
func (*TelemetryDataApi) UpdateAttributeEventDeadLetterStatus(c *gin.Context) {
	var req model.UpdateAttributeEventDeadLetterStatusReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.TelemetryData.UpdateAttributeEventDeadLetterStatus(
		c.Request.Context(),
		c.Param("id"),
		&req,
		claims,
	); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// DrainAttributeEventDeadLetters replays one bounded, permission-scoped batch.
func (*TelemetryDataApi) DrainAttributeEventDeadLetters(c *gin.Context) {
	var req model.DrainAttributeEventDeadLetterReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.TelemetryData.DrainAttributeEventDeadLetters(
		c.Request.Context(),
		&req,
		claims,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
