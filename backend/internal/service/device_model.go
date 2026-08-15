// device_model.go owns the shared device model service boundary.
//
// Create/update/list flows live in focused same-package files so this file can
// stay on shared access checks, deletion, and common errors.
package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

type DeviceModel struct{}

func ensureDeviceModelTenantWriteAccess(tenantID string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify device model")
	}
	if claims.Authority != constant.SYS_ADMIN && tenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify device model")
	}
	return nil
}

func ensureDeviceModelGeneralWriteAccess(id string, what string, claims *utils.UserClaims) (string, error) {
	tenantID, err := loadDeviceModelGeneralTenantID(id, what)
	if err != nil {
		return "", err
	}
	if err := ensureDeviceModelTenantWriteAccess(tenantID, claims); err != nil {
		return "", err
	}
	return tenantID, nil
}

func loadDeviceModelGeneralTenantID(id string, what string) (string, error) {
	switch what {
	case model.DEVICE_MODEL_TELEMETRY:
		data, err := query.DeviceModelTelemetry.Where(query.DeviceModelTelemetry.ID.Eq(id)).First()
		if err != nil {
			return "", wrapDeviceModelDBError(err)
		}
		return data.TenantID, nil
	case model.DEVICE_MODEL_ATTRIBUTES:
		data, err := query.DeviceModelAttribute.Where(query.DeviceModelAttribute.ID.Eq(id)).First()
		if err != nil {
			return "", wrapDeviceModelDBError(err)
		}
		return data.TenantID, nil
	case model.DEVICE_MODEL_EVENTS:
		data, err := query.DeviceModelEvent.Where(query.DeviceModelEvent.ID.Eq(id)).First()
		if err != nil {
			return "", wrapDeviceModelDBError(err)
		}
		return data.TenantID, nil
	case model.DEVICE_MODEL_COMMANDS:
		data, err := query.DeviceModelCommand.Where(query.DeviceModelCommand.ID.Eq(id)).First()
		if err != nil {
			return "", wrapDeviceModelDBError(err)
		}
		return data.TenantID, nil
	default:
		return "", invalidDeviceModelTypeError()
	}
}

func wrapDeviceModelDBError(err error) error {
	if err == nil {
		return nil
	}
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func invalidDeviceModelTypeError() error {
	return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
		"param_err": "DEVICE_MODEL is not a valid type",
	})
}

func (*DeviceModel) DeleteDeviceModelGeneral(id string, what string, claims *utils.UserClaims) (err error) {
	if _, err := ensureDeviceModelGeneralWriteAccess(id, what, claims); err != nil {
		return err
	}
	switch what {
	case model.DEVICE_MODEL_TELEMETRY:
		err = dal.DeleteDeviceModelTelemetry(id)
	case model.DEVICE_MODEL_ATTRIBUTES:
		err = dal.DeleteDeviceModelAttribute(id)
	case model.DEVICE_MODEL_EVENTS:
		err = dal.DeleteDeviceModelEvent(id)
	case model.DEVICE_MODEL_COMMANDS:
		err = dal.DeleteDeviceModelCommand(id)
	default:
		return invalidDeviceModelTypeError()
	}
	return err
}
