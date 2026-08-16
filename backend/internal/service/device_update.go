package service

import (
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	protocolplugin "aetherlink-iot/backend/internal/service/protocol_plugin"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type updateDeviceContext struct {
	oldDevice             *model.Device
	disconnectAfterUpdate bool
	condsMap              map[string]interface{}
}

// UpdateDevice updates device metadata and applies required runtime disconnect side effects.
func (*Device) UpdateDevice(req model.UpdateDeviceReq, claims *utils.UserClaims) (*model.Device, error) {
	req, updateContext, err := buildUpdateDeviceContext(req, claims)
	if err != nil {
		return nil, err
	}

	device, err := dal.UpdateDeviceByMap(req.Id, updateContext.condsMap)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	applyUpdateDevicePostUpdateEffects(req, updateContext.oldDevice, updateContext.disconnectAfterUpdate)

	return device, err
}

func buildUpdateDeviceContext(req model.UpdateDeviceReq, claims *utils.UserClaims) (model.UpdateDeviceReq, *updateDeviceContext, error) {
	var err error
	req, err = normalizeUpdateDeviceInput(req)
	if err != nil {
		return req, nil, err
	}

	oldDevice, err := getUpdateDeviceTarget(&req)
	if err != nil {
		return req, nil, err
	}

	if err := authorizeUpdateDevice(req, oldDevice, claims); err != nil {
		return req, nil, err
	}

	disconnectAfterUpdate, err := validateUpdateDeviceNumberChange(req, oldDevice)
	if err != nil {
		return req, nil, err
	}

	condsMap, err := buildUpdateDeviceFields(req, time.Now().UTC())
	if err != nil {
		return req, nil, err
	}

	return req, &updateDeviceContext{
		oldDevice:             oldDevice,
		disconnectAfterUpdate: disconnectAfterUpdate,
		condsMap:              condsMap,
	}, nil
}

// normalizeUpdateDeviceInput trims and normalizes optional update fields before validation.
func normalizeUpdateDeviceInput(req model.UpdateDeviceReq) (model.UpdateDeviceReq, error) {
	if err := normalizeOptionalRDIDeviceNumber(&req.DeviceNumber, req.PIDNumber); err != nil {
		return req, err
	}
	return req, nil
}

// getUpdateDeviceTarget loads the existing device targeted by an update request.
func getUpdateDeviceTarget(req *model.UpdateDeviceReq) (*model.Device, error) {
	// "EMPTY" is a compatibility sentinel from older callers, not a normal missing-ID state.
	if req.Id != "EMPTY" {
		oldDevice, err := dal.GetDeviceByID(req.Id)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		return oldDevice, nil
	}

	if req.DeviceNumber == nil || *req.DeviceNumber == "" {
		return nil, errcode.New(204003)
	}

	oldDevice, err := dal.GetDeviceByDeviceNumber(*req.DeviceNumber)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if oldDevice == nil {
		return nil, errcode.New(204003)
	}
	req.Id = oldDevice.ID
	return oldDevice, nil
}

func authorizeUpdateDevice(req model.UpdateDeviceReq, oldDevice *model.Device, claims *utils.UserClaims) error {
	if _, err := ensureTelemetryDeviceWriteAccess(req.Id, claims); err != nil {
		return err
	}
	if req.DeviceConfigId == nil || *req.DeviceConfigId == "" {
		return nil
	}

	_, err := ensureWritableDeviceConfigForTenant(
		*req.DeviceConfigId,
		claims,
		oldDevice.TenantID,
		"device and device config tenant mismatch",
	)
	return err
}

func validateUpdateDeviceNumberChange(req model.UpdateDeviceReq, oldDevice *model.Device) (bool, error) {
	if req.DeviceNumber == nil || *req.DeviceNumber == "" || oldDevice.DeviceNumber == *req.DeviceNumber {
		return false, nil
	}

	if err := ensureUpdatedDeviceNumberAvailable(*req.DeviceNumber); err != nil {
		return false, err
	}

	return updateDeviceNumberChangeRequiresDisconnect(req, oldDevice)
}

func ensureUpdatedDeviceNumberAvailable(deviceNumber string) error {
	// Device numbers must stay unique when a device is renamed or rebound.
	exists, err := dal.CheckDeviceNumberExists(deviceNumber)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"message":   "check device number exists failed",
		})
	}
	if exists {
		return errcode.New(204004)
	}
	return nil
}

func updateDeviceNumberChangeRequiresDisconnect(req model.UpdateDeviceReq, oldDevice *model.Device) (bool, error) {
	deviceConfigID, ok := resolveUpdateDeviceConfigID(req, oldDevice)
	if !ok {
		return false, nil
	}

	deviceConfig, err := loadUpdateDeviceConfigForDisconnect(deviceConfigID)
	if err != nil {
		return false, err
	}
	return shouldDisconnectAfterUpdateDeviceNumberChange(deviceConfig), nil
}

func resolveUpdateDeviceConfigID(req model.UpdateDeviceReq, oldDevice *model.Device) (string, bool) {
	if req.DeviceConfigId != nil {
		return *req.DeviceConfigId, true
	}
	if oldDevice.DeviceConfigID != nil {
		return *oldDevice.DeviceConfigID, true
	}
	return "", false
}

func loadUpdateDeviceConfigForDisconnect(deviceConfigID string) (*model.DeviceConfig, error) {
	deviceConfig, err := dal.GetDeviceConfigByID(deviceConfigID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return deviceConfig, nil
}

func shouldDisconnectAfterUpdateDeviceNumberChange(deviceConfig *model.DeviceConfig) bool {
	if deviceConfig.ProtocolType == nil || *deviceConfig.ProtocolType == "MQTT" {
		return false
	}

	// MQTT-backed devices must reconnect after number changes so broker identity stays aligned.
	return deviceConfig.DeviceType == "1" || deviceConfig.DeviceType == "2"
}

func buildUpdateDeviceFields(req model.UpdateDeviceReq, updatedAt time.Time) (map[string]interface{}, error) {
	condsMap, err := StructToMapAndVerifyJson(req, "additional_info", "protocol_config")
	if err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	condsMap["update_at"] = updatedAt
	return condsMap, nil
}

func applyUpdateDevicePostUpdateEffects(req model.UpdateDeviceReq, oldDevice *model.Device, disconnectAfterUpdate bool) {
	// Parent-gateway routing changes require the old child session to be disconnected.
	if updateDeviceSubDeviceAddrChanged(req, oldDevice) {
		disconnectAfterUpdate = true
	}
	if disconnectAfterUpdate {
		if disconnectErr := protocolplugin.DisconnectDeviceByDeviceID(req.Id); disconnectErr != nil {
			logrus.Error("DisconnectDeviceByDeviceID failed")
		}
	}
}

func updateDeviceSubDeviceAddrChanged(req model.UpdateDeviceReq, oldDevice *model.Device) bool {
	if req.SubDeviceAddr == nil || *req.SubDeviceAddr == "" {
		return false
	}
	if oldDevice.SubDeviceAddr == nil || *oldDevice.SubDeviceAddr == "" {
		return false
	}
	return *oldDevice.SubDeviceAddr != *req.SubDeviceAddr
}
