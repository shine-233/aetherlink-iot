package service

import (
	"os"
	"strings"
	"testing"
)

func TestDeleteAlarmConfigPreservesExistingAlarmHistory(t *testing.T) {
	// 9-28 域拆分后 DeleteAlarmConfig 与 UpdateAlarmConfig 迁至 alarm_config.go，
	// 契约断言跟随实现文件，标记对（Delete → Update 边界）保持不变。
	source, err := os.ReadFile("alarm_config.go")
	if err != nil {
		t.Fatalf("read alarm_config.go: %v", err)
	}

	text := string(source)
	startMarker := "func (*Alarm) DeleteAlarmConfig("
	start := strings.Index(text, startMarker)
	if start < 0 {
		t.Fatalf("DeleteAlarmConfig function not found")
	}

	endOffset := strings.Index(text[start:], "\nfunc (*Alarm) UpdateAlarmConfig(")
	if endOffset < 0 {
		t.Fatalf("UpdateAlarmConfig boundary not found")
	}
	deleteConfigSource := text[start : start+endOffset]

	if !strings.Contains(deleteConfigSource, "dal.DeleteAlarmConfig(id)") {
		t.Fatalf("DeleteAlarmConfig no longer deletes the alarm rule")
	}
	for _, forbiddenCall := range []string{
		"dal.DeleteAlarmHistory(id",
		"dal.DeleteAlarmHistoryByConfigId(id",
	} {
		if strings.Contains(deleteConfigSource, forbiddenCall) {
			t.Fatalf("DeleteAlarmConfig must preserve alarm history; found %q", forbiddenCall)
		}
	}
}
