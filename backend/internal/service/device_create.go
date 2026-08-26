package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

type createDeviceContext struct {
	createdAt    time.Time
	deviceConfig *model.DeviceConfig
	voucher      string
}

func (*Device) CreateDevice(req model.CreateDeviceReq, claims *utils.UserClaims) (device model.Device, err error) {
	if err := ensureTenantScopedWriteClaims(claims, "create device"); err != nil {
		return device, err
	}

	req, err = normalizeCreateDevicePIDNumber(req)
	if err != nil {
		return device, err
	}

	device.ID, err = resolveCreateDeviceID(req.ID)
	if err != nil {
		return device, err
	}
	device.Name = req.Name

	createContext, err := buildCreateDeviceContext(req, claims)
	if err != nil {
		return device, err
	}

	device.Voucher = createContext.voucher
	device.TenantID = claims.TenantID
	device.OwnerUserID = createdDeviceOwnerUserID(claims)
	device.CreatedAt = &createContext.createdAt
	device.UpdateAt = &createContext.createdAt

	if err := applyCreateDeviceNumber(&device, req.DeviceNumber); err != nil {
		return device, err
	}
	if err := ensureCreateDeviceNumberAvailable(device.DeviceNumber); err != nil {
		return device, err
	}

	applyCreateDeviceRequestFields(&device, req)
	if err := persistCreateDevice(&device); err != nil {
		return device, err
	}
	return device, nil
}

func normalizeCreateDevicePIDNumber(req model.CreateDeviceReq) (model.CreateDeviceReq, error) {
	if err := normalizeOptionalRDIDeviceNumber(&req.DeviceNumber, req.PIDNumber); err != nil {
		return req, err
	}
	return req, nil
}

func buildCreateDeviceContext(req model.CreateDeviceReq, claims *utils.UserClaims) (createDeviceContext, error) {
	deviceConfig, err := loadCreateDeviceConfig(normalizeCreateDeviceConfigID(req.DeviceConfigId), claims)
	if err != nil {
		return createDeviceContext{}, err
	}

	createdAt := time.Now().UTC()
	return createDeviceContext{
		createdAt:    createdAt,
		deviceConfig: deviceConfig,
		voucher:      buildCreateDeviceVoucher(req.Voucher, deviceConfig),
	}, nil
}

func resolveCreateDeviceID(requestedID *string) (string, error) {
	if requestedID == nil || *requestedID == "" {
		return uuid.New(), nil
	}

	deviceID := *requestedID
	if len(deviceID) < 8 {
		return "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "设备 ID 至少需要 8 个字符",
		})
	}
	if !isValidDeviceID(deviceID) {
		return "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "设备 ID 格式无效，必须为 8-36 个字符，且只能包含字母、数字、连字符和下划线",
		})
	}

	existingDevice, err := dal.GetDeviceByIDUnscoped(deviceID)
	if err == nil && existingDevice != nil {
		return "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "设备 ID 已存在",
		})
	}
	return deviceID, nil
}

func normalizeCreateDeviceConfigID(configID *string) *string {
	if configID != nil && strings.TrimSpace(*configID) == "" {
		return nil
	}
	return configID
}

func loadCreateDeviceConfig(configID *string, claims *utils.UserClaims) (*model.DeviceConfig, error) {
	if configID == nil {
		return nil, nil
	}
	return ensureDeviceConfigWriteAccess(*configID, claims)
}

func buildCreateDeviceVoucher(requestedVoucher *string, deviceConfig *model.DeviceConfig) string {
	if requestedVoucher != nil {
		return *requestedVoucher
	}
	if deviceConfig == nil {
		return buildCreateDeviceBasicVoucher()
	}
	if deviceConfig.ProtocolType != nil && *deviceConfig.ProtocolType == "MQTT" {
		if deviceConfig.VoucherType != nil && *deviceConfig.VoucherType == "BASIC" {
			return buildCreateDeviceBasicVoucher()
		}
		return `{"username":"` + uuid.New()[0:22] + `"}`
	}
	return `{"default":"` + uuid.New() + `"}`
}

func buildCreateDeviceBasicVoucher() string {
	return `{"username":"` + uuid.New()[0:22] + `","password":"` + uuid.New()[0:7] + `"}`
}

func applyCreateDeviceNumber(device *model.Device, requestedNumber *string) error {
	if requestedNumber == nil {
		device.DeviceNumber = device.ID
	} else {
		device.DeviceNumber = *requestedNumber
	}

	if strings.TrimSpace(device.DeviceNumber) == "" {
		return errcode.WithVars(100005, map[string]interface{}{
			"field": "device_number",
		})
	}
	return nil
}

func ensureCreateDeviceNumberAvailable(deviceNumber string) error {
	exists, err := dal.CheckDeviceNumberExists(deviceNumber)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"message":   "检查设备编号是否存在失败",
		})
	}
	if exists {
		return errcode.New(204004)
	}
	return nil
}

func applyCreateDeviceRequestFields(device *model.Device, req model.CreateDeviceReq) {
	device.ProductID = req.ProductID
	device.ParentID = req.ParentID
	device.Protocol = req.Protocol
	device.Label = req.Label
	device.Location = req.Location
	device.SubDeviceAddr = req.SubDeviceAddr
	device.CurrentVersion = req.CurrentVersion
	device.AdditionalInfo = req.AdditionalInfo
	device.ProtocolConfig = req.ProtocolConfig
	device.Remark1 = req.Remark1
	device.Remark2 = req.Remark2
	device.Remark3 = req.Remark3
	device.AccessWay = req.AccessWay
	device.Description = req.Description
	// A blank device_config_id must persist as NULL: devices.device_config_id has
	// fk_device_config_id REFERENCES device_configs(id), so writing "" raises a
	// 23503 foreign-key error. Normalizing here (the single place that writes the
	// column) keeps callers that pass `"device_config_id": ""` working.
	device.DeviceConfigID = normalizeCreateDeviceConfigID(req.DeviceConfigId)
	device.IsOnline = 0
	device.ActivateFlag = "active"
	// Newly created devices are immediately visible/usable in this API. Keep the
	// enablement flag aligned with the active lifecycle state so MQTT auth does
	// not reject a device created for simulation or onboarding.
	device.IsEnabled = "enabled"
}

func persistCreateDevice(device *model.Device) error {
	if err := dal.CreateDevice(device); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}
