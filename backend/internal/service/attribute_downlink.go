package service

import (
	"encoding/json"
	"fmt"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type attributeDeviceProfile struct {
	device       *model.Device
	deviceType   string
	protocolType string
}

func ensureAttributeWriteAccess(deviceID string, claimsOpt ...*utils.UserClaims) error {
	if len(claimsOpt) == 0 {
		return nil
	}
	_, err := ensureTelemetryDeviceWriteAccess(deviceID, claimsOpt[0])
	return err
}

func loadAttributeSetDeviceProfile(deviceID string) (*attributeDeviceProfile, error) {
	device, err := initialize.GetDeviceCacheById(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	profile := &attributeDeviceProfile{
		device:       device,
		deviceType:   "1",
		protocolType: "MQTT",
	}
	if device.DeviceConfigID == nil {
		return profile, nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}
	profile.deviceType = deviceConfig.DeviceType
	if deviceConfig.ProtocolType != nil {
		profile.protocolType = *deviceConfig.ProtocolType
	}
	return profile, nil
}

func buildAttributeSetPayload(req *model.AttributePutMessage, device *model.Device, deviceType string) ([]byte, error) {
	if err := transformAttributeDataForMultiLevelGateway(req, device, deviceType); err != nil {
		return nil, fmt.Errorf("failed to transform attribute data: %w", err)
	}
	return []byte(req.Value), nil
}

func buildAttributeGetPayload(keys []string) ([]byte, error) {
	data := map[string]interface{}{
		"keys": keys,
	}
	if len(keys) == 0 {
		data["keys"] = []string{}
	}
	return json.Marshal(data)
}

func (a *AttributeData) publishAttributeSet(device, targetDevice *model.Device, targetDeviceNumber, deviceType, topicPrefix, messageID string, jsonData []byte) error {
	if a.downlinkBus == nil {
		return fmt.Errorf("downlink service not available")
	}
	msg := &downlink.Message{
		DeviceID:       device.ID,
		DeviceNumber:   targetDeviceNumber,
		DeviceType:     deviceType,
		DeviceConfigID: a.getDeviceConfigID(targetDevice),
		Type:           downlink.MessageTypeAttributeSet,
		Data:           jsonData,
		TopicPrefix:    topicPrefix,
		MessageID:      messageID,
	}
	a.downlinkBus.PublishAttributeSet(msg)

	logrus.WithFields(logrus.Fields{
		"device_id":            device.ID,
		"target_device_id":     targetDevice.ID,
		"target_device_number": targetDeviceNumber,
		"device_type":          deviceType,
		"message_id":           messageID,
	}).Info("Attribute set sent via downlink")
	return nil
}

func (a *AttributeData) publishAttributeGet(device, targetDevice *model.Device, targetDeviceNumber, deviceType, topicPrefix string, payload []byte) error {
	if a.downlinkBus == nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"system_error": "downlink bus not initialized",
		})
	}
	msg := &downlink.Message{
		DeviceID:       device.ID,
		DeviceNumber:   targetDeviceNumber,
		DeviceType:     deviceType,
		DeviceConfigID: a.getDeviceConfigID(targetDevice),
		Type:           downlink.MessageTypeAttributeGet,
		Data:           payload,
		TopicPrefix:    topicPrefix,
		MessageID:      "",
	}
	a.downlinkBus.PublishAttributeGet(msg)

	logrus.WithFields(logrus.Fields{
		"device_id":            device.ID,
		"target_device_id":     targetDevice.ID,
		"target_device_number": targetDeviceNumber,
		"device_type":          deviceType,
	}).Info("Attribute get sent via downlink")
	return nil
}
