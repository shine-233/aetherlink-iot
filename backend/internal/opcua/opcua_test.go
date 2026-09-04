// 文件用途：OPC UA 包装层自有逻辑单测——配置校验、归一化、安全模式映射与选项组装、
// NodeID 解析、连接前读取应报错。
package opcua

import (
	"context"
	"testing"

	"github.com/gopcua/opcua/ua"
)

func TestValidateConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		bad  bool
	}{
		{"空端点", Config{}, true},
		{"非 opc.tcp 端点", Config{Endpoint: "http://x:4840"}, true},
		{"合法默认", Config{Endpoint: "opc.tcp://127.0.0.1:4840"}, false},
		{"未知安全模式", Config{Endpoint: "opc.tcp://x:4840", SecurityMode: "TLS"}, true},
		{"SignAndEncrypt 合法", Config{Endpoint: "opc.tcp://x:4840", SecurityMode: "SignAndEncrypt"}, false},
	}
	for _, c := range cases {
		err := Validate(c.cfg)
		if c.bad && err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
		if !c.bad && err != nil {
			t.Errorf("%s: 不应报错: %v", c.name, err)
		}
	}
}

func TestNormalizeDefaults(t *testing.T) {
	cfg := Normalize(Config{Endpoint: "opc.tcp://x:4840"})
	if cfg.SecurityMode != "None" || cfg.TimeoutSeconds != 10 || cfg.ApplicationName == "" {
		t.Fatalf("默认值未填充: %+v", cfg)
	}
}

func TestSecurityModeMapping(t *testing.T) {
	if securityMode("None") != ua.MessageSecurityModeNone {
		t.Fatal("None 映射错")
	}
	if securityMode("Sign") != ua.MessageSecurityModeSign {
		t.Fatal("Sign 映射错")
	}
	if securityMode("SignAndEncrypt") != ua.MessageSecurityModeSignAndEncrypt {
		t.Fatal("SignAndEncrypt 映射错")
	}
	if securityMode("garbage") != ua.MessageSecurityModeNone {
		t.Fatal("未知模式应回落 None（配合 Validate 前置拦截）")
	}
}

func TestOptionsBuild(t *testing.T) {
	anon := options(Config{Endpoint: "opc.tcp://x:4840"})
	if len(anon) < 3 {
		t.Fatalf("匿名选项过少: %d", len(anon))
	}
	auth := options(Config{Endpoint: "opc.tcp://x:4840", Username: "u", Password: "p"})
	if len(auth) < 3 {
		t.Fatalf("带认证选项过少: %d", len(auth))
	}
}

func TestParseNodeIDAcceptance(t *testing.T) {
	for _, good := range []string{"ns=2;s=Device1.Temperature", "i=85", "ns=0;i=2253"} {
		if _, err := ua.ParseNodeID(good); err != nil {
			t.Errorf("ParseNodeID(%q) 应成功: %v", good, err)
		}
	}
}

func TestReadBeforeConnectRejected(t *testing.T) {
	c, err := NewClient(Config{Endpoint: "opc.tcp://127.0.0.1:4840"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReadValue(context.Background(), "i=85"); err == nil {
		t.Fatal("未连接时 Read 应报错")
	}
}
