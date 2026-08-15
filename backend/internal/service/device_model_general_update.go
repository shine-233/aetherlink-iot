package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
	"time"
)

func (*DeviceModel) UpdateDeviceModelGeneral(req model.UpdateDeviceModelReq, what string, claims *utils.UserClaims) (interface{}, error) {

	if req.AdditionalInfo != nil && !IsJSON(*req.AdditionalInfo) {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "additional_info is not a valid JSON",
		})
	}

	tenantID, err := ensureDeviceModelGeneralWriteAccess(req.ID, what, claims)
	if err != nil {
		return nil, err
	}

	t := time.Now().UTC()

	switch what {
	case model.DEVICE_MODEL_TELEMETRY:
		var deviceModel model.DeviceModelTelemetry
		deviceModel.ID = req.ID
		deviceModel.DataName = req.DataName
		deviceModel.DataIdentifier = req.DataIdentifier
		deviceModel.ReadWriteFlag = req.ReadWriteFlag
		deviceModel.DataType = req.DataType
		deviceModel.Unit = req.Unit
		deviceModel.Description = req.Description
		deviceModel.AdditionalInfo = req.AdditionalInfo
		deviceModel.UpdatedAt = t
		deviceModel.Remark = req.Remark
		deviceModel.TenantID = tenantID
		err := dal.UpdateDeviceModelTelemetry(&deviceModel)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		} else {
			return deviceModel, nil
		}

	case model.DEVICE_MODEL_ATTRIBUTES:
		var deviceModel model.DeviceModelAttribute
		deviceModel.ID = req.ID
		deviceModel.DataName = req.DataName
		deviceModel.DataIdentifier = req.DataIdentifier
		deviceModel.ReadWriteFlag = req.ReadWriteFlag
		deviceModel.DataType = req.DataType
		deviceModel.Unit = req.Unit
		deviceModel.Description = req.Description
		deviceModel.AdditionalInfo = req.AdditionalInfo
		deviceModel.UpdatedAt = t
		deviceModel.Remark = req.Remark
		deviceModel.TenantID = tenantID
		err := dal.UpdateDeviceModelAttribute(&deviceModel)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		} else {
			return deviceModel, nil
		}
	default:
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "DEVICE_MODEL is not a valid type",
		})
	}
}
