package aetherlink

import (
	"context"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
)

func (t *AetherLinkPlugin) OnConnectedWrapper(pre server.OnConnected) server.OnConnected {
	return func(ctx context.Context, client server.Client) {
		pre(ctx, client)
		publishMQTTDeviceOnlineStatus(client, "1", "online")
	}
}

func (t *AetherLinkPlugin) OnClosedWrapper(pre server.OnClosed) server.OnClosed {
	return func(ctx context.Context, client server.Client, err error) {
		pre(ctx, client, err)
		defer forgetMQTTAuthenticatedClientBinding(client)
		Log.Info(
			"mqtt connection closed",
			zap.String("username", client.ClientOptions().Username),
			zap.String("client_id", client.ClientOptions().ClientID),
			zap.Error(err),
		)
		recordMQTTDisconnectDebugLog(client, err)
		publishMQTTDeviceOnlineStatus(client, "0", "offline")
	}
}

func recordMQTTDisconnectDebugLog(client server.Client, closeErr error) {
	opts := client.ClientOptions()
	if isMQTTSystemUser(opts.Username) {
		return
	}

	deviceID, ok := mqttAuthenticatedDeviceForClient(client)
	if !ok {
		return
	}

	outcome := "ok"
	code := "disconnect_normal"
	errorMessage := ""
	if closeErr != nil {
		outcome = "error"
		code = "disconnect_error"
		errorMessage = closeErr.Error()
	}

	meta := map[string]interface{}{
		"disconnect_reason": outcome,
	}
	if connectedAt := client.ConnectedAt(); connectedAt.Unix() > 0 {
		meta["connected_at"] = connectedAt.Format(time.RFC3339Nano)
	}

	recordMQTTDiagnosticForClient(client, mqttDiagnosticEvent{
		deviceID:  deviceID,
		username:  opts.Username,
		action:    "disconnect",
		direction: "na",
		outcome:   outcome,
		error:     errorMessage,
		code:      code,
		meta:      meta,
	})
}

func publishMQTTDeviceOnlineStatus(client server.Client, status string, statusLabel string) {
	if isMQTTSystemUser(client.ClientOptions().Username) {
		return
	}

	deviceID, ok := mqttAuthenticatedDeviceForClient(client)
	if !ok {
		Log.Warn(
			"mqtt "+statusLabel+" callback missing device id",
			zap.String("client_id", client.ClientOptions().ClientID),
		)
		return
	}
	if err := DefaultMqttClient.SendData("devices/status/"+deviceID, []byte(status)); err != nil {
		Log.Warn(
			"mqtt "+statusLabel+" status publish failed",
			zap.String("client_id", client.ClientOptions().ClientID),
			zap.String("device_id", deviceID),
			zap.Error(err),
		)
	}
}
