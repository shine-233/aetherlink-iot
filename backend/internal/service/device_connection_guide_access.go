package service

import (
	"context"
	"strings"

	model "aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
)

func guideProtocol(profile deviceConnectProfile) string {
	if strings.TrimSpace(profile.protocolType) == "" {
		return "MQTT"
	}
	return profile.protocolType
}

func buildDeviceConnectionGuideCredentialForm(d *Device, connectCtx deviceConnectContext) (any, error) {
	profile, hasForm, err := resolveDeviceConnectFormProfile(connectCtx)
	if err != nil {
		return nil, err
	}
	if !hasForm {
		return nil, nil
	}
	return d.GetVoucherTypeForm(profile.voucherType, profile.deviceType, profile.protocolType)
}

func buildDeviceConnectionGuideInfo(
	ctx context.Context,
	deviceID string,
	connectCtx deviceConnectContext,
	profile deviceConnectProfile,
	lang string,
) (any, error) {
	if profile.protocolType == "MQTT" {
		return buildMQTTDeviceConnectResponse(deviceID, connectCtx.device.DeviceNumber, profile.deviceType, lang), nil
	}
	return buildPluginDeviceConnectResponse(ctx, profile.protocolType, lang)
}

func buildDeviceConnectionGuideProfile(
	ctx context.Context,
	deviceID string,
	connectCtx deviceConnectContext,
	profile deviceConnectProfile,
) (*model.DeviceConnectionGuideProfile, error) {
	if profile.protocolType == "MQTT" {
		return buildMQTTDeviceConnectionGuideProfile(deviceID, connectCtx.device.DeviceNumber, profile), nil
	}

	pp, err := loadDeviceConnectServicePlugin(ctx, profile.protocolType)
	if err != nil {
		return nil, err
	}
	accessConfig, err := deviceConnectPluginAccessConfig(pp)
	if err != nil {
		return nil, err
	}
	host, port := splitConnectionGuideAddress(accessConfig.AccessAddress)
	return &model.DeviceConnectionGuideProfile{
		Protocol:           guideProtocol(profile),
		Endpoint:           accessConfig.AccessAddress,
		Host:               host,
		Port:               port,
		CredentialMode:     profile.voucherType,
		CredentialRequired: profile.voucherType != "",
		DeviceType:         profile.deviceType,
		DeviceNumber:       connectCtx.device.DeviceNumber,
		HTTPAddress:        accessConfig.HttpAddress,
		SubTopicPrefix:     accessConfig.SubTopicPrefix,
	}, nil
}

func buildMQTTDeviceConnectionGuideProfile(
	deviceID string,
	deviceNumber string,
	profile deviceConnectProfile,
) *model.DeviceConnectionGuideProfile {
	accessAddress := deviceConnectMQTTAccessAddress()
	host, port := splitConnectionGuideAddress(accessAddress)
	reportTopic := "devices/telemetry"
	controlTopic := "devices/telemetry/control/" + deviceNumber
	testPayload := `{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0}`
	if profile.deviceType == "2" {
		reportTopic = "gateway/telemetry"
		controlTopic = "gateway/telemetry/control/" + deviceNumber
		testPayload = `{"gateway_data":{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0},"sub_device_data":{"sub_device_address":{"temperature":26.8,"humidity":58,"rssi":-61,"online":true,"alarm_count":0}}}`
	}
	username := buildDeviceConnectMQTTUsername(deviceID)
	return &model.DeviceConnectionGuideProfile{
		Protocol:           "MQTT",
		Endpoint:           accessAddress,
		Host:               host,
		Port:               port,
		TLSEnabled:         strings.HasPrefix(accessAddress, "mqtts://") || port == "8883",
		CredentialMode:     profile.voucherType,
		CredentialRequired: true,
		DeviceType:         profile.deviceType,
		DeviceNumber:       deviceNumber,
		ClientID:           username,
		Username:           username,
		TelemetryTopic:     reportTopic,
		CommandTopic:       controlTopic,
		TestPayload:        testPayload,
		SamplePayload:      testPayload,
	}
}

func splitConnectionGuideAddress(address string) (string, string) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return "", ""
	}
	withoutScheme := strings.TrimPrefix(strings.TrimPrefix(trimmed, "mqtt://"), "mqtts://")
	withoutScheme = strings.TrimPrefix(strings.TrimPrefix(withoutScheme, "http://"), "https://")
	if strings.HasPrefix(withoutScheme, ":") {
		return "", strings.TrimPrefix(withoutScheme, ":")
	}
	parts := strings.SplitN(withoutScheme, ":", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func buildConnectionGuideTLSHint() model.DeviceConnectionGuideTLSHint {
	broker := strings.TrimSpace(viper.GetString("mqtts.broker"))
	return model.DeviceConnectionGuideTLSHint{
		Enabled:            broker != "",
		Broker:             broker,
		CertificateHint:    "Use the platform CA/client certificate settings when MQTT over TLS is enabled.",
		AdvertisedToDevice: "unknown",
	}
}
