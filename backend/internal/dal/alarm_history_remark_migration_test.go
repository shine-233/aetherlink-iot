package dal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/global"
)

func TestAlarmHistoryRemarkMigrationAllowsCumulativeAuditJSON(t *testing.T) {
	if global.VERSION_NUMBER < 43 {
		t.Fatalf("VERSION_NUMBER = %d, want at least 43", global.VERSION_NUMBER)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve alarm history migration test source path")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "sql", "43.sql"))
	if err != nil {
		t.Fatalf("read 43.sql: %v", err)
	}
	sql := strings.Join(strings.Fields(strings.ToLower(string(raw))), " ")
	for _, contract := range []string{
		"drop view if exists public.latest_device_alarms",
		"drop view if exists public.current_device_alarm_streams",
		"alter table public.alarm_history alter column remark type text",
		"create view public.current_device_alarm_streams as",
		"create view public.latest_device_alarms as",
	} {
		if !strings.Contains(sql, contract) {
			t.Fatalf("43.sql is missing migration contract %q: %s", contract, sql)
		}
	}

	acknowledged := actionAlarmHistoryAcknowledgeRemark(nil, "tenant-user-with-a-long-audit-identifier", "2026-07-29T15:01:47Z", "batch acknowledge seeded alarm closure")
	reset := actionAlarmHistoryResetRemark(&acknowledged, "tenant-user-with-a-long-audit-identifier", "2026-07-29T15:01:48Z", "H", "batch reset seeded alarm closure")
	if len(reset) <= 255 {
		t.Fatalf("combined audit JSON length = %d, test fixture must exceed the legacy varchar(255) limit", len(reset))
	}
}
