// 文件用途：维护 cmd\gmqttd\command\start_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package command

import (
	"testing"

	"github.com/DrmagicE/gmqtt/config"
)

func TestNewStartCmdExposesStartCommandWithoutRunningBroker(t *testing.T) {
	cmd := NewStartCmd()

	if cmd.Use != "start" {
		t.Fatalf("command use = %q, want start", cmd.Use)
	}
	if cmd.Short != "Start gmqtt broker" {
		t.Fatalf("command short = %q", cmd.Short)
	}
	if cmd.Run == nil {
		t.Fatal("start command must install a Run handler")
	}
}

func TestGetListenersBuildsWebsocketServerWithoutOpeningTCPPort(t *testing.T) {
	listeners, websockets, err := GetListeners(config.Config{
		Listeners: []*config.ListenerConfig{
			{
				Address: "127.0.0.1:0",
				Websocket: &config.WebsocketOptions{
					Path: "/mqtt",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GetListeners returned error: %v", err)
	}
	if len(listeners) != 0 {
		t.Fatalf("websocket listener should not create TCP listeners, got %d", len(listeners))
	}
	if len(websockets) != 1 {
		t.Fatalf("websocket servers = %d, want 1", len(websockets))
	}
	if websockets[0].Server.Addr != "127.0.0.1:0" || websockets[0].Path != "/mqtt" {
		t.Fatalf("websocket server = addr %q path %q", websockets[0].Server.Addr, websockets[0].Path)
	}
}
