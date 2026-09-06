// 文件用途：SNMP 采集器单测——点表解析校验、内嵌 UDP SNMP agent 全链路采集、
// 错误码路径与不可达目标 fail-closed。
package collector

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/collector/pointconfig"
	"aetherlink-iot/backend/internal/snmp"
)

func TestParseSnmpConfigValidation(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"空配置", "", "为空"},
		{"坏JSON", "{bad", "非法 JSON"},
		{"缺target", `{"community":"public","points":[{"key":"k","oid":"1.3.6.1"}]}`, "target"},
		{"缺community", `{"target":"127.0.0.1:161","points":[{"key":"k","oid":"1.3.6.1"}]}`, "community"},
		{"缺points", `{"target":"127.0.0.1:161","community":"public"}`, "points"},
		{"点位缺oid", `{"target":"127.0.0.1:161","community":"public","points":[{"key":"k"}]}`, "key/oid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pointconfig.ParseSnmpConfig(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望错误含 %q，实际 %v", tc.wantErr, err)
			}
		})
	}
	cfg, err := pointconfig.ParseSnmpConfig(`{"target":"10.0.0.5:161","community":"public","points":[{"key":"temperature","oid":"1.3.6.1.4.1.99.1"}]}`)
	if err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
	if cfg.Target != "10.0.0.5:161" || len(cfg.Points) != 1 || cfg.Points[0].Key != "temperature" {
		t.Fatalf("解析结果不符: %+v", cfg)
	}
}

// startSnmpTestAgent 启动内嵌 UDP SNMP agent：对任意请求回固定 GetResponse。
// 返回监听地址与停车函数。
func startSnmpTestAgent(t *testing.T, binds []snmp.VarBind, respErrorStatus int) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 65536)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			_ = n
			resp, err := snmp.BuildGetResponse("public", 1, respErrorStatus, 1, binds)
			if err != nil {
				continue
			}
			_, _ = conn.WriteTo(resp, addr)
		}
	}()
	return conn.LocalAddr().String(), func() {
		_ = conn.Close()
		<-done
	}
}

func TestSnmpPollerEndToEnd(t *testing.T) {
	binds := []snmp.VarBind{
		{OID: "1.3.6.1.2.1.1.3.0", Value: snmp.IntegerValue(12345)},
		{OID: "1.3.6.1.2.1.1.1.0", Value: snmp.OctetStringValue("demo-agent")},
	}
	addr, stop := startSnmpTestAgent(t, binds, 0)
	defer stop()

	cfgJSON := `{"target":"` + addr + `","community":"public","points":[` +
		`{"key":"uptime","oid":"1.3.6.1.2.1.1.3.0"},` +
		`{"key":"agent_name","oid":"1.3.6.1.2.1.1.1.0"}]}`

	poller := SnmpPoller{}
	values, err := poller.Poll(context.Background(), deviceTarget{DeviceID: "dev-1", ConfigJSON: cfgJSON})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if v, ok := values["uptime"].(float64); !ok || v != 12345 {
		t.Fatalf("uptime=%v", values["uptime"])
	}
	if v, ok := values["agent_name"].(string); !ok || v != "demo-agent" {
		t.Fatalf("agent_name=%v", values["agent_name"])
	}
	if poller.Protocol() != "snmp" {
		t.Fatalf("protocol=%q", poller.Protocol())
	}
}

func TestSnmpPollerAgentErrorStatus(t *testing.T) {
	binds := []snmp.VarBind{{OID: "1.3.6.1.2.1.1.1.0", Value: snmp.OctetStringValue("x")}}
	addr, stop := startSnmpTestAgent(t, binds, 2)
	defer stop()

	cfgJSON := `{"target":"` + addr + `","community":"public","points":[{"key":"name","oid":"1.3.6.1.2.1.1.1.0"}]}`
	_, err := SnmpPoller{}.Poll(context.Background(), deviceTarget{ConfigJSON: cfgJSON})
	if err == nil || !strings.Contains(err.Error(), "status=2") {
		t.Fatalf("期望代理错误码路径，实际 %v", err)
	}
}

func TestSnmpPollerUnreachableTarget(t *testing.T) {
	// 不可达端口 + 200ms 预算：必须超时失败且不 panic。
	cfgJSON := `{"target":"127.0.0.1:1","community":"public","timeout_ms":200,"points":[{"key":"k","oid":"1.3.6.1"}]}`
	start := time.Now()
	_, err := SnmpPoller{}.Poll(context.Background(), deviceTarget{ConfigJSON: cfgJSON})
	if err == nil {
		t.Fatal("不可达目标应报错")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("超时预算未生效: %v", elapsed)
	}
}
