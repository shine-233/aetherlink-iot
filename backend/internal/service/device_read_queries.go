package service

import (
	"context"
	"encoding/json"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/diagnostics"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func (*Device) GetDeviceListByPage(req *model.GetDeviceListByPageReq, u *utils.UserClaims) (map[string]interface{}, error) {
	scopes, err := resolveDeviceListScopes(req, u)
	if err != nil {
		return nil, err
	}
	applyDeviceListOwnerFilterForClaims(req, u)
	total, list, err := dal.GetDeviceListByPageForScopes(req, scopes)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if len(list) > 0 {
		for i := range list {
			list[i].DeviceStatus = list[i].IsOnline
			list[i].PIDNumber = list[i].DeviceNumber
			list[i].SharedStatus = rdiDeviceSharedStatus(list[i].AdditionalInfo)
			if req.AllTenants {
				list[i].ScopeTenantID = list[i].TenantID
			}
			if req.IncludeRDISystemInfoSummary {
				summary := rdiSystemInfoSummaryFromAdditionalInfo(list[i].AdditionalInfo)
				list[i].RDISystemInfoSummary = &summary
			}
			if list[i].WarnStatus == "N" || list[i].WarnStatus == "" {
				list[i].WarnStatus = "N"
			} else {
				list[i].WarnStatus = "Y"
			}
		}
	}
	deviceListRsp := make(map[string]interface{})
	deviceListRsp["total"] = total
	deviceListRsp["list"] = list

	return deviceListRsp, err
}

func resolveDeviceListTenantScope(req *model.GetDeviceListByPageReq, claims *utils.UserClaims) (string, error) {
	if req == nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "device list request is required")
	}
	if err := requireSystemAdminAllTenantsScope(
		req.AllTenants,
		claims,
		"all-tenants device list is only available to system administrators",
	); err != nil {
		return "", err
	}
	if req.AllTenants {
		return "", nil
	}
	return requireDeviceTenantClaims(claims, "no permission to query device list")
}

// resolveDeviceListScopes 返回设备列表查询的层级作用域（self∪祖先）。
// 与旧 resolveDeviceListTenantScope 等价守卫：AllTenants 仅系统管理员可用，非全量时要求租户 claims。
func resolveDeviceListScopes(req *model.GetDeviceListByPageReq, claims *utils.UserClaims) ([]string, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device list request is required")
	}
	if err := requireSystemAdminAllTenantsScope(
		req.AllTenants,
		claims,
		"all-tenants device list is only available to system administrators",
	); err != nil {
		return nil, err
	}
	if req.AllTenants {
		return nil, nil
	}
	self, err := requireDeviceTenantClaims(claims, "no permission to query device list")
	if err != nil {
		return nil, err
	}
	return expandTenantIDScope(self), nil
}

func rdiSystemInfoSummaryFromAdditionalInfo(additionalInfo *string) model.RDISystemInfoSummary {
	info := systemInfoFromAdditionalInfo(parseAdditionalInfo(additionalInfo))
	return model.RDISystemInfoSummary{
		InstallationLocation:   info.InstallationLocation,
		Address:                info.Address,
		InstallationDate:       info.InstallationDate,
		InstallerCompany:       info.InstallerCompany,
		InstallerContact:       info.InstallerContact,
		InstallerName:          info.InstallerName,
		InstallerPhone:         info.InstallerPhone,
		InstallerEmail:         info.InstallerEmail,
		ControllerSerialNumber: info.ControllerSerialNumber,
		MaintenanceTechnician:  info.MaintenanceTechnician,
	}
}

func rdiDeviceSharedStatus(additionalInfo *string) string {
	if additionalInfo == nil || strings.TrimSpace(*additionalInfo) == "" {
		return "unshared"
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(*additionalInfo), &payload); err != nil {
		return "unshared"
	}

	recipientsRaw, ok := payload["rdi_share_recipients"]
	if !ok || len(recipientsRaw) == 0 {
		return "unshared"
	}

	var recipients []model.RDIShareRecipientRecord
	if err := json.Unmarshal(recipientsRaw, &recipients); err != nil || len(recipients) == 0 {
		return "unshared"
	}
	for _, recipient := range recipients {
		if strings.TrimSpace(recipient.UserID) != "" {
			return "shared"
		}
	}

	return "unshared"
}

func (*Device) GetDevicePreRegisterListByPage(req *model.GetDevicePreRegisterListByPageReq, u *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetDevicePreRegisterListByPage(req, u.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	deviceListRsp := make(map[string]interface{})
	deviceListRsp["total"] = total
	deviceListRsp["list"] = list

	return deviceListRsp, err
}

func (*Device) GetTenantDeviceList(req *model.GetDeviceMenuReq, userClaims *utils.UserClaims) ([]map[string]interface{}, error) {
	tenantID, err := requireDeviceTenantClaims(userClaims, "no permission to query device menu")
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = deviceOwnerUserIDFilterForClaims(userClaims)

	var data []map[string]interface{}

	if req.GroupId != "" {
		// Group filtering narrows the tenant device menu when a group id is supplied.
		data, err = dal.GetDeviceSelectByGroupId(tenantID, req.GroupId, req.DeviceName, req.BindConfig, req.OwnerUserID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
	} else {
		data, err = dal.DeviceQuery{}.GetDeviceSelect(tenantID, req.DeviceName, req.BindConfig, req.OwnerUserID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
	}

	if data == nil {
		data = []map[string]interface{}{}
	}
	return data, nil
}

// GetSubList returns the child-device page for a parent device.
func (*Device) GetSubList(ctx context.Context, parent_id string, page, pageSize int64, userClaims *utils.UserClaims) ([]model.GetSubListResp, int64, error) {
	parentDevice, err := ensureTelemetryDeviceReadAccess(parent_id, userClaims)
	if err != nil {
		return nil, 0, err
	}

	data, count, err := dal.DeviceQuery{}.GetSubList(ctx, parent_id, pageSize, page, parentDevice.TenantID)
	if err != nil {
		return nil, 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get sub device list failed:" + err.Error(),
			"id":    parent_id,
		})
	}

	return data, count, nil
}

func (*Device) GetDiagnostics(deviceID string, claims *utils.UserClaims) (*diagnostics.DiagnosticsResponse, error) {
	if _, err := ensureTelemetryDeviceReadAccess(deviceID, claims); err != nil {
		return nil, err
	}

	collector := diagnostics.GetInstance()
	data, err := collector.GetDiagnostics(deviceID)
	if err != nil {
		if err == diagnostics.ErrNotInitialized {
			return &diagnostics.DiagnosticsResponse{
				DeviceID:       deviceID,
				Stats:          nil,
				RecentFailures: []diagnostics.FailureRecord{},
			}, nil
		}
		return nil, errcode.WithData(errcode.CodeSystemError, err.Error())
	}

	return data, nil
}

// GetDeviceTemplateChartSelect returns thing-model chart options visible to the caller tenant.
func (*Device) GetDeviceTemplateChartSelect(userClaims *utils.UserClaims) (any, error) {
	tenantId, err := requireDeviceTenantClaims(userClaims, "no permission to query thing model chart selector")
	if err != nil {
		return nil, err
	}
	data, err := dal.GetDeviceTemplateChartSelect(tenantId)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}
