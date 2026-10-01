// 文件用途：回归验证系统账号（root/plugin）认证在未配置密码时失败闭合。
// 核心逻辑：期望密码为空串时，空密码 CONNECT 不得通过（否则绕过全部 ACL）。

package aetherlink

import (
	"testing"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func TestAuthenticateMQTTSystemUserRejectsUnconfiguredPassword(t *testing.T) {
	prevLog := Log
	Log = zap.NewNop()
	prevRoot := viper.GetString("mqtt.password")
	prevPlugin := viper.GetString("mqtt.plugin_password")
	t.Cleanup(func() {
		Log = prevLog
		viper.Set("mqtt.password", prevRoot)
		viper.Set("mqtt.plugin_password", prevPlugin)
	})

	viper.Set("mqtt.password", "")
	viper.Set("mqtt.plugin_password", "")
	for _, user := range []string{"root", "plugin"} {
		handled, err := authenticateMQTTSystemUser(user, "")
		if !handled || err == nil {
			t.Fatalf("%s with unconfigured password: handled=%v err=%v, want handled denial", user, handled, err)
		}
	}

	viper.Set("mqtt.password", "root-secret")
	viper.Set("mqtt.plugin_password", "plugin-secret")
	cases := []struct {
		user, pass string
		ok         bool
	}{
		{"root", "root-secret", true},
		{"root", "root-secre", false},
		{"root", "", false},
		{"plugin", "plugin-secret", true},
		{"plugin", "root-secret", false},
	}
	for _, c := range cases {
		handled, err := authenticateMQTTSystemUser(c.user, c.pass)
		if !handled {
			t.Fatalf("%s must be handled as system user", c.user)
		}
		if (err == nil) != c.ok {
			t.Fatalf("%s/%q: err=%v, want ok=%v", c.user, c.pass, err, c.ok)
		}
	}

	if handled, err := authenticateMQTTSystemUser("device-1", ""); handled || err != nil {
		t.Fatalf("non-system user must not be handled: handled=%v err=%v", handled, err)
	}
}
