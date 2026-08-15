// 文件用途：维护 plugin\aetherlink\util\check_pub_topic_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package util

import "testing"

func TestValidateTopic(t *testing.T) {
	var cases = []struct {
		input string
		want  bool
	}{
		{"devices/telemetry", true},
		{"devices/status/device-001", true},
		{"devices/attributes/test", true},
		{"devices/event/test", true},
		{"gateway/attributes/test", true},
		{"gateway/event/test", true},
		{"devices/telemetry/test", false},
		{"devices/status", false},
		{"devices/status/", false},
		{"devices/status/device-001/extra", false},
		{"devices/test/telemetry", false},
		{"devices_test", false},
		{"devices/attributes/", false},
		{"devices/event/", false},
		{"gateway/attributes/", false},
		{"gateway/event/", false},
		{"/up", false},
		{"", false},
		{"xxxx/up", true},
	}

	for _, c := range cases {
		got := ValidateTopic(c.input)
		if got != c.want {
			t.Errorf("ValidateTopic(%q) == %v, want %v", c.input, got, c.want)
		}
	}
}
