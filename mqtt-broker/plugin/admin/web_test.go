// 文件用途：维护 plugin\admin\web_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import "testing"

func TestAdminCredentialsMatchRequiresExplicitEnvironment(t *testing.T) {
	t.Setenv("GMQTT_ADMIN_USERNAME", "")
	t.Setenv("GMQTT_ADMIN_PASSWORD", "")

	if adminCredentialsMatch("admin", "admin") {
		t.Fatal("built-in default credentials must not be accepted")
	}

	t.Setenv("GMQTT_ADMIN_USERNAME", "operator")
	t.Setenv("GMQTT_ADMIN_PASSWORD", "change-me")

	if !adminCredentialsMatch("operator", "change-me") {
		t.Fatal("explicit admin environment credentials should be accepted")
	}
	if adminCredentialsMatch("admin", "admin") {
		t.Fatal("built-in default credentials should remain rejected")
	}
}
