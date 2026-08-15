package service

import (
	"context"
	"strings"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// BatchUpdateDeviceConfig binds a batch of devices to the same device config after access checks.
func (*DeviceConfig) BatchUpdateDeviceConfig(req *model.BatchUpdateDeviceConfigReq, claims *utils.UserClaims) error {
	deviceConfig, err := ensureDeviceConfigWriteAccess(req.DeviceConfigID, claims)
	if err != nil {
		return err
	}

	deviceIDs := normalizeBatchDeviceConfigIDs(req.DeviceIds)
	if err := validateBatchDeviceConfigDevices(deviceIDs, deviceConfig, claims); err != nil {
		return err
	}

	if err := dal.UpdateDeviceDeviceConfigIDs(deviceIDs, &req.DeviceConfigID); err != nil {
		return wrapDeviceConfigDBError(err)
	}
	_ = initialize.DelDeviceCaches(deviceIDs)

	return nil
}

func normalizeBatchDeviceConfigIDs(deviceIDs []string) []string {
	normalizedIDs := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			continue
		}
		if _, ok := seen[deviceID]; ok {
			continue
		}
		seen[deviceID] = struct{}{}
		normalizedIDs = append(normalizedIDs, deviceID)
	}
	return normalizedIDs
}

func validateBatchDeviceConfigDevices(deviceIDs []string, deviceConfig *model.DeviceConfig, claims *utils.UserClaims) error {
	devicesByID, err := dal.GetDevicesByIDs(deviceIDs)
	if err != nil {
		return wrapDeviceConfigDBError(err)
	}

	for _, deviceID := range deviceIDs {
		deviceInfo := devicesByID[deviceID]
		if !hasTelemetryTenantAccess(deviceInfo, claims, false) {
			return errcode.NewWithMessage(errcode.CodeNoPermission, telemetryWritePermissionMessage)
		}
		if deviceInfo.TenantID != deviceConfig.TenantID {
			return errcode.NewWithMessage(errcode.CodeNoPermission, "device and device config tenant mismatch")
		}
		if deviceInfo.DeviceConfigID != nil && !common.CheckEmpty(*deviceInfo.DeviceConfigID) {
			deviceName := ""
			if deviceInfo.Name != nil {
				deviceName = *deviceInfo.Name
			}
			return errcode.WithVars(200071, map[string]interface{}{
				"device_name": deviceName,
			})
		}
	}
	return nil
}

// GetDeviceConfigConnect returns the connect-option metadata for the target device config.
func (*DeviceConfig) GetDeviceConfigConnect(ctx context.Context, deviceID string, lang string, claims *utils.UserClaims) (res *model.DeviceConfigConnectRes, err error) {
	_ = ctx

	deviceInfo, err := ensureTelemetryDeviceWriteAccess(deviceID, claims)
	if err != nil {
		return nil, err
	}
	if deviceInfo.DeviceConfigID == nil || common.CheckEmpty(*deviceInfo.DeviceConfigID) {
		return nil, nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*deviceInfo.DeviceConfigID)
	if err != nil {
		return nil, wrapDeviceConfigDBError(err)
	}
	if deviceConfig == nil || deviceConfig.ProtocolType == nil {
		return nil, nil
	}

	if *deviceConfig.ProtocolType != "MQTT" {
		return nil, nil
	}

	basicLabel, tokenLabel := deviceConfigCredentialLabels(lang)
	return &model.DeviceConfigConnectRes{
		Basic:       basicLabel,
		AccessToken: tokenLabel,
	}, nil
}

// GetVoucherTypeForm returns the voucher selector schema for a device config protocol.
func (*DeviceConfig) GetVoucherTypeForm(deviceType string, protocolType string, lang string) (data interface{}, err error) {
	if protocolType != "MQTT" {
		var pd ServicePlugin
		return pd.GetPluginForm(protocolType, deviceType, string(constant.VOUCHER_FORM))
	}

	basicLabel, tokenLabel := deviceConfigCredentialLabels(lang)
	return map[string]interface{}{
		basicLabel: "BASIC",
		tokenLabel: "ACCESSTOKEN",
	}, nil
}

// deviceConfigCredentialLabels returns localized credential labels shared by connect and form endpoints.
func deviceConfigCredentialLabels(lang string) (basicLabel, tokenLabel string) {
	basicLabel = "账号密码认证"
	tokenLabel = "账号认证（无密码）"
	normalizedLang := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(normalizedLang, "en") {
		basicLabel = "Username & Password"
		tokenLabel = "Username (No Password)"
	} else if strings.HasPrefix(normalizedLang, "fr") {
		basicLabel = "Nom d'utilisateur et mot de passe"
		tokenLabel = "Nom d'utilisateur (sans mot de passe)"
	} else if strings.HasPrefix(normalizedLang, "es") {
		basicLabel = "Usuario y contrasena"
		tokenLabel = "Usuario (sin contrasena)"
	}
	return basicLabel, tokenLabel
}
