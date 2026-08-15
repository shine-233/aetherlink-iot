// 文件用途：验证设备 MQTT 标准下行订阅主题的形状与保留命名空间契约。
// 安全职责：覆盖非法身份槽仍属于标准候选，防止授权逻辑回退到自定义 mapping。

package util

import "testing"

func TestValidateSubTopic(t *testing.T) {
	var cases = []struct {
		input string
		want  bool
	}{
		{"devices/telemetry", false},
		{"devices/telemetry/xxxxxx/+", false},
		{"devices/telemetry/control/xxxxxx/+", true},
		{"devices/attributes/set/xxxxxx/+", true},
		{"devices/attributes/set/+/+", false},
		{"devices/attributes/set//mode", false},
		{"devices/attributes/get/xxxxxx", true},
		{"devices/attributes/get/", false},
		{"devices/command/xxxxx/+", true},
		{"devices/command//reboot", false},
		{"ota/devices/infrom/xxxxx", false},
		{"ota/devices/inform/xxxxx", true},
		{"ota/devices/inform/", false},
		{"evices/attributes/response/xxxx/+", false},
		{"devices/attributes/response/xxxx/+", true},
		{"devices/attributes/response//ok", false},
		{"devices/event/response/xxxxxx/+", true},
		{"gateway/command//reboot", false},
		{"/down", false},
		{"", false},
		{"001/down", true},
	}

	for _, c := range cases {
		got := ValidateSubTopic(c.input)
		if got != c.want {
			t.Errorf("ValidateSubTopic(%q) == %v, want %v", c.input, got, c.want)
		}
	}
}
