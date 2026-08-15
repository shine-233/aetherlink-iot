// 文件用途：维护 plugin\aetherlink\hooks_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"context"
	"errors"
	"testing"

	"github.com/DrmagicE/gmqtt/server"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
)

func TestAetherLinkPluginName(t *testing.T) {
	if Name != "aetherlink" {
		t.Fatalf("plugin name = %q, want aetherlink", Name)
	}
	if got := (&AetherLinkPlugin{}).Name(); got != Name {
		t.Fatalf("plugin Name() = %q, want %q", got, Name)
	}
}

func TestEnsureMQTTDeviceActiveRejectsUnboundOrDisabledDevices(t *testing.T) {
	tests := []struct {
		name   string
		device *Device
		ok     bool
	}{
		{name: "active enabled tenant device", device: &Device{ActivateFlag: "active", IsEnabled: "enabled", TenantID: "tenant-1"}, ok: true},
		{name: "inactive after physical unbind", device: &Device{ActivateFlag: "inactive", IsEnabled: "disabled"}},
		{name: "disabled active device", device: &Device{ActivateFlag: "active", IsEnabled: "disabled", TenantID: "tenant-1"}},
		{name: "missing tenant binding", device: &Device{ActivateFlag: "active", IsEnabled: "enabled"}},
		{name: "nil device", device: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureMQTTDeviceActive(tt.device)
			if tt.ok && err != nil {
				t.Fatalf("ensureMQTTDeviceActive() error = %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("ensureMQTTDeviceActive() should reject device")
			}
		})
	}
}

func TestAetherLinkHookWrapperRegistersAuthSubscribePublishAndLifecycleHooks(t *testing.T) {
	wrapper := (&AetherLinkPlugin{}).HookWrapper()

	if wrapper.OnBasicAuthWrapper == nil {
		t.Fatal("basic auth hook wrapper is not registered")
	}
	if wrapper.OnSubscribeWrapper == nil {
		t.Fatal("subscribe hook wrapper is not registered")
	}
	if wrapper.OnMsgArrivedWrapper == nil {
		t.Fatal("message arrived hook wrapper is not registered")
	}
	if wrapper.OnConnectedWrapper == nil {
		t.Fatal("connected hook wrapper is not registered")
	}
	if wrapper.OnClosedWrapper == nil {
		t.Fatal("closed hook wrapper is not registered")
	}
}

func TestAetherLinkHookWrappersPreservePreviousHooks(t *testing.T) {
	prevLog := Log
	Log = zap.NewNop()
	t.Cleanup(func() { Log = prevLog })

	ctx := context.Background()
	plugin := &AetherLinkPlugin{}
	sentinel := errors.New("previous hook")

	ctrl := gomock.NewController(t)
	client := server.NewMockClient(ctrl)
	rootOptions := &server.ClientOptions{Username: "root", ClientID: "root-client"}
	client.EXPECT().ClientOptions().Return(rootOptions).AnyTimes()

	connectedCalled := false
	plugin.OnConnectedWrapper(func(context.Context, server.Client) {
		connectedCalled = true
	})(ctx, client)
	if !connectedCalled {
		t.Fatal("OnConnectedWrapper did not call previous hook")
	}

	closedCalled := false
	plugin.OnClosedWrapper(func(context.Context, server.Client, error) {
		closedCalled = true
	})(ctx, client, nil)
	if !closedCalled {
		t.Fatal("OnClosedWrapper did not call previous hook")
	}

	subscribe := plugin.OnSubscribeWrapper(func(context.Context, server.Client, *server.SubscribeRequest) error {
		return sentinel
	})
	if err := subscribe(ctx, client, nil); !errors.Is(err, sentinel) {
		t.Fatalf("OnSubscribeWrapper error = %v, want sentinel", err)
	}

	msgArrived := plugin.OnMsgArrivedWrapper(func(context.Context, server.Client, *server.MsgArrivedRequest) error {
		return sentinel
	})
	if err := msgArrived(ctx, client, nil); !errors.Is(err, sentinel) {
		t.Fatalf("OnMsgArrivedWrapper error = %v, want sentinel", err)
	}
}

func TestDispatchMQTTUplinkRejectsBeforeCustomTopicMapping(t *testing.T) {
	previousLog := Log
	Log = zap.NewNop()
	resetPayloadSchemaResolver(t)
	t.Cleanup(func() { Log = previousLog })

	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{
			Fields: []PayloadSchemaFieldConstraint{
				{Name: "temp", Type: PayloadSchemaFieldTypeNumber, Required: true},
			},
		}, true
	})

	ctrl := gomock.NewController(t)
	client := server.NewMockClient(ctrl)
	client.EXPECT().ClientOptions().Return(&server.ClientOptions{ClientID: "mapped-client"})

	route := mqttDeviceRoute{deviceID: "dev-1", deviceConfigID: "cfg-1"}
	msg := mqttArrivedPayload{
		publishTopic: "custom/upstream/topic",
		rawPayload:   []byte(`{"humidity":60}`),
	}
	request := &server.MsgArrivedRequest{}

	err := route.dispatchMQTTUplink(context.Background(), client, request, msg, "device-user")
	if !errors.Is(err, errMQTTMessageDiscarded) {
		t.Fatalf("dispatchMQTTUplink() error = %v, want errMQTTMessageDiscarded", err)
	}
}
