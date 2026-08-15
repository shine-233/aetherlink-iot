package aetherlink

import (
	"context"
	"errors"

	"github.com/DrmagicE/gmqtt/plugin/aetherlink/util"
	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
)

type mqttSubscribeDecision struct {
	allow                bool
	deviceID             string
	customMappingAllowed bool
}

type mqttSubscribePolicy struct {
	client         server.Client
	subTopic       string
	fallbackDevice string
}

func (t *AetherLinkPlugin) OnSubscribeWrapper(pre server.OnSubscribe) server.OnSubscribe {
	return func(ctx context.Context, client server.Client, req *server.SubscribeRequest) error {
		if err := pre(ctx, client, req); err != nil {
			return err
		}
		username := client.ClientOptions().Username
		// System users retain their existing unconditional subscribe bypass, including
		// for requests without decoded topics.
		if isMQTTSystemUser(username) {
			return nil
		}
		if req == nil || req.Subscribe == nil || len(req.Subscribe.Topics) == 0 {
			return errors.New("permission denied")
		}

		// A SUBSCRIBE packet can contain multiple topic filters; all of them must be
		// authorized or the broker rejects the whole request.
		for _, topic := range req.Subscribe.Topics {
			if err := handleMQTTSubscribeAuthorization(ctx, client, username, topic.Name); err != nil {
				return err
			}
		}
		return nil
	}
}

func handleMQTTSubscribeAuthorization(
	ctx context.Context,
	client server.Client,
	username string,
	subTopic string,
) error {
	decision := decideMQTTSubscribe(ctx, client, subTopic)
	logMQTTSubscribeDecision(client, username, subTopic, decision)
	if decision.allow {
		return nil
	}
	return errors.New("permission denied")
}

func decideMQTTSubscribe(ctx context.Context, client server.Client, subTopic string) mqttSubscribeDecision {
	return newMQTTSubscribePolicy(client, subTopic).decide(ctx)
}

func newMQTTSubscribePolicy(client server.Client, subTopic string) mqttSubscribePolicy {
	deviceID, _ := mqttAuthenticatedDeviceForClient(client)
	return mqttSubscribePolicy{
		client:         client,
		subTopic:       subTopic,
		fallbackDevice: deviceID,
	}
}

func (policy mqttSubscribePolicy) decide(ctx context.Context) mqttSubscribeDecision {
	device, err := loadActiveMQTTDevice(policy.fallbackDevice)
	if err != nil {
		forgetMQTTAuthenticatedClientBinding(policy.client)
		forgetMQTTAuthenticatedDevice(policy.client.ClientOptions().ClientID)
		return policy.deny(policy.fallbackDevice)
	}
	policy.fallbackDevice = device.ID
	return policy.decideTopicContract(ctx, device.DeviceNumber)
}

func (policy mqttSubscribePolicy) decideTopicContract(ctx context.Context, deviceNumber string) mqttSubscribeDecision {
	if decision, handled := policy.decideStandardTopic(deviceNumber); handled {
		return decision
	}
	return policy.decideMappedDownSubscribe(ctx)
}

func (policy mqttSubscribePolicy) decideStandardTopic(deviceNumber string) (mqttSubscribeDecision, bool) {
	if !util.IsStandardSubTopicCandidate(policy.subTopic) {
		return mqttSubscribeDecision{}, false
	}
	if !util.ValidateSubTopicForDevice(policy.subTopic, deviceNumber) {
		return policy.deny(policy.fallbackDevice), true
	}
	return policy.allowStandard(), true
}

func (policy mqttSubscribePolicy) allowStandard() mqttSubscribeDecision {
	return mqttSubscribeDecision{
		allow:    true,
		deviceID: policy.fallbackDevice,
	}
}

func (policy mqttSubscribePolicy) decideMappedDownSubscribe(ctx context.Context) mqttSubscribeDecision {
	deviceIDFromRedis, ok := policy.loadMappedSubscribeDeviceID()
	if !ok {
		return policy.deny(policy.fallbackDevice)
	}

	return policy.decideMappedDownSubscribeForDevice(ctx, deviceIDFromRedis)
}

func (policy mqttSubscribePolicy) loadMappedSubscribeDeviceID() (string, bool) {
	return mqttAuthenticatedDeviceForClient(policy.client)
}

func (policy mqttSubscribePolicy) decideMappedDownSubscribeForDevice(ctx context.Context, deviceID string) mqttSubscribeDecision {
	deviceConfigID, ok := policy.loadMappedSubscribeDeviceConfigID(deviceID)
	if !ok {
		return policy.deny(deviceID)
	}

	return policy.decideMappedDownSubscribeForConfig(ctx, deviceID, deviceConfigID)
}

func (policy mqttSubscribePolicy) decideMappedDownSubscribeForConfig(
	ctx context.Context,
	deviceID string,
	deviceConfigID string,
) mqttSubscribeDecision {
	if !policy.allowMappedDownSubscribe(ctx, deviceConfigID) {
		return policy.deny(deviceID)
	}

	return policy.allowMapped(deviceID)
}

func (policy mqttSubscribePolicy) allowMapped(deviceID string) mqttSubscribeDecision {
	return mqttSubscribeDecision{
		allow:                true,
		deviceID:             deviceID,
		customMappingAllowed: true,
	}
}

func (policy mqttSubscribePolicy) loadMappedSubscribeDeviceConfigID(deviceID string) (string, bool) {
	dev, err := loadActiveMQTTDevice(deviceID)
	if err != nil || dev.DeviceConfigID == nil {
		return "", false
	}
	return *dev.DeviceConfigID, true
}

func (policy mqttSubscribePolicy) allowMappedDownSubscribe(ctx context.Context, deviceConfigID string) bool {
	svc := NewTopicMapService()
	return svc.AllowDownSubscribe(ctx, deviceConfigID, policy.subTopic)
}

func (policy mqttSubscribePolicy) deny(deviceID string) mqttSubscribeDecision {
	return mqttSubscribeDecision{
		allow:    false,
		deviceID: deviceID,
	}
}

func logMQTTSubscribeDecision(client server.Client, username string, subTopic string, decision mqttSubscribeDecision) {
	if decision.customMappingAllowed {
		Log.Info("custom subscribe mapping accepted", zap.String("topic", subTopic))
	}
	if !decision.allow {
		Log.Warn("mqtt subscribe permission denied", zap.String("topic", subTopic), zap.String("client_id", client.ClientOptions().ClientID))
	}
	if decision.deviceID == "" {
		return
	}

	outcome := "ok"
	errMsg := ""
	code := "subscribe_ok"
	if !decision.allow {
		outcome = "deny"
		errMsg = "permission denied"
		code = "subscribe_denied"
	}
	meta := map[string]interface{}{}
	if decision.customMappingAllowed {
		meta["custom_mapping_allowed"] = true
	}

	recordMQTTDiagnosticForClient(client, mqttDiagnosticEvent{
		deviceID:  decision.deviceID,
		username:  username,
		action:    "subscribe",
		direction: "na",
		outcome:   outcome,
		error:     errMsg,
		code:      code,
		topic:     subTopic,
		meta:      meta,
	})
}
