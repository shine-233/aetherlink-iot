// 文件用途：OPC UA 采集器单测——点表解析与归一化、值转换表、连接失败 fail-closed 路径。
// 真实 opc.tcp 服务器读写属环境绑定 E2E，接入时按 opcua 包既定口径补验。
package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/gopcua/opcua/ua"
)

func TestParseOpcuaConfigValidation(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"空配置", "", "为空"},
		{"坏JSON", "{bad", "非法 JSON"},
		{"缺points", `{"endpoint":"opc.tcp://127.0.0.1:4840"}`, "points"},
		{"点位缺node", `{"endpoint":"opc.tcp://127.0.0.1:4840","points":[{"key":"k"}]}`, "key/node"},
		{"endpoint前缀非法", `{"endpoint":"http://x","points":[{"key":"k","node":"ns=2;s=T"}]}`, "opc.tcp://"},
		{"SecurityMode非法", `{"endpoint":"opc.tcp://127.0.0.1:4840","security_mode":"Bogus","points":[{"key":"k","node":"ns=2;s=T"}]}`, "SecurityMode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOpcuaConfig(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望错误含 %q，实际 %v", tc.wantErr, err)
			}
		})
	}
	cfg, err := parseOpcuaConfig(`{"endpoint":"opc.tcp://127.0.0.1:4840","points":[{"key":"temperature","node":"ns=2;s=Demo.Temp"}]}`)
	if err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
	// Normalize 默认值：SecurityMode=None / TimeoutSeconds=10。
	if cfg.SecurityMode != "None" || cfg.TimeoutSeconds != 10 {
		t.Fatalf("默认值未归一化: %+v", cfg.Config)
	}
	if cfg.Points[0].Node != "ns=2;s=Demo.Temp" {
		t.Fatalf("点位解析不符: %+v", cfg.Points)
	}
}

func TestOpcuaValueConversion(t *testing.T) {
	mk := func(v interface{}) *ua.DataValue {
		variant, err := ua.NewVariant(v)
		if err != nil {
			t.Fatalf("NewVariant(%v): %v", v, err)
		}
		return &ua.DataValue{Value: variant}
	}
	cases := []struct {
		name string
		dv   *ua.DataValue
		want interface{}
		ok   bool
	}{
		{"float64", mk(26.5), 26.5, true},
		{"int32→float64", mk(int32(7)), float64(7), true},
		{"uint16→float64", mk(uint16(9)), float64(9), true},
		{"bool", mk(true), "true", true},
		{"string", mk("ok"), "ok", true},
		{"bytes", mk([]byte("raw")), "raw", true},
		{"nil DataValue", nil, nil, false},
		{"nil Variant", &ua.DataValue{}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := opcuaValue(tc.dv)
			if ok != tc.ok {
				t.Fatalf("ok=%v want=%v (got=%v)", ok, tc.ok, got)
			}
			if ok && got != tc.want {
				t.Fatalf("got=%v(%T) want=%v(%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

func TestOpcuaPollerConnectFailureFailClosed(t *testing.T) {
	poller := NewOpcuaPoller(nil)
	cfgJSON := `{"endpoint":"opc.tcp://127.0.0.1:1","timeout_seconds":1,"points":[{"key":"k","node":"ns=2;s=T"}]}`
	if _, err := poller.Poll(context.Background(), deviceTarget{DeviceID: "dev-opc-1", ConfigJSON: cfgJSON}); err == nil {
		t.Fatal("连接失败必须返回错误")
	}
	// 失败后连接条目不残留（fail-closed，不缓存半死连接）。
	poller.mu.Lock()
	n := len(poller.conns)
	poller.mu.Unlock()
	if n != 0 {
		t.Fatalf("失败连接应被清理，残留 %d 条", n)
	}
	if poller.Protocol() != "opcua" {
		t.Fatalf("protocol=%q", poller.Protocol())
	}
}
