package service

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	protocolplugin "aetherlink-iot/backend/internal/service/protocol_plugin"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// RemoveSubDevice detaches a child device from its parent and refreshes parent runtime state.
func (*Device) RemoveSubDevice(id string, claims *utils.UserClaims) error {
	device, err := ensureTelemetryDeviceWriteAccess(id, claims)
	if err != nil {
		return err
	}

	if err := removeSubDeviceBinding(id, device.TenantID); err != nil {
		return err
	}

	disconnectParentAfterSubDeviceRemoval(device)
	return nil
}

func removeSubDeviceBinding(id string, tenantID string) error {
	if err := dal.RemoveSubDevice(id, tenantID); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func disconnectParentAfterSubDeviceRemoval(device *model.Device) {
	if device == nil {
		return
	}
	if device.ParentID != nil {
		if disconnectErr := protocolplugin.DisconnectDeviceByDeviceID(*device.ParentID); disconnectErr != nil {
			logrus.Error(disconnectErr)
		}
	}
}

func (*Device) GetDeviceOnlineStatus(deviceID string, claims *utils.UserClaims) (map[string]int, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device online status")
	}
	deviceInfo, err := dal.GetDeviceByID(deviceID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device info failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	if !canReadDeviceOnlineStatus(deviceInfo, claims) {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device online status")
	}
	data := make(map[string]int)
	data["device_status"] = int(deviceInfo.IsOnline)
	data["is_online"] = data["device_status"]
	return data, nil
}

func (*Device) GetDeviceOnlineStatuses(deviceIDs []string, claims *utils.UserClaims) (map[string]map[string]int, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device online status")
	}

	devices, err := dal.GetDevicesByIDs(deviceIDs)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device info failed:" + err.Error(),
		})
	}

	result := make(map[string]map[string]int, len(devices))
	for _, deviceInfo := range devices {
		if deviceInfo == nil {
			continue
		}
		if !canReadDeviceOnlineStatus(deviceInfo, claims) {
			continue
		}
		deviceStatus := int(deviceInfo.IsOnline)
		result[deviceInfo.ID] = map[string]int{
			"device_status": deviceStatus,
			"is_online":     deviceStatus,
		}
	}
	return result, nil
}

func canReadDeviceOnlineStatus(deviceInfo *model.Device, claims *utils.UserClaims) bool {
	return hasTelemetryTenantAccess(deviceInfo, claims, true)
}
