package aetherlink

import (
	"context"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
)

func (t *AetherLinkPlugin) OnConnectedWrapper(pre server.OnConnected) server.OnConnected {
	return func(ctx context.Context, client server.Client) {
		// 先把认证阶段的 pending 绑定转为已连接：此后它只由 OnClosed 回收，
		// 不再参与 pending TTL 兜底扫描（见 hooks_auth.go 生命周期说明）。
		promoteMQTTAuthenticatedClientBinding(client)
		pre(ctx, client)
		publishMQTTDeviceOnlineStatus(client, "1", "online")
	}
}

func (t *AetherLinkPlugin) OnClosedWrapper(pre server.OnClosed) server.OnClosed {
	return func(ctx context.Context, client server.Client, err error) {
		// defer 必须先于 pre 注册：内层钩子或下方日志/上报 panic 时仍保证绑定被回收。
		// defer 在函数返回时才执行，下方断连日志与离线上报仍能读到绑定。
		defer forgetMQTTAuthenticatedClientBinding(client)
		pre(ctx, client, err)
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
	if err := DefaultMqttClient.WaitReady(context.Background()); err != nil {
		Log.Warn(
			"mqtt "+statusLabel+" status publish skipped before internal client readiness",
			zap.String("client_id", client.ClientOptions().ClientID),
			zap.String("device_id", deviceID),
			zap.Error(err),
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
