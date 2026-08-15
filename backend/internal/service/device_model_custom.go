package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
	"fmt"

	"github.com/go-basic/uuid"
)

func ensureDeviceModelCustomCommandWriteAccess(id string, claims *utils.UserClaims) (*model.DeviceModelCustomCommand, error) {
	data, err := dal.GetDeviceModelCustomCommandById(id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if err := ensureDeviceModelTenantWriteAccess(data.TenantID, claims); err != nil {
		return nil, err
	}
	return data, nil
}

func ensureDeviceModelCustomControlWriteAccess(id string, claims *utils.UserClaims) (*model.DeviceModelCustomControl, error) {
	data, err := dal.GetDeviceModelCustomControlById(id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if err := ensureDeviceModelTenantWriteAccess(data.TenantID, claims); err != nil {
		return nil, err
	}
	return data, nil
}

func (*DeviceModel) CreateDeviceModelCustomCommands(req model.CreateDeviceModelCustomCommandReq, claims *utils.UserClaims) error {
	deviceTemplate, err := ensureDeviceTemplateWriteAccess(req.DeviceTemplateId, claims)
	if err != nil {
		return err
	}

	if req.EnableStatus != "enable" && req.EnableStatus != "disable" {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param": "enable_status",
			"value": req.EnableStatus,
			"valid": []string{"enable", "disable"},
		})
	}

	var deviceModelCustomCommand model.DeviceModelCustomCommand

	deviceModelCustomCommand.ID = uuid.New()
	deviceModelCustomCommand.DeviceTemplateID = req.DeviceTemplateId
	deviceModelCustomCommand.ButtomName = req.ButtomName
	deviceModelCustomCommand.DataIdentifier = req.DataIdentifier
	deviceModelCustomCommand.Description = req.Description
	deviceModelCustomCommand.Instruct = req.Instruct
	deviceModelCustomCommand.EnableStatus = req.EnableStatus
	deviceModelCustomCommand.Remark = req.Remark
	deviceModelCustomCommand.TenantID = deviceTemplate.TenantID

	err = dal.CreateDeviceModelCustomCommand(&deviceModelCustomCommand)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*DeviceModel) DeleteDeviceModelCustomCommands(id string, claims *utils.UserClaims) error {
	if _, err := ensureDeviceModelCustomCommandWriteAccess(id, claims); err != nil {
		return err
	}
	err := dal.DeleteDeviceModelCustomCommandById(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*DeviceModel) UpdateDeviceModelCustomCommands(req model.UpdateDeviceModelCustomCommandReq, claims *utils.UserClaims) error {
	if _, err := ensureDeviceModelCustomCommandWriteAccess(req.ID, claims); err != nil {
		return err
	}

	if req.EnableStatus != "enable" && req.EnableStatus != "disable" {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"param": "enable_status",
			"value": req.EnableStatus,
			"valid": []string{"enable", "disable"},
		})
	}

	var deviceModelCustomCommand model.DeviceModelCustomCommand

	deviceModelCustomCommand.ID = req.ID
	deviceModelCustomCommand.ButtomName = req.ButtomName
	deviceModelCustomCommand.DataIdentifier = req.DataIdentifier
	deviceModelCustomCommand.Description = req.Description
	deviceModelCustomCommand.Instruct = req.Instruct
	deviceModelCustomCommand.EnableStatus = req.EnableStatus
	deviceModelCustomCommand.Remark = req.Remark

	_, err := dal.UpdateDeviceModelCustomCommand(&deviceModelCustomCommand)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*DeviceModel) GetDeviceModelCustomCommandsByPage(req model.GetDeviceModelListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetDeviceModelCustomCommandsByPage(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	listRsp := make(map[string]interface{})
	listRsp["total"] = total
	listRsp["list"] = list

	return listRsp, nil
}

func (*DeviceModel) GetDeviceModelCustomCommandsByDeviceId(deviceId string, claims *utils.UserClaims) ([]*model.DeviceModelCustomCommand, error) {
	device, err := ensureTelemetryDeviceReadAccess(deviceId, claims)
	if err != nil {
		return nil, err
	}
	data, err := dal.GetDeviceModelCustomCommandsByDeviceId(deviceId, device.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}

func (*DeviceModel) CreateDeviceModelCustomControl(req model.CreateDeviceModelCustomControlReq, claims *utils.UserClaims) error {
	deviceTemplate, err := ensureDeviceTemplateWriteAccess(req.DeviceTemplateId, claims)
	if err != nil {
		return err
	}

	if req.EnableStatus != "enable" && req.EnableStatus != "disable" {
		return fmt.Errorf("enable status error")
	}

	var deviceModelCustomControl model.DeviceModelCustomControl

	deviceModelCustomControl.ID = uuid.New()
	deviceModelCustomControl.DeviceTemplateID = req.DeviceTemplateId
	deviceModelCustomControl.Name = req.Name
	deviceModelCustomControl.ControlType = req.ControlType
	deviceModelCustomControl.Description = req.Description
	deviceModelCustomControl.Content = req.Content
	deviceModelCustomControl.EnableStatus = req.EnableStatus
	deviceModelCustomControl.Remark = req.Remark
	deviceModelCustomControl.TenantID = deviceTemplate.TenantID

	err = dal.CreateDeviceModelCustomControl(&deviceModelCustomControl)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}

func (*DeviceModel) DeleteDeviceModelCustomControl(id string, claims *utils.UserClaims) error {
	if _, err := ensureDeviceModelCustomControlWriteAccess(id, claims); err != nil {
		return err
	}
	err := dal.DeleteDeviceModelCustomControlById(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}

func (*DeviceModel) UpdateDeviceModelCustomControl(req model.UpdateDeviceModelCustomControlReq, claims *utils.UserClaims) error {
	current, err := ensureDeviceModelCustomControlWriteAccess(req.ID, claims)
	if err != nil {
		return err
	}

	if req.EnableStatus != nil && *req.EnableStatus != "enable" && *req.EnableStatus != "disable" {
		return fmt.Errorf("enable status error")
	}

	deviceModelCustomControl := *current
	if req.DeviceTemplateId != nil {
		deviceModelCustomControl.DeviceTemplateID = *req.DeviceTemplateId
	}
	if req.Name != nil {
		deviceModelCustomControl.Name = *req.Name
	}
	if req.ControlType != nil {
		deviceModelCustomControl.ControlType = *req.ControlType
	}
	if req.Description != nil {
		deviceModelCustomControl.Description = req.Description
	}
	if req.Content != nil {
		deviceModelCustomControl.Content = req.Content
	}
	if req.EnableStatus != nil {
		deviceModelCustomControl.EnableStatus = *req.EnableStatus
	}
	if req.Remark != nil {
		deviceModelCustomControl.Remark = req.Remark
	}

	_, err = dal.UpdateDeviceModelCustomControl(&deviceModelCustomControl)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}

func (*DeviceModel) GetDeviceModelCustomControlByPage(req model.GetDeviceModelListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetDeviceModelCustomControlByPage(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	listRsp := make(map[string]interface{})
	listRsp["total"] = total
	listRsp["list"] = list

	return listRsp, err
}
