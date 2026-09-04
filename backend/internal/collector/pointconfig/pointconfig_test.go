// 文件用途：pointconfig 包自测——内置采集器点表解析/校验/协议归一化的独立契约。
// （采集器轮询链路的解析行为由 internal/collector 包测试覆盖，此处锁定保存链路入口。）
package pointconfig

import (
	"strings"
	"testing"
)

func TestValidateSnmpConfig(t *testing.T) {
	if err := ValidateSnmpConfig(`{"target":"10.0.0.5:161","community":"public","points":[{"key":"k","oid":"1.3.6.1"}]}`); err != nil {
		t.Fatalf("合法点表不应报错: %v", err)
	}
	for _, raw := range []string{
		``,
		`{}`,
		`{"target":"10.0.0.5:161","community":"public","points":[]}`,
		`{"target":"10.0.0.5:161","community":"public","points":[{"key":"k"}]}`,
	} {
		if err := ValidateSnmpConfig(raw); err == nil {
			t.Fatalf("非法点表应报错: %q", raw)
		}
	}
}

func TestValidateOpcuaConfig(t *testing.T) {
	if err := ValidateOpcuaConfig(`{"endpoint":"opc.tcp://10.0.0.6:4840","points":[{"key":"k","node":"ns=2;s=T"}]}`); err != nil {
		t.Fatalf("合法点表不应报错: %v", err)
	}
	// 非法 SecurityMode 在校验入口同样被拒。
	err := ValidateOpcuaConfig(`{"endpoint":"opc.tcp://10.0.0.6:4840","security_mode":"Bogus","points":[{"key":"k","node":"ns=2;s=T"}]}`)
	if err == nil || !strings.Contains(err.Error(), "SecurityMode") {
		t.Fatalf("非法 SecurityMode 应报错，实际 %v", err)
	}
}

func TestNormalizeProtocolType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"snmp", "SNMP"},
		{" OPCUA ", "OPCUA"},
		{"opcua", "OPCUA"},
		{"MQTT", "MQTT"},  // 非内置协议原样返回
		{"MODBUS", "MODBUS"}, // 非内置协议原样返回
	}
	for _, tc := range cases {
		if got := NormalizeProtocolType(tc.in); got != tc.want {
			t.Fatalf("NormalizeProtocolType(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
