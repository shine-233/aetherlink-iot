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

func TestValidateSnmpConfigV3(t *testing.T) {
	// 合法 v3 点表：v3_user 非空即 v3，community 可缺省。
	valid := `{"target":"10.0.0.5:161","v3_user":"authUser","auth_proto":"sha","auth_passphrase":"authkey123","points":[{"key":"k","oid":"1.3.6.1"}]}`
	cfg, err := ParseSnmpConfig(valid)
	if err != nil {
		t.Fatalf("合法 v3 点表不应报错: %v", err)
	}
	if !cfg.V3Enabled() || cfg.V3User != "authUser" || cfg.V3AuthProto != "sha" || cfg.V3AuthPassphrase != "authkey123" {
		t.Fatalf("v3 字段解析不符: %+v", cfg)
	}
	// md5 协议大小写不敏感与别名。
	if _, err := ParseSnmpConfig(`{"target":"t:1","v3_user":"u","auth_proto":"HMAC-MD5","auth_passphrase":"authkey123","community":"c","points":[{"key":"k","oid":"1.3"}]}`); err != nil {
		t.Fatalf("md5 别名不应报错: %v", err)
	}

	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"缺认证口令", `{"target":"t:1","community":"c","v3_user":"u","auth_proto":"sha","points":[{"key":"k","oid":"1.3"}]}`, "auth_passphrase 必填"},
		{"口令过短", `{"target":"t:1","community":"c","v3_user":"u","auth_proto":"sha","auth_passphrase":"short","points":[{"key":"k","oid":"1.3"}]}`, "至少 8 字符"},
		{"未知认证协议", `{"target":"t:1","community":"c","v3_user":"u","auth_proto":"des","auth_passphrase":"authkey123","points":[{"key":"k","oid":"1.3"}]}`, "不支持的认证协议"},
		{"缺认证协议", `{"target":"t:1","community":"c","v3_user":"u","auth_passphrase":"authkey123","points":[{"key":"k","oid":"1.3"}]}`, "不支持的认证协议"},
		{"priv加密未实现", `{"target":"t:1","community":"c","v3_user":"u","auth_proto":"sha","auth_passphrase":"authkey123","priv_proto":"aes","points":[{"key":"k","oid":"1.3"}]}`, "暂不支持"},
		{"priv口令悬空", `{"target":"t:1","community":"c","v3_user":"u","auth_proto":"sha","auth_passphrase":"authkey123","priv_passphrase":"privkey123","points":[{"key":"k","oid":"1.3"}]}`, "不得填写 priv_passphrase"},
		{"v3字段缺v3_user", `{"target":"t:1","community":"c","auth_proto":"sha","points":[{"key":"k","oid":"1.3"}]}`, "缺少 v3_user"},
		{"v3字段缺v3_user2", `{"target":"t:1","community":"c","auth_passphrase":"authkey123","points":[{"key":"k","oid":"1.3"}]}`, "缺少 v3_user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSnmpConfig(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望错误含 %q，实际 %v", tc.wantErr, err)
			}
		})
	}
	// v2c 路径不回归：无 v3 字段时 community 仍必填。
	if _, err := ParseSnmpConfig(`{"target":"t:1","points":[{"key":"k","oid":"1.3"}]}`); err == nil || !strings.Contains(err.Error(), "community 必填") {
		t.Fatalf("v2c community 校验不回归, err=%v", err)
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
