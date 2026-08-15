package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
	"fmt"
	"time"

	"github.com/go-basic/uuid"
)

type deviceModelGeneralV2Base struct {
	ID               string
	DeviceTemplateID string
	DataName         *string
	DataIdentifier   string
	Param            *string
	Description      *string
	AdditionalInfo   *string
	Remark           *string
	TenantID         string
	UpdatedAt        time.Time
	CreatedAt        time.Time
}

func validateDeviceModelGeneralV2JSON(additionalInfo *string, params *string, wrapped bool) error {
	if additionalInfo != nil && !IsJSON(*additionalInfo) {
		if wrapped {
			return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"param_err": "additional_info is not a valid JSON",
			})
		}
		return fmt.Errorf("additional_info is not a valid JSON")
	}

	if params != nil && !IsJSON(*params) {
		if wrapped {
			return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"param_err": "params is not a valid JSON",
			})
		}
		return fmt.Errorf("params is not a valid JSON")
	}

	return nil
}

func (*DeviceModel) CreateDeviceModelGeneralV2(req model.CreateDeviceModelV2Req, what string, claims *utils.UserClaims) (interface{}, error) {
	deviceTemplate, err := ensureDeviceTemplateWriteAccess(req.DeviceTemplateId, claims)
	if err != nil {
		return nil, err
	}

	if err := validateDeviceModelGeneralV2JSON(req.AdditionalInfo, req.Params, false); err != nil {
		return nil, err
	}

	return createDeviceModelGeneralV2Definition(req, what, deviceTemplate.TenantID)
}

func (*DeviceModel) UpdateDeviceModelGeneralV2(req model.UpdateDeviceModelV2Req, what string, claims *utils.UserClaims) (interface{}, error) {
	if err := validateDeviceModelGeneralV2JSON(req.AdditionalInfo, req.Params, true); err != nil {
		return nil, err
	}

	tenantID, err := ensureDeviceModelGeneralWriteAccess(req.ID, what, claims)
	if err != nil {
		return nil, err
	}

	return updateDeviceModelGeneralV2Definition(req, what, tenantID)
}

func newCreateDeviceModelGeneralV2Base(req model.CreateDeviceModelV2Req, tenantID string, now time.Time) deviceModelGeneralV2Base {
	return deviceModelGeneralV2Base{
		ID:               uuid.New(),
		DeviceTemplateID: req.DeviceTemplateId,
		DataName:         req.DataName,
		DataIdentifier:   req.DataIdentifier,
		Param:            req.Params,
		Description:      req.Description,
		AdditionalInfo:   req.AdditionalInfo,
		Remark:           req.Remark,
		TenantID:         tenantID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func newUpdateDeviceModelGeneralV2Base(req model.UpdateDeviceModelV2Req, tenantID string, now time.Time) deviceModelGeneralV2Base {
	return deviceModelGeneralV2Base{
		ID:             req.ID,
		DataName:       req.DataName,
		DataIdentifier: req.DataIdentifier,
		Param:          req.Params,
		Description:    req.Description,
		AdditionalInfo: req.AdditionalInfo,
		Remark:         req.Remark,
		TenantID:       tenantID,
		UpdatedAt:      now,
	}
}

func createDeviceModelGeneralV2Definition(req model.CreateDeviceModelV2Req, what string, tenantID string) (interface{}, error) {
	base := newCreateDeviceModelGeneralV2Base(req, tenantID, time.Now().UTC())

	switch what {
	case model.DEVICE_MODEL_EVENTS:
		deviceModel := model.DeviceModelEvent{
			ID:               base.ID,
			DeviceTemplateID: base.DeviceTemplateID,
			DataName:         base.DataName,
			DataIdentifier:   base.DataIdentifier,
			Param:            base.Param,
			Description:      base.Description,
			AdditionalInfo:   base.AdditionalInfo,
			CreatedAt:        base.CreatedAt,
			UpdatedAt:        base.UpdatedAt,
			Remark:           base.Remark,
			TenantID:         base.TenantID,
		}
		if err := dal.CreateDeviceModelEvent(&deviceModel); err != nil {
			return nil, err
		}
		return deviceModel, nil
	case model.DEVICE_MODEL_COMMANDS:
		deviceModel := model.DeviceModelCommand{
			ID:               base.ID,
			DeviceTemplateID: base.DeviceTemplateID,
			DataName:         base.DataName,
			DataIdentifier:   base.DataIdentifier,
			Param:            base.Param,
			Description:      base.Description,
			AdditionalInfo:   base.AdditionalInfo,
			CreatedAt:        base.CreatedAt,
			UpdatedAt:        base.UpdatedAt,
			Remark:           base.Remark,
			TenantID:         base.TenantID,
		}
		if err := dal.CreateDeviceModelCommand(&deviceModel); err != nil {
			return nil, err
		}
		return deviceModel, nil
	default:
		return nil, fmt.Errorf("不支持的创建类型")
	}
}

func updateDeviceModelGeneralV2Definition(req model.UpdateDeviceModelV2Req, what string, tenantID string) (interface{}, error) {
	base := newUpdateDeviceModelGeneralV2Base(req, tenantID, time.Now().UTC())

	switch what {
	case model.DEVICE_MODEL_EVENTS:
		deviceModel := model.DeviceModelEvent{
			ID:             base.ID,
			DataName:       base.DataName,
			DataIdentifier: base.DataIdentifier,
			Param:          base.Param,
			Description:    base.Description,
			AdditionalInfo: base.AdditionalInfo,
			UpdatedAt:      base.UpdatedAt,
			Remark:         base.Remark,
			TenantID:       base.TenantID,
		}
		if err := dal.UpdateDeviceModelEvent(&deviceModel); err != nil {
			return nil, wrapDeviceModelDBError(err)
		}
		return deviceModel, nil
	case model.DEVICE_MODEL_COMMANDS:
		deviceModel := model.DeviceModelCommand{
			ID:             base.ID,
			DataName:       base.DataName,
			DataIdentifier: base.DataIdentifier,
			Param:          base.Param,
			Description:    base.Description,
			AdditionalInfo: base.AdditionalInfo,
			UpdatedAt:      base.UpdatedAt,
			Remark:         base.Remark,
			TenantID:       base.TenantID,
		}
		if err := dal.UpdateDeviceModelCommand(&deviceModel); err != nil {
			return nil, wrapDeviceModelDBError(err)
		}
		return deviceModel, nil
	default:
		return nil, invalidDeviceModelTypeError()
	}
}
