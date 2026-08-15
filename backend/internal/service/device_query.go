package service

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// 返回租户可见的设备选择项。
func (*Device) GetDeviceSelector(
	req model.DeviceSelectorReq,
	userClaims *utils.UserClaims,
) (*model.DeviceSelectorRes, error) {
	tenantID, err := requireDeviceTenantClaims(userClaims, "no permission to query device selector")
	if err != nil {
		return nil, err
	}
	applyDeviceSelectorOwnerFilterForClaims(&req, userClaims)
	list, err := dal.GetDeviceSelector(req, tenantID)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func requireDeviceTenantClaims(userClaims *utils.UserClaims, message string) (string, error) {
	if err := requireSupportedScopeAuthority(userClaims, message); err != nil {
		return "", err
	}
	if userClaims.TenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, message)
	}
	return userClaims.TenantID, nil
}

// 汇总租户最近上报的设备遥测数据。
func (d *Device) GetTenantTelemetryData(userClaims *utils.UserClaims) ([]map[string]interface{}, error) {
	tenantID, err := requireDeviceTenantClaims(userClaims, "no permission to query tenant telemetry")
	if err != nil {
		return nil, err
	}
	devices, err := dal.GetTenantTelemetryData(tenantID, deviceOwnerUserIDFilterForClaims(userClaims))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get tenant telemetry data failed:" + err.Error(),
			"id":    tenantID,
		})
	}

	deviceIDs := tenantTelemetryDeviceIDs(devices)
	deviceMap, err := dal.GetDevicesByIDsForTenant(deviceIDs, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get tenant telemetry devices failed:" + err.Error(),
			"id":    tenantID,
		})
	}

	telemetryByDeviceID, err := dal.GetCurrentTelemetryDataEvolutionByDeviceIDs(deviceIDs)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get tenant telemetry current data failed:" + err.Error(),
			"id":    tenantID,
		})
	}

	telemetryDataList := make([]map[string]interface{}, 0, len(devices))
	for _, device := range devices {
		deviceInfo := deviceMap[device.DeviceID]
		if deviceInfo == nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error": "get device failed: device is nil for id: " + device.DeviceID,
				"id":    device.DeviceID,
			})
		}

		telemetry := telemetryByDeviceID[device.DeviceID]
		labelMap, getErr := loadMapTelemetryLabels(deviceInfo, device.DeviceID, telemetry)
		if getErr != nil {
			return nil, getErr
		}
		telemetryDataList = append(telemetryDataList, buildMapTelemetryResponse(deviceInfo, telemetry, labelMap))
	}

	return telemetryDataList, nil
}

func tenantTelemetryDeviceIDs(devices []dal.NewDeviceData) []string {
	deviceIDs := make([]string, 0, len(devices))
	for _, device := range devices {
		deviceIDs = append(deviceIDs, device.DeviceID)
	}
	return deviceIDs
}

// 获取设备状态历史记录。
func (*Device) GetDeviceStatusHistory(
	req *model.GetDeviceStatusHistoryReq,
	userClaims *utils.UserClaims,
) (map[string]interface{}, error) {
	deviceInfo, err := ensureTelemetryDeviceReadAccess(req.DeviceID, userClaims)
	if err != nil {
		return nil, err
	}

	total, list, err := dal.GetDeviceStatusHistoryByPage(req, deviceInfo.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	result := make(map[string]interface{})
	result["total"] = total
	result["list"] = list
	return result, nil
}
