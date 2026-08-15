package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

func (*Device) ActiveDevice(req model.ActiveDeviceReq, claims *utils.UserClaims) (any, error) {
	pid, device, err := resolveActivationDevice(req.DeviceNumber, claims)
	if err != nil {
		return nil, err
	}
	req.DeviceNumber = pid
	req.Name = strings.TrimSpace(req.Name)
	t := time.Now().UTC()
	prepareActiveDeviceUpdate(device, req.DeviceNumber, req.Name, t)
	device, err = persistActivatedDevice(device)
	if err != nil {
		return nil, err
	}
	// Return the activated device snapshot after persisting normalized fields.
	return device, nil
}

func resolveActivationDevice(deviceNumber string, claims *utils.UserClaims) (string, *model.Device, error) {
	pid, err := NormalizeRDIPID(deviceNumber)
	if err != nil {
		return "", nil, err
	}

	device, err := loadActivationDeviceByNumber(pid)
	if err != nil {
		return "", nil, err
	}
	if err := ensureActivationDeviceAccess(device, claims); err != nil {
		return "", nil, err
	}
	if err := ensureActivationDeviceInactive(device); err != nil {
		return "", nil, err
	}
	return pid, device, nil
}

func loadActivationDeviceByNumber(deviceNumber string) (*model.Device, error) {
	device, err := dal.GetDeviceByDeviceNumber(deviceNumber)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.WithVars(204001, map[string]interface{}{
				"error": deviceNumber,
			})
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return device, nil
}

func ensureActivationDeviceAccess(device *model.Device, claims *utils.UserClaims) error {
	if claims != nil && claims.Authority != constant.SYS_ADMIN && device.TenantID != claims.TenantID {
		return errcode.New(errcode.CodeNoPermission)
	}
	return nil
}

func ensureActivationDeviceInactive(device *model.Device) error {
	if device.ActivateFlag == "active" {
		return errcode.New(204002)
	}
	return nil
}

func persistActivatedDevice(device *model.Device) (*model.Device, error) {
	updatedDevice, err := dal.UpdateDevice(device)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return updatedDevice, nil
}

func prepareActiveDeviceUpdate(device *model.Device, deviceNumber, requestedName string, activatedAt time.Time) {
	device.DeviceNumber = deviceNumber
	if requestedName != "" {
		device.Name = &requestedName
	} else if device.Name == nil || strings.TrimSpace(*device.Name) == "" {
		device.Name = &deviceNumber
	}
	device.ActivateFlag = "active"
	device.IsEnabled = "enabled"
	device.UpdateAt = &activatedAt
	device.ActivateAt = &activatedAt
}

func (d *Device) CheckDeviceNumber(deviceNumber string, claims *utils.UserClaims) (*errcode.Error, bool) {
	_, _, err := resolveActivationDevice(deviceNumber, claims)
	if err != nil {
		if e, ok := err.(*errcode.Error); ok {
			return e, false
		}
		return errcode.NewWithMessage(errcode.CodeParamError, err.Error()), false
	}

	return errcode.WithVars(204003, nil), true
}
