package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"
)

func (*DeviceModel) GetDeviceModelListByPageGeneral(req model.GetDeviceModelListByPageReq, what string, claims *utils.UserClaims) (interface{}, error) {

	// 自上而下租户作用域（self∪子孙）：总部/父级管理员可下钻查看子租户模板的物模型定义。
	scopes := expandTenantIDScope(claims.TenantID)
	listRsp := make(map[string]interface{})
	switch what {
	case model.DEVICE_MODEL_TELEMETRY:
		count, data, err := dal.GetDeviceModelTelemetryListByPage(req, scopes)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		listRsp["total"] = count
		listRsp["list"] = data
		return listRsp, nil
	case model.DEVICE_MODEL_ATTRIBUTES:
		count, data, err := dal.GetDeviceModelAttributesListByPage(req, scopes)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		listRsp["total"] = count
		listRsp["list"] = data
		return listRsp, nil
	case model.DEVICE_MODEL_EVENTS:
		count, data, err := dal.GetDeviceModelEventsListByPage(req, scopes)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		listRsp["total"] = count
		listRsp["list"] = data
		return listRsp, nil
	case model.DEVICE_MODEL_COMMANDS:
		count, data, err := dal.GetDeviceModelCommandsListByPage(req, scopes)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		listRsp["total"] = count
		listRsp["list"] = data
		return listRsp, nil
	default:
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param_err": "DEVICE_MODEL is not a valid type",
		})
	}
}

func (*DeviceModel) GetModelSourceAT(ctx context.Context, param *model.ParamID, claims *utils.UserClaims) ([]model.GetModelSourceATRes, error) {
	if _, err := ensureDeviceTemplateReadAccess(param.ID, claims); err != nil {
		return nil, err
	}
	var (
		res = make([]model.GetModelSourceATRes, 0)
	)

	resInfo := model.GetModelSourceATRes{
		DataSourceTypeRes: string(constant.TelemetrySource),
		Options:           make([]*model.Options, 0),
	}

	telemetryList, err := dal.DeviceModelTelemetryQuery{}.Find(ctx, query.DeviceModelTelemetry.DeviceTemplateID.Eq(param.ID))
	if err != nil {
		logrus.Error(ctx, "[GetModelSourceAT]telemetryList failed:", err)
	}

	for _, telemetry := range telemetryList {
		info := &model.Options{
			Key:      telemetry.DataIdentifier,
			Label:    telemetry.DataName,
			DataType: telemetry.DataType,
		}
		if info.DataType != nil && *info.DataType == "Enum" {
			json.Unmarshal([]byte(*telemetry.AdditionalInfo), &info.Enum)
		}
		resInfo.Options = append(resInfo.Options, info)
	}
	res = append(res, resInfo)

	resInfo = model.GetModelSourceATRes{
		DataSourceTypeRes: string(constant.AttributeSource),
		Options:           make([]*model.Options, 0),
	}
	attributeList, err := dal.DeviceModelAttributeQuery{}.Find(ctx, query.DeviceModelAttribute.DeviceTemplateID.Eq(param.ID))
	if err != nil {
		logrus.Error(ctx, "[GetModelSourceAT]attributeList failed:", err)
	}

	for _, attribute := range attributeList {
		info := &model.Options{
			Key:      attribute.DataIdentifier,
			Label:    attribute.DataName,
			DataType: attribute.DataType,
		}
		if info.DataType != nil && *info.DataType == "Enum" {
			json.Unmarshal([]byte(*attribute.AdditionalInfo), &info.Enum)
		}
		resInfo.Options = append(resInfo.Options, info)
	}

	res = append(res, resInfo)
	return res, err
}
