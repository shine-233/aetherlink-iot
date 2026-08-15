package aetherlink

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type mqttVoucherPayload struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

var mqttAuthenticatedClientBindings sync.Map

type mqttAuthenticatedClientBinding struct {
	deviceID           string
	deviceStateVersion time.Time
}

func (t *AetherLinkPlugin) OnBasicAuthWrapper(pre server.OnBasicAuth) server.OnBasicAuth {
	return func(ctx context.Context, client server.Client, req *server.ConnectRequest) (err error) {
		err = pre(ctx, client, req)
		if err != nil {
			Log.Error(err.Error())
			return err
		}

		username := string(req.Connect.Username)
		password := string(req.Connect.Password)
		clientID := string(req.Connect.ClientID)
		if handled, err := authenticateMQTTSystemUser(username, password); handled {
			return err
		}

		logMQTTAuthStart(username, clientID)

		voucher, err := buildMQTTVoucher(username, password)
		if err != nil {
			return err
		}
		device, err := GetDeviceByVoucher(voucher)
		if err != nil {
			handleMQTTAuthFailure(username, password, clientID, err)
			return err
		}
		if err := ensureMQTTDeviceActive(device); err != nil {
			forgetMQTTDeviceLookup(voucher, device)
			handleMQTTAuthFailure(username, password, clientID, err)
			return err
		}

		handleMQTTAuthSuccess(device, username, clientID)
		err = rememberMQTTAuthenticatedDevice(client, clientID, device)
		if err != nil {
			Log.Error(err.Error())
			return err
		}
		return nil
	}
}

func authenticateMQTTSystemUser(username string, providedPassword string) (bool, error) {
	var expectedPassword string
	switch username {
	case "root":
		expectedPassword = viper.GetString("mqtt.password")
	case "plugin":
		expectedPassword = viper.GetString("mqtt.plugin_password")
	default:
		return false, nil
	}

	if providedPassword == expectedPassword {
		return true, nil
	}

	err := errors.New("password error")
	Log.Warn(err.Error())
	return true, err
}

func logMQTTAuthStart(username string, clientID string) {
	Log.Info(
		"mqtt auth start",
		zap.String("username", username),
		zap.String("client_id", clientID),
	)
}

func buildMQTTVoucher(username string, password string) (string, error) {
	if password != "" {
		v, err := json.Marshal(mqttVoucherPayload{
			Username: username,
			Password: password,
		})
		if err != nil {
			return "", err
		}
		return string(v), nil
	}

	v, err := json.Marshal(mqttVoucherPayload{
		Username: username,
	})
	if err != nil {
		return "", err
	}
	return string(v), nil
}

func handleMQTTAuthFailure(username string, password string, clientID string, authErr error) {
	Log.Warn(
		"mqtt auth failed",
		zap.String("client_id", clientID),
		zap.Error(authErr),
	)

	if isMQTTSystemUser(username) || password == "" {
		return
	}

	fb, fbErr := json.Marshal(mqttVoucherPayload{Username: username})
	if fbErr != nil {
		return
	}
	fallbackVoucher := string(fb)
	if dev, derr := GetDeviceByVoucher(fallbackVoucher); derr == nil && dev != nil {
		recordMQTTDiagnosticEvent(mqttDiagnosticEvent{
			deviceID:  dev.ID,
			clientID:  clientID,
			username:  username,
			action:    "auth",
			direction: "na",
			outcome:   "deny",
			error:     authErr.Error(),
			code:      "auth_denied",
		})
	}
}

func handleMQTTAuthSuccess(device *Device, username string, clientID string) {
	Log.Info(
		"mqtt auth passed",
		zap.String("client_id", clientID),
		zap.String("device_id", device.ID),
	)
	recordMQTTDiagnosticEvent(mqttDiagnosticEvent{
		deviceID:  device.ID,
		clientID:  clientID,
		username:  username,
		action:    "auth",
		direction: "na",
		outcome:   "ok",
		code:      "auth_ok",
	})
}

func rememberMQTTAuthenticatedDevice(client server.Client, clientID string, device *Device) error {
	if device == nil {
		return errors.New("mqtt authenticated device is nil")
	}
	deviceID := strings.TrimSpace(device.ID)
	if err := SetStr("mqtt_client_id_"+clientID, deviceID, 48*time.Hour); err != nil {
		return err
	}
	if client != nil {
		binding := mqttAuthenticatedClientBinding{deviceID: deviceID}
		if device.UpdateAt != nil {
			binding.deviceStateVersion = device.UpdateAt.UTC()
		} else if device.CreatedAt != nil {
			binding.deviceStateVersion = device.CreatedAt.UTC()
		}
		mqttAuthenticatedClientBindings.Store(client, binding)
	}
	return nil
}

func mqttAuthenticatedDeviceForClient(client server.Client) (string, bool) {
	binding, ok := mqttAuthenticatedBindingForClient(client)
	return binding.deviceID, ok
}

func mqttAuthenticatedBindingForClient(client server.Client) (mqttAuthenticatedClientBinding, bool) {
	if client == nil {
		return mqttAuthenticatedClientBinding{}, false
	}
	value, ok := mqttAuthenticatedClientBindings.Load(client)
	if !ok {
		return mqttAuthenticatedClientBinding{}, false
	}
	binding, bindingOK := value.(mqttAuthenticatedClientBinding)
	if bindingOK {
		binding.deviceID = strings.TrimSpace(binding.deviceID)
		return binding, binding.deviceID != ""
	}
	// Keep compatibility with bindings created by an older plugin instance during
	// an in-process reload. A zero version is conservatively revocable.
	deviceID, ok := value.(string)
	deviceID = strings.TrimSpace(deviceID)
	return mqttAuthenticatedClientBinding{deviceID: deviceID}, ok && deviceID != ""
}

func forgetMQTTAuthenticatedClientBinding(client server.Client) {
	if client != nil {
		mqttAuthenticatedClientBindings.Delete(client)
	}
}

func forgetMQTTAuthenticatedDevice(clientID string) {
	if strings.TrimSpace(clientID) == "" {
		return
	}
	_ = DelKey("mqtt_client_id_" + clientID)
}

func forgetMQTTDeviceLookup(voucher string, device *Device) {
	if strings.TrimSpace(voucher) != "" {
		_ = DelKey(voucher)
	}
	if device != nil && strings.TrimSpace(device.ID) != "" {
		_ = DelKey(device.ID)
	}
}

func ensureMQTTDeviceActive(device *Device) error {
	if device == nil {
		return errors.New("device not found")
	}
	if !strings.EqualFold(strings.TrimSpace(device.ActivateFlag), "active") ||
		!strings.EqualFold(strings.TrimSpace(device.IsEnabled), "enabled") ||
		strings.TrimSpace(device.TenantID) == "" {
		return errors.New("device is inactive or disabled")
	}
	return nil
}

func loadActiveMQTTDevice(deviceID string) (*Device, error) {
	device, err := GetDeviceById(strings.TrimSpace(deviceID))
	if err != nil {
		return nil, err
	}
	if err := ensureMQTTDeviceActive(device); err != nil {
		return nil, err
	}
	return device, nil
}

func isMQTTSystemUser(username string) bool {
	return username == "root" || username == "plugin"
}
