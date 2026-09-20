// 文件用途：覆盖 SCADA / 移动端启动装配选项的 Go 测试。
// 核心逻辑：验证装配后 GroupApp.ScadaControl 与 GroupApp.Mobile 变为非 nil（接口不再
// fail closed），以及未配置二次确认密钥时的降级行为。
// 关键注意事项：装配会写全局 GroupApp，测试结束必须还原，否则会污染同包其它用例。
package app

import (
	"context"
	"testing"

	service "aetherlink-iot/backend/internal/service"

	"github.com/spf13/viper"
)

func restoreGroupApp(t *testing.T) {
	t.Helper()
	prevControl := service.GroupApp.ScadaControl
	prevMobile := service.GroupApp.Mobile
	t.Cleanup(func() {
		service.GroupApp.ScadaControl = prevControl
		service.GroupApp.Mobile = prevMobile
	})
}

func TestWithScadaMobileWiringAssignsServices(t *testing.T) {
	restoreGroupApp(t)
	service.GroupApp.ScadaControl = nil
	service.GroupApp.Mobile = nil

	cfg := viper.New()
	cfg.Set(scadaControlConfirmationSecretKey, "test-secret")

	app := &Application{Config: cfg}
	if err := WithScadaMobileWiring()(app); err != nil {
		t.Fatalf("WithScadaMobileWiring returned error: %v", err)
	}
	if service.GroupApp.ScadaControl == nil {
		t.Fatal("ScadaControl must be wired after the option runs")
	}
	if service.GroupApp.Mobile == nil {
		t.Fatal("Mobile must be wired after the option runs")
	}
	// 密钥已配置 → 能签发令牌；签发不出来说明配置没读到装配里。
	token, err := service.GroupApp.ScadaControl.IssueConfirmation(context.Background(), "t1", "d1", "w1", "open_valve", "u1")
	if err != nil || token == "" {
		t.Fatalf("IssueConfirmation = (%q, %v), want a token when the secret is configured", token, err)
	}
	if !service.GroupApp.Mobile.Capabilities(context.Background()).Commands {
		t.Fatal("mobile commands must be available after wiring")
	}
}

// TestWithScadaMobileWiringDegradesWithoutSecret 密钥缺失时服务仍接线，
// 但签发器不可用——这是"控制能用"与"危险操作点得动"的区别，不能混为一谈。
func TestWithScadaMobileWiringDegradesWithoutSecret(t *testing.T) {
	restoreGroupApp(t)
	service.GroupApp.ScadaControl = nil
	service.GroupApp.Mobile = nil

	cfg := viper.New() // 不设密钥

	app := &Application{Config: cfg}
	if err := WithScadaMobileWiring()(app); err != nil {
		t.Fatalf("WithScadaMobileWiring must not fail when the secret is absent: %v", err)
	}
	if service.GroupApp.ScadaControl == nil {
		t.Fatal("ScadaControl should still be wired (non-confirm commands remain usable)")
	}
	if _, err := service.GroupApp.ScadaControl.IssueConfirmation(context.Background(), "t1", "d1", "w1", "open_valve", "u1"); err == nil {
		t.Fatal("confirmation must be refused when the secret is not configured")
	}
}

// TestWithScadaMobileWiringNilConfigDoesNotPanic 纯单元测试装配路径（Config 为 nil）
// 不应该崩，否则所有不带配置的 Application 构造都会被拖累。
func TestWithScadaMobileWiringNilConfigDoesNotPanic(t *testing.T) {
	restoreGroupApp(t)
	service.GroupApp.ScadaControl = nil

	app := &Application{}
	if err := WithScadaMobileWiring()(app); err != nil {
		t.Fatalf("WithScadaMobileWiring with nil config: %v", err)
	}
	if service.GroupApp.ScadaControl == nil {
		t.Fatal("ScadaControl must be wired even without a config instance")
	}
}
