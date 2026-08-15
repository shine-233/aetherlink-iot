// 文件用途：维护 plugin\aetherlink\topicmap_matcher_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import "testing"

func TestTopicMapMatcherExtractsNormalizedDownlinkDeviceNumbers(t *testing.T) {
	tests := []struct {
		topic string
		want  string
	}{
		{"devices/telemetry/control/dev-001", "dev-001"},
		{"devices/attributes/set/dev-002/mode", "dev-002"},
		{"gateway/command/dev-003/reboot", "dev-003"},
		{"ota/devices/inform/dev-004", "dev-004"},
	}

	for _, tt := range tests {
		got, ok := TryExtractDeviceNumberFromNormalized(tt.topic)
		if !ok {
			t.Fatalf("TryExtractDeviceNumberFromNormalized(%q) did not match", tt.topic)
		}
		if got != tt.want {
			t.Fatalf("device number = %q, want %q", got, tt.want)
		}
	}
}

func TestTopicMapMatcherRejectsMultiLevelWildcardAndRendersVariables(t *testing.T) {
	if _, ok := compileSourcePattern("devices/#"); ok {
		t.Fatal("compileSourcePattern should reject multi-level wildcard")
	}
	if _, ok := compileTargetPattern("gateway/#"); ok {
		t.Fatal("compileTargetPattern should reject multi-level wildcard")
	}

	got := renderTopicFromTemplate("devices/{device_number}/command/{method}", map[string]string{
		"device_number": "dev-1",
		"method":        "reboot",
	})
	if got != "devices/dev-1/command/reboot" {
		t.Fatalf("rendered topic = %q", got)
	}
}
