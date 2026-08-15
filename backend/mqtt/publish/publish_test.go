// 文件用途：验证 MQTT 发布客户端的连接参数、重连策略和主题拼接约定。
// 核心逻辑：直接测试可纯函数化的配置构造与 topic helper，避免依赖真实 Broker。
// 关键注意事项：这些测试锁定 AetherLink 发布端 clientID、OTA 下行主题和共享在线状态主题，变更协议需同步更新。
// 重构建议：后续可补充发布失败路径的 mock MQTT client 测试，覆盖 token error 和初始化顺序。
package publish

import (
	"testing"
	"time"

	config "aetherlink-iot/backend/mqtt"
)

func TestBuildPublishClientOptionsUsesAetherLinkBrokerAndReconnectPolicy(t *testing.T) {
	config.MqttConfig.Broker = "mqtt-broker.local:1883"
	config.MqttConfig.User = "aetherlink"
	config.MqttConfig.Pass = "secret"

	opts := buildPublishClientOptions("aetherlink-go-pub-test")

	if len(opts.Servers) != 1 || opts.Servers[0].String() != "tcp://mqtt-broker.local:1883" {
		t.Fatalf("servers = %v, want tcp://mqtt-broker.local:1883", opts.Servers)
	}
	if opts.Username != "aetherlink" || opts.Password != "secret" {
		t.Fatalf("mqtt credentials = %q/%q", opts.Username, opts.Password)
	}
	if opts.ClientID != "aetherlink-go-pub-test" {
		t.Fatalf("client id = %q", opts.ClientID)
	}
	if !opts.CleanSession || !opts.ResumeSubs || !opts.AutoReconnect {
		t.Fatalf("expected clean session, resumed subscriptions, and auto reconnect")
	}
	if opts.ConnectRetryInterval != 5*time.Second {
		t.Fatalf("connect retry interval = %s", opts.ConnectRetryInterval)
	}
	if opts.MaxReconnectInterval != 20*time.Second {
		t.Fatalf("max reconnect interval = %s", opts.MaxReconnectInterval)
	}
	if opts.Order {
		t.Fatal("publish client should not require ordered message delivery")
	}
}

func TestPublishTopicMatchesOTADownlinkConvention(t *testing.T) {
	config.MqttConfig.OTA.PublishTopic = "ota/devices/package/"

	if got := otaAddressTopic("device-001"); got != "ota/devices/package/device-001" {
		t.Fatalf("ota topic = %q", got)
	}
}
