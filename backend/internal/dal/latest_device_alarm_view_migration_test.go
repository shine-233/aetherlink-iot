package dal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/global"
)

func TestLatestDeviceAlarmViewMigrationPrefersAnyActiveSeverity(t *testing.T) {
	if global.VERSION_NUMBER < 35 {
		t.Fatalf("VERSION_NUMBER = %d, want at least 35", global.VERSION_NUMBER)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration test source path")
	}
	migrationPath := filepath.Join(filepath.Dir(filename), "..", "..", "sql", "35.sql")
	raw, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read 35.sql: %v", err)
	}
	sql := string(raw)

	for _, contract := range []string{
		"CREATE OR REPLACE VIEW public.current_device_alarm_streams",
		"CREATE OR REPLACE VIEW public.latest_device_alarms",
		"PARTITION BY tenant_id, device_id, alarm_config_id, group_id, scene_automation_id",
		"WHERE stream_rn = 1",
		"FROM public.current_device_alarm_streams",
		"PARTITION BY tenant_id, device_id",
		"CASE WHEN alarm_status IN ('H', 'M', 'L') THEN 0 ELSE 1 END",
		"create_at DESC NULLS LAST",
		"id DESC",
	} {
		if !strings.Contains(sql, contract) {
			t.Fatalf("35.sql is missing active-first view contract %q", contract)
		}
	}
}
