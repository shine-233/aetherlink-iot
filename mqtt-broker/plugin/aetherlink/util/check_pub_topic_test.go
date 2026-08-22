// 文件用途：验证上行发布主题的形状白名单与设备身份槽绑定契约。
// 安全职责：锁定 devices/status 设备 ID 槽与 '+/up' 设备编号槽的跨设备拒绝行为，
// 防止已认证设备向其它设备的身份主题注入消息。

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

func TestValidatePubTopicForDevice(t *testing.T) {
	const (
		deviceID     = "device-uuid-001"
		deviceNumber = "device-001"
	)

	var cases = []struct {
		name  string
		topic string
		devID string
		devNo string
		want  bool
	}{
		// devices/status 的 '+' 槽位承载平台设备 ID，必须等于发布者自身。
		{name: "status with own device id is allowed", topic: "devices/status/" + deviceID, devID: deviceID, devNo: deviceNumber, want: true},
		{name: "status with foreign device id is denied", topic: "devices/status/device-uuid-002", devID: deviceID, devNo: deviceNumber, want: false},
		{name: "status without publisher identity is denied", topic: "devices/status/" + deviceID, devID: "", devNo: deviceNumber, want: false},

		// message_id 类槽位不是设备身份，不做绑定（归因由 payload 重包兜底）。
		{name: "attribute message id slot stays open", topic: "devices/attributes/msg-1", devID: deviceID, devNo: deviceNumber, want: true},
		{name: "event message id slot stays open", topic: "devices/event/msg-2", devID: deviceID, devNo: deviceNumber, want: true},
		{name: "command response message id slot stays open", topic: "devices/command/response/msg-3", devID: deviceID, devNo: deviceNumber, want: true},
		{name: "gateway response message id slot stays open", topic: "gateway/command/response/msg-4", devID: deviceID, devNo: deviceNumber, want: true},

		// 无身份槽的共享主题保持放行，归因为发布者自身。
		{name: "shared telemetry stays allowed", topic: "devices/telemetry", devID: deviceID, devNo: deviceNumber, want: true},
		{name: "ota progress stays allowed", topic: "ota/devices/progress", devID: deviceID, devNo: deviceNumber, want: true},

		// '+/up' 首层与订阅侧 '{device_number}/down' 对应，必须等于发布者设备编号。
		{name: "uplink with own device number is allowed", topic: deviceNumber + "/up", devID: deviceID, devNo: deviceNumber, want: true},
		{name: "uplink with foreign device number is denied", topic: "device-002/up", devID: deviceID, devNo: deviceNumber, want: false},
		{name: "uplink without publisher number is denied", topic: deviceNumber + "/up", devID: deviceID, devNo: "", want: false},

		// 形状不匹配依旧拒绝；空身份槽 fail-closed。
		{name: "unknown shape is denied", topic: "devices/other", devID: deviceID, devNo: deviceNumber, want: false},
		{name: "empty status slot is denied", topic: "devices/status/", devID: deviceID, devNo: deviceNumber, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ValidatePubTopicForDevice(c.topic, c.devID, c.devNo)
			if got != c.want {
				t.Errorf("ValidatePubTopicForDevice(%q, %q, %q) == %v, want %v", c.topic, c.devID, c.devNo, got, c.want)
			}
		})
	}
}
