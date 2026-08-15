package service

// 文件用途：
// - 承接设备实例与设备配置之间的绑定/解绑切换边界。
// 核心逻辑：
// - 统一处理配置切换前的设备写权限、当前配置可否切换、目标配置租户归属和兼容输入归一化。
// 使用注意：
// - `DeviceConfigID == ""` 在这里仍表示兼容旧输入的“解绑配置”语义，不应在新逻辑中继续扩散。
// 重构建议：
// - 后续如继续细化，可把“当前配置可否切换”和“目标配置授权”拆成更小 helper，但不要改变解绑、跨租户和网关/子设备限制的既有语义。

import (
	initialize "aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
	"strconv"
)

// ensureWritableDeviceConfigForTenant 读取目标配置，并校验调用者是否有权在对应租户下写入设备。
func ensureWritableDeviceConfigForTenant(configID string, claims *utils.UserClaims, tenantID string, mismatchMessage string) (*model.DeviceConfig, error) {
	deviceConfig, err := ensureDeviceConfigWriteAccess(configID, claims)
	if err != nil {
		return nil, err
	}
	if err := ensureDeviceConfigTenantMatch(deviceConfig, tenantID, mismatchMessage); err != nil {
		return nil, err
	}
	return deviceConfig, nil
}

func ensureDeviceConfigTenantMatch(deviceConfig *model.DeviceConfig, tenantID string, mismatchMessage string) error {
	if deviceConfig.TenantID != tenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, mismatchMessage)
	}
	return nil
}

func ensureDeviceConfigChangeAllowed(device *model.Device, deviceID string) error {
	if device.DeviceConfigID == nil {
		return nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device config info failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	return ensureCurrentConfigCanBeChanged(device, deviceConfig, deviceID)
}

func ensureCurrentConfigCanBeChanged(device *model.Device, deviceConfig *model.DeviceConfig, deviceID string) error {
	switch deviceConfig.DeviceType {
	case strconv.Itoa(constant.GATEWAY_DEVICE):
		data, err := dal.GetSubDeviceListByParentID(deviceID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error": "get sub device list failed:" + err.Error(),
				"id":    deviceID,
			})
		}
		if len(data) > 0 {
			return errcode.New(200061)
		}
	case strconv.Itoa(constant.GATEWAY_SON_DEVICE):
		if device.ParentID != nil {
			return errcode.New(200063)
		}
	}
	return nil
}

func normalizeAndAuthorizeNextDeviceConfig(param *model.ChangeDeviceConfigReq, device *model.Device, claims *utils.UserClaims) error {
	if param.DeviceConfigID == nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "device config id is null",
		})
	}
	if *param.DeviceConfigID == "" {
		param.DeviceConfigID = nil
		return nil
	}

	_, err := ensureWritableDeviceConfigForTenant(
		*param.DeviceConfigID,
		claims,
		device.TenantID,
		"device config tenant mismatch",
	)
	return err
}

// UpdateDeviceConfig 在完成设备写权限与配置切换策略校验后，执行设备配置绑定/解绑切换。
func (*Device) UpdateDeviceConfig(param *model.ChangeDeviceConfigReq, claims *utils.UserClaims) error {
	// Config changes share the telemetry write-access gate for the target device.
	device, err := ensureTelemetryDeviceWriteAccess(param.DeviceID, claims)
	if err != nil {
		return err
	}
	if err := ensureDeviceConfigChangeAllowed(device, param.DeviceID); err != nil {
		return err
	}

	if err := normalizeAndAuthorizeNextDeviceConfig(param, device, claims); err != nil {
		return err
	}

	// Persist the config change before disconnecting affected runtime sessions.
	err = dal.DeviceQuery{}.ChangeDeviceConfig(param.DeviceID, param.DeviceConfigID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "update device config failed:" + err.Error(),
			"id":    param.DeviceID,
		})
	}
	// The command/attribute services read the device through Redis first. A
	// binding change must invalidate that snapshot before the next preview or
	// downlink; otherwise an unbind/rebind sequence can dereference a deleted
	// device_config row from the stale device cache.
	_ = initialize.DelDeviceCache(param.DeviceID)
	// Surface the config-change persistence error directly.
	return err
}
