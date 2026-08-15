// 文件用途：维护设备认证凭据和接入 token 生成服务。
// 核心逻辑：生成、刷新和校验设备接入凭据，并绑定设备配置或设备实例的认证信息。
// 关键注意事项：凭据泄露会影响设备接入安全，日志、错误返回和默认值不得暴露密钥。
// 重构建议：抽出凭据存储与生成接口，补齐过期、轮换、跨租户和并发刷新测试。
package service

import (
	"errors"
	"time"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

type DeviceAuth struct{}

func (*DeviceAuth) Auth(req *model.DeviceAuthReq) (*model.DeviceAuthRes, error) {
	deviceConfig, err := lookupAuthDeviceConfig(req.TemplateSecret)
	if err != nil {
		return nil, err
	}

	if _, err = ensureDeviceNumberAvailable(req.DeviceNumber); err != nil {
		return nil, err
	}

	productID, err := lookupAuthProductID(req.ProductKey)
	if err != nil {
		return nil, err
	}

	device, err := buildAuthDevice(req, deviceConfig, productID)
	if err != nil {
		return nil, err
	}

	if err = dal.CreateDevice(device); err != nil {
		logrus.Error("[DeviceAuth][Auth] CreateDevice failed:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return buildDeviceAuthRes(device), nil
}

func lookupAuthDeviceConfig(templateSecret string) (*model.DeviceConfig, error) {
	deviceConfig, err := dal.GetDeviceConfigByTemplateSecret(templateSecret)
	if err != nil {
		logrus.Error("[DeviceAuth][Auth] GetDeviceConfigByTemplateSecret failed:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	if deviceConfig == nil {
		return nil, errcode.New(200080)
	}

	if deviceConfig.AutoRegister != 1 {
		return nil, errcode.New(200081)
	}

	return deviceConfig, nil
}

func ensureDeviceNumberAvailable(deviceNumber string) (*model.Device, error) {
	device, err := dal.GetDeviceByDeviceNumber(deviceNumber)
	if err != nil {
		if !errors.Is(err, dal.ErrRecordNotFound) {
			logrus.Error("[DeviceAuth][Auth] GetDeviceByDeviceNumber failed:", err)
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		return nil, nil
	}

	return device, errcode.New(200082)
}

func lookupAuthProductID(productKey *string) (string, error) {
	if productKey == nil || *productKey == "" {
		return "", nil
	}

	product, err := dal.GetProductByProductKey(*productKey)
	if err != nil {
		logrus.Error("[DeviceAuth][Auth] GetProductByProductKey failed:", err)
		return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if product == nil {
		return "", errcode.New(200083)
	}

	return product.ID, nil
}

func buildAuthDevice(req *model.DeviceAuthReq, deviceConfig *model.DeviceConfig, productID string) (*model.Device, error) {
	now := time.Now().UTC()
	device := &model.Device{
		ID:             uuid.New(),
		DeviceNumber:   req.DeviceNumber,
		CreatedAt:      &now,
		UpdateAt:       &now,
		DeviceConfigID: &deviceConfig.ID,
		ActivateFlag:   "active",
		IsOnline:       0,
		TenantID:       deviceConfig.TenantID,
		IsEnabled:      "enable",
		Name:           resolveAuthDeviceName(req),
		Voucher:        buildAuthVoucher(deviceConfig),
	}

	if productID != "" {
		device.ProductID = &productID
	}

	if err := applyAuthSubDeviceLink(device, req, deviceConfig); err != nil {
		return nil, err
	}

	return device, nil
}

func resolveAuthDeviceName(req *model.DeviceAuthReq) *string {
	if req.DeviceName != nil && *req.DeviceName != "" {
		return req.DeviceName
	}

	defaultName := "Device_" + req.DeviceNumber
	return &defaultName
}

func applyAuthSubDeviceLink(device *model.Device, req *model.DeviceAuthReq, deviceConfig *model.DeviceConfig) error {
	if deviceConfig.DeviceType != "3" {
		return nil
	}

	if req.SubDeviceAddr == nil || *req.SubDeviceAddr == "" || req.ParentDeviceNumber == nil || *req.ParentDeviceNumber == "" {
		return errcode.New(200084)
	}

	parentDevice, err := dal.GetDeviceByDeviceNumber(*req.ParentDeviceNumber)
	if err != nil {
		return errcode.WithData(200085, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if parentDevice == nil {
		return errcode.New(200085)
	}

	if dal.GetSubDeviceExists(parentDevice.ID, *req.SubDeviceAddr) {
		return errcode.New(200086)
	}

	device.SubDeviceAddr = req.SubDeviceAddr
	device.ParentID = &parentDevice.ID
	return nil
}

func buildAuthVoucher(deviceConfig *model.DeviceConfig) string {
	if deviceConfig.ProtocolType != nil && *deviceConfig.ProtocolType == "MQTT" {
		if deviceConfig.VoucherType != nil && *deviceConfig.VoucherType == "ACCESSTOKEN" {
			return `{"username":"` + uuid.New() + `"}`
		}
		if deviceConfig.VoucherType != nil && *deviceConfig.VoucherType == "BASIC" {
			return `{"username":"` + uuid.New() + `","password":"` + uuid.New()[0:7] + `"}`
		}
		return `{"username":"` + uuid.New() + `"}`
	}

	return `{"voucher":"` + uuid.New() + `"}`
}

func buildDeviceAuthRes(device *model.Device) *model.DeviceAuthRes {
	return &model.DeviceAuthRes{
		DeviceID: device.ID,
		Voucher:  device.Voucher,
	}
}
