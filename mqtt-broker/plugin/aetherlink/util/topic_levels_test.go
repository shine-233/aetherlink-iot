// 文件用途：栈上分层 ACL 匹配与 strings.Split 参考实现的等价性测试（含超深主题溢出路径）。
package util

import (
	"strings"
	"testing"
)

// refValidateSubTopicForDevice 是预拆分优化前的参考实现。
func refValidateSubTopicForDevice(topic, deviceNumber string) bool {
	if deviceNumber == "" {
		return false
	}
	for _, pattern := range subList {
		if !strings.Contains(pattern, deviceNumberSlot) || !matchesPatternSub(topic, pattern) {
			continue
		}
		tp, pp := strings.Split(topic, "/"), strings.Split(pattern, "/")
		if deviceSlotsBound(tp, pp, deviceNumber) {
			return true
		}
	}
	return false
}

func refIsStandardSubTopicCandidate(topic string) bool {
	for _, pattern := range subList {
		if matchesSubStructureParts(strings.Split(topic, "/"), strings.Split(pattern, "/")) {
			return true
		}
	}
	return false
}

func TestSubACLMatchesReference(t *testing.T) {
	topics := []string{
		"", "/", "SN1/down", "/down", "+/down", "#", "SN2/down",
		"devices/telemetry/control/SN1", "devices/telemetry/control/SN1/x", "devices/telemetry/control/+",
		"devices/command/SN1/+", "devices/command/SN1/#", "devices/register/response/abc",
		"gateway/event/response/SN1/m1", "a/b/c/d/e/f/g/h/i/j", "devices/telemetry/control/SN1/x/y/z/w/v",
	}
	for _, topic := range topics {
		if got, want := ValidateSubTopicForDevice(topic, "SN1"), refValidateSubTopicForDevice(topic, "SN1"); got != want {
			t.Errorf("ValidateSubTopicForDevice(%q)=%v want %v", topic, got, want)
		}
		if got, want := IsStandardSubTopicCandidate(topic), refIsStandardSubTopicCandidate(topic); got != want {
			t.Errorf("IsStandardSubTopicCandidate(%q)=%v want %v", topic, got, want)
		}
	}
}

func TestSplitTopicLevelsOverflowRejects(t *testing.T) {
	deep := strings.Repeat("a/", maxPatternLevels) + "up"
	var buf topicLevels
	if _, ok := splitTopicLevels(deep, &buf); ok {
		t.Fatal("超过 maxPatternLevels 层的主题应返回 ok=false")
	}
	if ValidateTopic(deep) || ValidatePubTopicForDevice(deep, "d", "a") {
		t.Fatal("超深主题必须被拒绝")
	}
	parts, ok := splitTopicLevels("a//b/", &buf)
	if !ok || strings.Join(parts, "|") != strings.Join(strings.Split("a//b/", "/"), "|") {
		t.Fatalf("分层语义应与 strings.Split 一致，got %q", parts)
	}
}
