package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type deviceConnectContext struct {
	device       *model.Device
	deviceConfig *model.DeviceConfig
}

type deviceConnectProfile struct {
	voucherType  string
	deviceType   string
	protocolType string
}

// DeviceConnectForm returns the credential form a device needs for manual connect guidance.
func (d *Device) DeviceConnectForm(ctx context.Context, param *model.DeviceConnectFormReq, claims *utils.UserClaims) (any, error) {
	connectCtx, err := loadDeviceConnectContext(ctx, param.DeviceID, claims)
	if err != nil {
		return nil, err
	}

	profile, hasForm, err := resolveDeviceConnectFormProfile(connectCtx)
	if err != nil {
		return nil, err
	}
	if !hasForm {
		return nil, nil
	}

	data, err := d.GetVoucherTypeForm(profile.voucherType, profile.deviceType, profile.protocolType)
	if err != nil {
		logrus.Error(ctx, "get voucher type form failed:", err)
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"msg": "get voucher type form failed",
		})
	}

	return data, nil
}

// GetVoucherTypeForm returns the connect form schema for a voucher / protocol combination.
func (*Device) GetVoucherTypeForm(voucherType string, deviceType string, protocolType string) (interface{}, error) {
	if protocolType != "MQTT" {
		logrus.Debug("load voucher form from protocol plugin")
		var pp ServicePlugin
		return pp.GetPluginForm(protocolType, deviceType, string(constant.VOUCHER_FORM))
	}

	p1 := &model.DeviceConnectFormRes{
		DataKey:     "username",
		Label:       "MQTT 用户名",
		Placeholder: "MQTT 用户名",
		Type:        "input",
		Validate: model.DeviceConnectFormValidateRes{
			Message:  "用户名不能为空",
			Required: true,
			Type:     "string",
		},
	}
	p2 := &model.DeviceConnectFormRes{
		DataKey:     "password",
		Label:       "MQTT 密码",
		Placeholder: "MQTT 密码",
		Type:        "input",
		Validate: model.DeviceConnectFormValidateRes{
			Required: true,
			Type:     "string",
		},
	}

	switch voucherType {
	case "BASIC":
		return []*model.DeviceConnectFormRes{p1, p2}, nil
	case "ACCESSTOKEN":
		p1.Label = "MQTT 用户名（密码留空）"
		return []*model.DeviceConnectFormRes{p1}, nil
	default:
		return nil, fmt.Errorf("voucher type is error: %s", voucherType)
	}
}

// DeviceConnect returns the runtime connection payload for MQTT or protocol-plugin devices.
func (*Device) DeviceConnect(ctx context.Context, param *model.DeviceConnectFormReq, lang string, claims *utils.UserClaims) (any, error) {
	connectCtx, err := loadDeviceConnectContext(ctx, param.DeviceID, claims)
	if err != nil {
		return nil, err
	}

	profile, err := resolveDeviceConnectRuntimeProfile(connectCtx)
	if err != nil {
		return nil, err
	}
	if profile.protocolType == "MQTT" {
		return buildMQTTDeviceConnectResponse(param.DeviceID, connectCtx.device.DeviceNumber, profile.deviceType, lang), nil
	}
	return buildPluginDeviceConnectResponse(ctx, profile.protocolType, lang)
}

func loadDeviceConnectContext(ctx context.Context, deviceID string, claims *utils.UserClaims) (deviceConnectContext, error) {
	device, err := ensureTelemetryDeviceWriteAccess(deviceID, claims)
	if err != nil {
		return deviceConnectContext{}, err
	}
	if device.DeviceConfigID == nil {
		return deviceConnectContext{device: device}, nil
	}

	deviceConfig, err := loadDeviceConnectConfig(ctx, deviceID, *device.DeviceConfigID)
	if err != nil {
		return deviceConnectContext{}, err
	}
	return deviceConnectContext{
		device:       device,
		deviceConfig: deviceConfig,
	}, nil
}

func loadDeviceConnectConfig(ctx context.Context, deviceID string, deviceConfigID string) (*model.DeviceConfig, error) {
	deviceConfig, err := dal.GetDeviceConfigByID(deviceConfigID)
	if err != nil {
		logrus.Error(ctx, "[Device][DeviceConnect]GetDeviceConfigByID failed:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "get device config info failed:" + err.Error(),
			"id":    deviceID,
		})
	}
	return deviceConfig, nil
}

func defaultDeviceConnectProfile() deviceConnectProfile {
	return deviceConnectProfile{
		voucherType:  "BASIC",
		deviceType:   "1",
		protocolType: "MQTT",
	}
}

func resolveDeviceConnectFormProfile(connectCtx deviceConnectContext) (deviceConnectProfile, bool, error) {
	if connectCtx.deviceConfig == nil {
		return defaultDeviceConnectProfile(), true, nil
	}
	if connectCtx.deviceConfig.DeviceType == strconv.Itoa(constant.GATEWAY_SON_DEVICE) {
		return deviceConnectProfile{}, false, nil
	}
	profile, err := resolveConfiguredDeviceConnectProfile(connectCtx.deviceConfig)
	if err != nil {
		return deviceConnectProfile{}, false, err
	}
	return profile, true, nil
}

func resolveDeviceConnectRuntimeProfile(connectCtx deviceConnectContext) (deviceConnectProfile, error) {
	if connectCtx.deviceConfig == nil {
		return defaultDeviceConnectProfile(), nil
	}

	return resolveConfiguredDeviceConnectProfile(connectCtx.deviceConfig)
}

func resolveConfiguredDeviceConnectProfile(deviceConfig *model.DeviceConfig) (deviceConnectProfile, error) {
	profile := deviceConnectProfile{}
	if deviceConfig == nil {
		return profile, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"msg": "device_config is null",
		})
	}
	profile.deviceType = deviceConfig.DeviceType

	protocolType, err := requireDeviceConnectProtocolType(deviceConfig)
	if err != nil {
		return deviceConnectProfile{}, err
	}
	profile.protocolType = protocolType

	if deviceConfig.VoucherType != nil {
		profile.voucherType = *deviceConfig.VoucherType
	}

	return profile, nil
}

func requireDeviceConnectProtocolType(deviceConfig *model.DeviceConfig) (string, error) {
	if deviceConfig == nil || deviceConfig.ProtocolType == nil {
		return "", errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"msg": "device_config protocol_type is null",
		})
	}
	return *deviceConfig.ProtocolType, nil
}

func buildMQTTDeviceConnectResponse(deviceID, deviceNumber, deviceType, lang string) any {
	accessAddress := deviceConnectMQTTAccessAddress()
	switch deviceType {
	case "1":
		return buildDirectMQTTDeviceConnectResponse(lang, accessAddress, deviceID, deviceNumber)
	case "2":
		return buildGatewayMQTTDeviceConnectResponse(lang, accessAddress, deviceID, deviceNumber)
	default:
		return nil
	}
}

func buildDirectMQTTDeviceConnectResponse(lang, accessAddress, deviceID, deviceNumber string) map[string]string {
	return deviceConnectMQTTResponse(
		lang,
		accessAddress,
		deviceID,
		"devices/telemetry",
		fmt.Sprintf("devices/telemetry/control/%s", deviceNumber),
		"{\"temperature\":25.5,\"humidity\":60,\"rssi\":-52,\"online\":true,\"alarm_count\":0}",
	)
}

func buildGatewayMQTTDeviceConnectResponse(lang, accessAddress, deviceID, deviceNumber string) map[string]string {
	remark := `{"gateway_data":{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0},"sub_device_data":{"sub_device_address":{"temperature":26.8,"humidity":58,"rssi":-61,"online":true,"alarm_count":0}}}`
	return deviceConnectMQTTResponse(
		lang,
		accessAddress,
		deviceID,
		"gateway/telemetry",
		fmt.Sprintf("gateway/telemetry/control/%s", deviceNumber),
		remark,
	)
}

func deviceConnectMQTTAccessAddress() string {
	accessAddress := viper.GetString("mqtt.access_address")
	if accessAddress == "" {
		return ":1883"
	}
	return accessAddress
}

func deviceConnectMQTTResponse(lang, accessAddress, deviceID, reportTopic, controlTopic, remark string) map[string]string {
	return map[string]string{
		global.ResponseHandler.ErrManager.GetMessage(500001, lang): accessAddress,
		global.ResponseHandler.ErrManager.GetMessage(500002, lang): buildDeviceConnectMQTTUsername(deviceID),
		global.ResponseHandler.ErrManager.GetMessage(500003, lang): reportTopic,
		global.ResponseHandler.ErrManager.GetMessage(500004, lang): controlTopic,
		global.ResponseHandler.ErrManager.GetMessage(500005, lang): remark,
	}
}

func buildDeviceConnectMQTTUsername(deviceID string) string {
	if len(deviceID) <= 12 {
		return "mqtt_" + deviceID
	}
	return "mqtt_" + deviceID[:12]
}

// buildPluginDeviceConnectResponse loads non-MQTT connection details from the protocol plugin config.
func buildPluginDeviceConnectResponse(ctx context.Context, protocolType string, lang string) (any, error) {
	pp, err := loadDeviceConnectServicePlugin(ctx, protocolType)
	if err != nil {
		return nil, err
	}

	return buildDeviceConnectPluginInfo(pp, lang)
}

func loadDeviceConnectServicePlugin(ctx context.Context, protocolType string) (*model.ServicePlugin, error) {
	pp, err := dal.GetServicePluginByServiceIdentifier(protocolType)
	if err != nil {
		logrus.Error(ctx, "get protocol plugin failed:", err)
		return nil, err
	}
	return pp, nil
}

func buildDeviceConnectPluginInfo(pp *model.ServicePlugin, lang string) (map[string]interface{}, error) {
	info := make(map[string]interface{})
	if pp == nil || pp.ServiceType != int32(1) {
		return info, nil
	}

	accessAddress, err := deviceConnectPluginAccessAddress(pp)
	if err != nil {
		return nil, err
	}

	info[global.ResponseHandler.ErrManager.GetMessage(500006, lang)] = accessAddress
	return info, nil
}

func deviceConnectPluginAccessAddress(pp *model.ServicePlugin) (string, error) {
	protocolAccessConfig, err := deviceConnectPluginAccessConfig(pp)
	if err != nil {
		return "", err
	}

	return protocolAccessConfig.AccessAddress, nil
}

func deviceConnectPluginAccessConfig(pp *model.ServicePlugin) (model.ProtocolAccessConfig, error) {
	if pp == nil || pp.ServiceConfig == nil {
		return model.ProtocolAccessConfig{}, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"msg": "service plugin config is null",
		})
	}

	var protocolAccessConfig model.ProtocolAccessConfig
	if err := json.Unmarshal([]byte(*pp.ServiceConfig), &protocolAccessConfig); err != nil {
		return model.ProtocolAccessConfig{}, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"msg": "service plugin config parse failed",
		})
	}

	return protocolAccessConfig, nil
}
