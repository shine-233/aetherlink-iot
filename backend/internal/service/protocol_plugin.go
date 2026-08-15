// 文件用途：维护协议插件服务入口及插件生命周期协调。
// 核心逻辑：连接设备配置、协议类型和插件通知能力，供设备变更和 broker 集成路径复用。
// 关键注意事项：协议插件是外部扩展边界，配置变更、插件不可达和设备删除顺序需谨慎处理。
// 重构建议：抽出插件注册与通知接口，补齐权限、事务后副作用、超时和兼容性测试。
package service

import (
	"encoding/json"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type ProtocolPlugin struct{}

func decodeProtocolPluginConfig(raw *string) (map[string]interface{}, error) {
	if raw == nil || !IsJSON(*raw) {
		return nil, nil
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(*raw), &config); err != nil {
		return nil, err
	}
	return config, nil
}

func ensurePluginDeviceTenantAccess(device *model.Device, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "plugin device config requires api key")
	}
	if device == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "device not found or no permission")
	}
	if claims.Authority != constant.SYS_ADMIN && device.TenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query plugin device config")
	}
	if claims.Authority == constant.TENANT_USER && !deviceOwnerMatchesClaims(device, claims) {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query plugin device config")
	}
	return nil
}

func (*ProtocolPlugin) GetDeviceConfig(req model.GetDeviceConfigReq, claims *utils.UserClaims) (interface{}, error) {
	if err := validateProtocolPluginDeviceConfigReq(req, claims); err != nil {
		return nil, err
	}

	device, err := loadProtocolPluginDevice(req)
	if err != nil {
		return nil, protocolPluginDBError(err)
	}
	if err := ensurePluginDeviceTenantAccess(device, claims); err != nil {
		return nil, err
	}

	rsp, err := buildProtocolPluginDeviceConfig(device, claims)
	if err != nil {
		return nil, err
	}

	// Do not log rsp: it contains device voucher credentials and plugin configuration.
	logrus.Info("device config prepared for protocol plugin")
	return rsp, nil
}

func validateProtocolPluginDeviceConfigReq(req model.GetDeviceConfigReq, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "plugin device config requires api key")
	}
	if req.DeviceId == "" && req.Voucher == "" && req.DeviceNumber == "" {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "device id and voucher and device_number must have one",
		})
	}
	return nil
}

func loadProtocolPluginDevice(req model.GetDeviceConfigReq) (*model.Device, error) {
	switch {
	case req.DeviceId != "":
		return dal.GetDeviceByID(req.DeviceId)
	case req.Voucher != "":
		return dal.GetDeviceByVoucher(req.Voucher)
	case req.DeviceNumber != "":
		return dal.GetDeviceByDeviceNumber(req.DeviceNumber)
	default:
		return nil, nil
	}
}

func buildProtocolPluginDeviceConfig(device *model.Device, claims *utils.UserClaims) (model.DeviceConfigForProtocolPlugin, error) {
	deviceConfig, err := loadProtocolPluginDeviceConfig(device)
	if err != nil {
		return model.DeviceConfigForProtocolPlugin{}, err
	}

	rsp, err := buildProtocolPluginBaseResponse(device, deviceConfig)
	if err != nil {
		return model.DeviceConfigForProtocolPlugin{}, err
	}

	if deviceConfig.DeviceType == "2" {
		subDevices, err := buildProtocolPluginSubDevices(device.ID, claims)
		if err != nil {
			return model.DeviceConfigForProtocolPlugin{}, err
		}
		rsp.SubDivices = subDevices
	}

	return rsp, nil
}

func loadProtocolPluginDeviceConfig(device *model.Device) (*model.DeviceConfig, error) {
	if device.DeviceConfigID == nil {
		logrus.Warn("deviceConfigID is nil")
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "device config not found",
		})
	}
	deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
	if err != nil {
		return nil, protocolPluginDBError(err)
	}
	if deviceConfig == nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "device config not found",
		})
	}
	return deviceConfig, nil
}

func buildProtocolPluginBaseResponse(device *model.Device, deviceConfig *model.DeviceConfig) (model.DeviceConfigForProtocolPlugin, error) {
	rsp := model.DeviceConfigForProtocolPlugin{
		ID:           device.ID,
		Voucher:      device.Voucher,
		DeviceNumber: device.DeviceNumber,
		DeviceType:   deviceConfig.DeviceType,
	}
	if deviceConfig.ProtocolType != nil {
		rsp.ProtocolType = *deviceConfig.ProtocolType
	}

	configTemplate, err := decodeProtocolPluginConfig(deviceConfig.ProtocolConfig)
	if err != nil {
		return rsp, protocolPluginConfigError(err)
	}
	rsp.ProtocolConfigTemplate = configTemplate

	config, err := decodeProtocolPluginConfig(device.ProtocolConfig)
	if err != nil {
		return rsp, protocolPluginConfigError(err)
	}
	rsp.Config = config

	return rsp, nil
}

func buildProtocolPluginSubDevices(parentID string, claims *utils.UserClaims) ([]model.SubDeviceConfigForProtocolPlugin, error) {
	subDeviceList, err := dal.GetSubDeviceListByParentID(parentID)
	if err != nil {
		return nil, protocolPluginDBError(err)
	}

	subDevices := make([]model.SubDeviceConfigForProtocolPlugin, 0, len(subDeviceList))
	templateCache := make(map[string]map[string]interface{})
	for _, subDevice := range subDeviceList {
		subRsp, err := buildProtocolPluginSubDevice(subDevice, claims, templateCache)
		if err != nil {
			return nil, err
		}
		subDevices = append(subDevices, subRsp)
	}
	return subDevices, nil
}

func buildProtocolPluginSubDevice(
	subDevice *model.Device,
	claims *utils.UserClaims,
	templateCache map[string]map[string]interface{},
) (model.SubDeviceConfigForProtocolPlugin, error) {
	if err := ensurePluginDeviceTenantAccess(subDevice, claims); err != nil {
		return model.SubDeviceConfigForProtocolPlugin{}, err
	}
	if subDevice.SubDeviceAddr == nil {
		logrus.Warn("subDeviceAddr is nil")
		return model.SubDeviceConfigForProtocolPlugin{}, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "subDeviceAddr not found",
		})
	}
	if subDevice.DeviceConfigID == nil {
		return model.SubDeviceConfigForProtocolPlugin{}, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "sub device config not found",
		})
	}

	subRsp := model.SubDeviceConfigForProtocolPlugin{
		DeviceID:      subDevice.ID,
		Voucher:       subDevice.Voucher,
		DeviceNumber:  subDevice.DeviceNumber,
		SubDeviceAddr: *subDevice.SubDeviceAddr,
	}

	subConfig, err := decodeProtocolPluginConfig(subDevice.ProtocolConfig)
	if err != nil {
		return subRsp, protocolPluginConfigError(err)
	}
	subRsp.Config = subConfig

	subTemplate, err := loadProtocolPluginSubDeviceTemplate(*subDevice.DeviceConfigID, templateCache)
	if err != nil {
		return subRsp, err
	}
	subRsp.ProtocolConfigTemplate = subTemplate

	return subRsp, nil
}

func loadProtocolPluginSubDeviceTemplate(
	deviceConfigID string,
	templateCache map[string]map[string]interface{},
) (map[string]interface{}, error) {
	if template, ok := templateCache[deviceConfigID]; ok {
		return template, nil
	}

	subDeviceConfig, err := dal.GetDeviceConfigByID(deviceConfigID)
	if err != nil {
		return nil, protocolPluginDBError(err)
	}
	if subDeviceConfig == nil {
		templateCache[deviceConfigID] = nil
		return nil, nil
	}
	subTemplate, err := decodeProtocolPluginConfig(subDeviceConfig.ProtocolConfig)
	if err != nil {
		return nil, protocolPluginConfigError(err)
	}
	templateCache[deviceConfigID] = subTemplate
	return subTemplate, nil
}

func protocolPluginDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func protocolPluginConfigError(err error) error {
	return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{"error": err.Error()})
}
func (*ProtocolPlugin) GetDevicesByProtocolPlugin(req model.GetDevicesByProtocolPluginReq, claims *utils.UserClaims) (interface{}, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "plugin device list requires api key")
	}
	if req.DeviceType != "1" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "protocol plugin device list only supports direct devices")
	}

	var devicesRsp model.GetDevicesByProtocolPluginRsp
	err := dal.GetDeviceListByProtocolType(req, claims.TenantID, deviceOwnerUserIDFilterForClaims(claims), &devicesRsp)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return devicesRsp, nil
}
