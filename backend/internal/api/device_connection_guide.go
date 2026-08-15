package api

import (
	"strconv"

	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// GetDeviceConnectionGuide returns a first-connection guide assembled from
// existing connection, diagnostics, twin, and command evidence.
// @Summary Get device onboarding connection guide
// @Description Returns a first-connection guide assembled from existing connection profile, diagnostics, twin, and command evidence to help operators onboard a device.
// @Tags DeviceOnboarding
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_id path string true "Device ID"
// @Param debug_log_limit query int false "Optional limit on the number of debug log entries included"
// @Param command_log_limit query int false "Optional limit on the number of command log entries included"
// @Success 200 {object} model.DeviceConnectionGuideResp "Connection guide payload"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/device/{device_id}/onboarding/connection-guide [get]
func (*DeviceApi) GetDeviceConnectionGuide(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "device_id is required"))
		return
	}

	debugLogLimit, ok := parseOptionalInt64Query(c, "debug_log_limit")
	if !ok {
		return
	}
	commandLogLimit, ok := parseOptionalIntQuery(c, "command_log_limit")
	if !ok {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Device.GetConnectionGuide(c.Request.Context(), service.DeviceConnectionGuideReq{
		DeviceID:        deviceID,
		DebugLogLimit:   debugLogLimit,
		CommandLogLimit: commandLogLimit,
	}, deviceConnectLanguage(c), userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

func parseOptionalInt64Query(c *gin.Context, key string) (int64, bool) {
	rawValue := c.Query(key)
	if rawValue == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(rawValue, 10, 64)
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeParamError, key+" must be an integer"))
		return 0, false
	}
	return parsed, true
}

func parseOptionalIntQuery(c *gin.Context, key string) (int, bool) {
	rawValue := c.Query(key)
	if rawValue == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(rawValue)
	if err != nil {
		c.Error(errcode.WithData(errcode.CodeParamError, key+" must be an integer"))
		return 0, false
	}
	return parsed, true
}
