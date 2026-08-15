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

type createDeviceModelGeneralBase struct {
	ID               string
	DeviceTemplateID string
	DataName         *string
	DataIdentifier   string
	Description      *string
	AdditionalInfo   *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Remark           *string
	TenantID         string
}

func validateCreateDeviceModelGeneralAccessAndParams(req model.CreateDeviceModelReq, what string, claims *utils.UserClaims) (string, error) {
	deviceTemplate, err := ensureDeviceTemplateWriteAccess(req.DeviceTemplateId, claims)
	if err != nil {
		return "", err
	}

	if req.AdditionalInfo != nil && !IsJSON(*req.AdditionalInfo) {
		return "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "additional_info is not a valid JSON",
		})
	}

	if err := ensureCreateDeviceModelGeneralDataIdentifierAvailable(req, what, claims.TenantID); err != nil {
		return "", err
	}

	return deviceTemplate.TenantID, nil
}

func ensureCreateDeviceModelGeneralDataIdentifierAvailable(req model.CreateDeviceModelReq, what string, tenantID string) error {
	var exists bool
	var err error

	switch what {
	case model.DEVICE_MODEL_TELEMETRY:
		exists, err = dal.CheckTelemetryDataIdentifierExists(req.DeviceTemplateId, tenantID, req.DataIdentifier)
	case model.DEVICE_MODEL_ATTRIBUTES:
		exists, err = dal.CheckAttributeDataIdentifierExists(req.DeviceTemplateId, tenantID, req.DataIdentifier)
	case model.DEVICE_MODEL_EVENTS:
		exists, err = dal.CheckEventDataIdentifierExists(req.DeviceTemplateId, tenantID, req.DataIdentifier)
	case model.DEVICE_MODEL_COMMANDS:
		exists, err = dal.CheckCommandDataIdentifierExists(req.DeviceTemplateId, tenantID, req.DataIdentifier)
	default:
		return invalidDeviceModelTypeError()
	}

	if err != nil {
		return wrapDeviceModelDBError(err)
	}
	if exists {
		return errcode.WithData(208001, map[string]interface{}{
			"param_err": fmt.Sprintf("data identifier '%s' already exists", req.DataIdentifier),
		})
	}
	return nil
}

func newCreateDeviceModelGeneralBase(req model.CreateDeviceModelReq, tenantID string, now time.Time) createDeviceModelGeneralBase {
	return createDeviceModelGeneralBase{
		ID:               uuid.New(),
		DeviceTemplateID: req.DeviceTemplateId,
		DataName:         req.DataName,
		DataIdentifier:   req.DataIdentifier,
		Description:      req.Description,
		AdditionalInfo:   req.AdditionalInfo,
		CreatedAt:        now,
		UpdatedAt:        now,
		Remark:           req.Remark,
		TenantID:         tenantID,
	}
}

func createDeviceModelGeneralDefinition(req model.CreateDeviceModelReq, what string, tenantID string) (interface{}, error) {
	base := newCreateDeviceModelGeneralBase(req, tenantID, time.Now().UTC())

	switch what {
	case model.DEVICE_MODEL_TELEMETRY:
		deviceModel, err := createDeviceModelTelemetryDefinition(req, base)
		if err != nil {
			return nil, err
		}
		return deviceModel, nil
	case model.DEVICE_MODEL_ATTRIBUTES:
		deviceModel, err := createDeviceModelAttributeDefinition(req, base)
		if err != nil {
			return nil, err
		}
		return deviceModel, nil
	case model.DEVICE_MODEL_EVENTS:
		deviceModel, err := createDeviceModelEventDefinition(base)
		if err != nil {
			return nil, err
		}
		return deviceModel, nil
	case model.DEVICE_MODEL_COMMANDS:
		deviceModel, err := createDeviceModelCommandDefinition(base)
		if err != nil {
			return nil, err
		}
		return deviceModel, nil
	default:
		return nil, invalidDeviceModelTypeError()
	}
}

func createDeviceModelTelemetryDefinition(req model.CreateDeviceModelReq, base createDeviceModelGeneralBase) (model.DeviceModelTelemetry, error) {
	deviceModel := model.DeviceModelTelemetry{
		ID:               base.ID,
		DeviceTemplateID: base.DeviceTemplateID,
		DataName:         base.DataName,
		DataIdentifier:   base.DataIdentifier,
		ReadWriteFlag:    req.ReadWriteFlag,
		DataType:         req.DataType,
		Unit:             req.Unit,
		Description:      base.Description,
		AdditionalInfo:   base.AdditionalInfo,
		CreatedAt:        base.CreatedAt,
		UpdatedAt:        base.UpdatedAt,
		Remark:           base.Remark,
		TenantID:         base.TenantID,
	}
	if err := dal.CreateDeviceModelTelemetry(&deviceModel); err != nil {
		return model.DeviceModelTelemetry{}, wrapDeviceModelDBError(err)
	}
	return deviceModel, nil
}

func createDeviceModelAttributeDefinition(req model.CreateDeviceModelReq, base createDeviceModelGeneralBase) (model.DeviceModelAttribute, error) {
	deviceModel := model.DeviceModelAttribute{
		ID:               base.ID,
		DeviceTemplateID: base.DeviceTemplateID,
		DataName:         base.DataName,
		DataIdentifier:   base.DataIdentifier,
		ReadWriteFlag:    req.ReadWriteFlag,
		DataType:         req.DataType,
		Unit:             req.Unit,
		Description:      base.Description,
		AdditionalInfo:   base.AdditionalInfo,
		CreatedAt:        base.CreatedAt,
		UpdatedAt:        base.UpdatedAt,
		Remark:           base.Remark,
		TenantID:         base.TenantID,
	}
	if err := dal.CreateDeviceModelAttribute(&deviceModel); err != nil {
		return model.DeviceModelAttribute{}, wrapDeviceModelDBError(err)
	}
	return deviceModel, nil
}

func createDeviceModelEventDefinition(base createDeviceModelGeneralBase) (model.DeviceModelEvent, error) {
	deviceModel := model.DeviceModelEvent{
		ID:               base.ID,
		DeviceTemplateID: base.DeviceTemplateID,
		DataName:         base.DataName,
		DataIdentifier:   base.DataIdentifier,
		Description:      base.Description,
		AdditionalInfo:   base.AdditionalInfo,
		CreatedAt:        base.CreatedAt,
		UpdatedAt:        base.UpdatedAt,
		Remark:           base.Remark,
		TenantID:         base.TenantID,
	}
	if err := dal.CreateDeviceModelEvent(&deviceModel); err != nil {
		return model.DeviceModelEvent{}, wrapDeviceModelDBError(err)
	}
	return deviceModel, nil
}

func createDeviceModelCommandDefinition(base createDeviceModelGeneralBase) (model.DeviceModelCommand, error) {
	deviceModel := model.DeviceModelCommand{
		ID:               base.ID,
		DeviceTemplateID: base.DeviceTemplateID,
		DataName:         base.DataName,
		DataIdentifier:   base.DataIdentifier,
		Description:      base.Description,
		AdditionalInfo:   base.AdditionalInfo,
		CreatedAt:        base.CreatedAt,
		UpdatedAt:        base.UpdatedAt,
		Remark:           base.Remark,
		TenantID:         base.TenantID,
	}
	if err := dal.CreateDeviceModelCommand(&deviceModel); err != nil {
		return model.DeviceModelCommand{}, wrapDeviceModelDBError(err)
	}
	return deviceModel, nil
}

func (*DeviceModel) CreateDeviceModelGeneral(req model.CreateDeviceModelReq, what string, claims *utils.UserClaims) (interface{}, error) {
	tenantID, err := validateCreateDeviceModelGeneralAccessAndParams(req, what, claims)
	if err != nil {
		return nil, err
	}
	return createDeviceModelGeneralDefinition(req, what, tenantID)
}
