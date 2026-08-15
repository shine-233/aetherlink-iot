package service

import (
	"os"
	"strings"
	"testing"
)

func TestDeleteAlarmConfigPreservesExistingAlarmHistory(t *testing.T) {
	source, err := os.ReadFile("alarm.go")
	if err != nil {
		t.Fatalf("read alarm.go: %v", err)
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
