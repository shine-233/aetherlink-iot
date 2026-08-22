// 文件用途：验证上行发布 hook 的设备身份绑定 ACL。
// 安全职责：已认证设备只能向绑定自身身份的上行主题发布（devices/status 设备 ID、
// '+/up' 设备编号），跨设备身份主题必须拒绝；合法发布的 payload 仍重包为发布者自身。

package aetherlink

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/server"
	"github.com/golang/mock/gomock"
)

func TestMQTTMessageArrivedBindsUplinkTopicsToPublisherIdentity(t *testing.T) {
	const (
		deviceID      = "device-uuid-001"
		deviceNumber  = "device-001"
		foreignID     = "device-uuid-002"
		foreignNumber = "device-002"
		rawPayload    = `{"values":[1,2,3]}`
	)

	tests := []struct {
		name    string
		topic   string
		wantErr bool
	}{
		{name: "own status topic is accepted", topic: "devices/status/" + deviceID},
		{name: "foreign status topic is denied", topic: "devices/status/" + foreignID, wantErr: true},
		{name: "shared telemetry stays accepted", topic: "devices/telemetry"},
		{name: "message id response topic stays accepted", topic: "devices/command/response/msg-1"},
		{name: "uplink with own device number is accepted", topic: deviceNumber + "/up"},
		{name: "uplink with foreign device number is denied", topic: foreignNumber + "/up", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installMQTTSubscribeTestStore(t, &Device{
				ID:           deviceID,
				DeviceNumber: deviceNumber,
				TenantID:     "tenant-1",
				ActivateFlag: "active",
				IsEnabled:    "enabled",
			})

			ctrl := gomock.NewController(t)
			client := server.NewMockClient(ctrl)
			client.EXPECT().ClientOptions().Return(&server.ClientOptions{
				Username: "device-user",
				ClientID: "client-publish-1",
			}).AnyTimes()
			mqttAuthenticatedClientBindings.Store(client, mqttAuthenticatedClientBinding{deviceID: deviceID})
			t.Cleanup(func() { mqttAuthenticatedClientBindings.Delete(client) })

			msgArrived := (&AetherLinkPlugin{}).OnMsgArrivedWrapper(func(context.Context, server.Client, *server.MsgArrivedRequest) error {
				return nil
			})
			request := &server.MsgArrivedRequest{
				Publish: &packets.Publish{TopicName: []byte(tt.topic)},
				Message: &gmqtt.Message{Topic: tt.topic, Payload: []byte(rawPayload)},
			}
			err := msgArrived(context.Background(), client, request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("OnMsgArrivedWrapper() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if err.Error() != "permission denied" {
					t.Fatalf("OnMsgArrivedWrapper() error = %v, want permission denied", err)
				}
				return
			}
			// 放行的上行消息 payload 必须被重包为发布者自身的设备 ID。
			var envelope struct {
				DeviceID string `json:"device_id"`
			}
			if err := json.Unmarshal(request.Message.Payload, &envelope); err != nil {
				t.Fatalf("rewrapped payload is not JSON: %v", err)
			}
			if envelope.DeviceID != deviceID {
				t.Fatalf("payload device_id = %q, want %q", envelope.DeviceID, deviceID)
			}
		})
	}
}
