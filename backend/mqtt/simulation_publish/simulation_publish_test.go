// 文件用途：验证模拟发布客户端的连接参数构造和 payload 转换逻辑。
// 核心逻辑：绕开真实 MQTT Broker，只断言 host/port、账号、clientID 和 payload 字节内容。
// 关键注意事项：测试用于保护模拟发布工具的输入约定，不代表真实网络发布链路已经验证。
// 重构建议：后续可通过 mock transport 或本地测试 Broker 补充连接失败和发布失败场景。
package simulationpublish

import "testing"

func TestBuildSimulationClientOptionsUsesExplicitBrokerAuthAndSessionPolicy(t *testing.T) {
	opts := buildSimulationClientOptions("127.0.0.1", "1883", "device-user", "device-pass", "sim-client-001")

	if len(opts.Servers) != 1 || opts.Servers[0].String() != "tcp://127.0.0.1:1883" {
		t.Fatalf("servers = %v, want tcp://127.0.0.1:1883", opts.Servers)
	}
	if opts.Username != "device-user" || opts.Password != "device-pass" {
		t.Fatalf("credentials = %q/%q", opts.Username, opts.Password)
	}
	if opts.ClientID != "sim-client-001" {
		t.Fatalf("client id = %q", opts.ClientID)
	}
	if !opts.CleanSession {
		t.Fatal("simulation publish should use a clean session")
	}
	if opts.ResumeSubs {
		t.Fatal("simulation publish should not resume subscriptions")
	}
	if opts.Order {
		t.Fatal("simulation publish should disable ordered delivery")
	}
}

func TestSimulationPayloadPreservesBusinessJSONBody(t *testing.T) {
	payload := `{"temperature":26.5,"humidity":63}`

	if got := string(simulationPayload(payload)); got != payload {
		t.Fatalf("payload bytes = %s", got)
	}
}
