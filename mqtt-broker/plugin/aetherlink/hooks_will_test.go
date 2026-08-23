// 文件用途：验证 will message 钩子的授权边界，防止设备借遗嘱绕过发布白名单。
// 核心逻辑：用 miniredis 驱动认证期绑定键，断言白名单丢弃、系统放行、设备包裹与默认拒绝四类行为。
// 关键注意事项：will 是断线副作用路径，测试必须覆盖"无任何绑定"的默认拒绝分支。
// 重构建议：后续与 publish 白名单测试共享同一组主题夹具，避免两处漂移。
package aetherlink

import (
	"context"
	"encoding/json"
	"testing"

	gmqtt "github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/server"
	miniredis "github.com/alicebob/miniredis/v2"
	"go.uber.org/zap"
	"gopkg.in/redis.v5"
)

func setupWillHookTestRedis(t *testing.T) {
	t.Helper()
	server := miniredis.RunT(t)
	previousRedisCache := redisCache
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	redisCache = client
	prevLog := Log
	Log = zap.NewNop()
	t.Cleanup(func() {
		_ = client.Close()
		redisCache = previousRedisCache
		Log = prevLog
	})
}

func TestOnWillPublishDropsWillOutsidePublishWhitelist(t *testing.T) {
	setupWillHookTestRedis(t)
	plugin := &AetherLinkPlugin{}
	hook := plugin.OnWillPublishWrapper(nil)

	req := &server.WillMsgRequest{Message: &gmqtt.Message{
		Topic:   "other/tenant/private/topic",
		Payload: []byte(`{"evil":true}`),
	}}
	hook(context.Background(), "device-client", req)

	if req.Message != nil {
		t.Fatal("will on non-whitelisted topic must be dropped")
	}
}

func TestOnWillPublishKeepsSystemUserWillUnchanged(t *testing.T) {
	setupWillHookTestRedis(t)
	if err := rememberMQTTClientUsername("root-loopback", "root"); err != nil {
		t.Fatalf("remember system user binding: %v", err)
	}
	plugin := &AetherLinkPlugin{}
	hook := plugin.OnWillPublishWrapper(nil)

	payload := []byte(`raw-system-payload`)
	req := &server.WillMsgRequest{Message: &gmqtt.Message{
		Topic:   "devices/telemetry",
		Payload: payload,
	}}
	hook(context.Background(), "root-loopback", req)

	if req.Message == nil {
		t.Fatal("system user will must not be dropped")
	}
	if string(req.Message.Payload) != string(payload) {
		t.Fatalf("system user will payload = %q, want unchanged %q", req.Message.Payload, payload)
	}
}

func TestOnWillPublishWrapsDeviceWillPayloadWithDeviceID(t *testing.T) {
	setupWillHookTestRedis(t)
	if err := SetStr(mqttClientDeviceBindingKeyPrefix+"device-client", "device-001", mqttClientBindingTTL); err != nil {
		t.Fatalf("seed device binding: %v", err)
	}
	plugin := &AetherLinkPlugin{}
	hook := plugin.OnWillPublishWrapper(nil)

	req := &server.WillMsgRequest{Message: &gmqtt.Message{
		Topic:   "devices/telemetry",
		Payload: []byte(`{"temperature":25.5}`),
	}}
	hook(context.Background(), "device-client", req)

	if req.Message == nil {
		t.Fatal("device will on whitelisted topic with valid binding must not be dropped")
	}
	var wrapped map[string]interface{}
	if err := json.Unmarshal(req.Message.Payload, &wrapped); err != nil {
		t.Fatalf("device will payload is not wrapped JSON: %v (payload=%q)", err, req.Message.Payload)
	}
	if wrapped["device_id"] != "device-001" {
		t.Fatalf("wrapped device_id = %v, want device-001", wrapped["device_id"])
	}
}

func TestOnWillPublishDropsWillWithoutAuthenticatedBinding(t *testing.T) {
	setupWillHookTestRedis(t)
	plugin := &AetherLinkPlugin{}
	hook := plugin.OnWillPublishWrapper(nil)

	// 主题合法但查不到任何认证绑定（含系统账号）：必须默认拒绝。
	req := &server.WillMsgRequest{Message: &gmqtt.Message{
		Topic:   "devices/telemetry",
		Payload: []byte(`{"spoofed":"payload"}`),
	}}
	hook(context.Background(), "unknown-client", req)

	if req.Message != nil {
		t.Fatal("will without authenticated binding must be dropped by default")
	}
}

func TestRememberMQTTClientUsernameRejectsEmptyInputs(t *testing.T) {
	if err := rememberMQTTClientUsername("", "root"); err == nil {
		t.Fatal("empty clientID should be rejected")
	}
	if err := rememberMQTTClientUsername("client-1", " "); err == nil {
		t.Fatal("blank username should be rejected")
	}
}
