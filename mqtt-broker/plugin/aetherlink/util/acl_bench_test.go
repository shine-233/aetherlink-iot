// 文件用途：设备主题 ACL 校验热路径基准。ValidatePubTopicForDevice 每条上行 PUBLISH 调用一次，
// IsStandardSubTopicCandidate + ValidateSubTopicForDevice 每个 SUBSCRIBE 主题调用一次，
// ValidateTopic 用于遗嘱主题校验。覆盖命中（首个/末个模式）与全表未命中。
package util

import "testing"

var aclSink bool

func BenchmarkValidatePubTopicForDevice(b *testing.B) {
	cases := []struct{ name, topic string }{
		{"telemetry", "devices/telemetry"},
		{"status", "devices/status/dev-0001"},
		{"plusUp_last", "SN0001/up"},
		{"miss", "other/room/1/temp"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				aclSink = ValidatePubTopicForDevice(c.topic, "dev-0001", "SN0001")
			}
		})
	}
}

func BenchmarkValidateSubTopicForDevice(b *testing.B) {
	cases := []struct{ name, topic string }{
		{"control_first", "devices/telemetry/control/SN0001"},
		{"down_late", "SN0001/down"},
		{"miss", "other/room/1/temp"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				aclSink = IsStandardSubTopicCandidate(c.topic) && ValidateSubTopicForDevice(c.topic, "SN0001")
			}
		})
	}
}

func BenchmarkValidateTopic(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		aclSink = ValidateTopic("devices/event/msg-1")
	}
}
