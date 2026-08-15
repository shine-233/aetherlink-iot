package dal

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestResetLoadedAlarmHistoryRejectsAStaleConcurrentReset(t *testing.T) {
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open alarm action sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AlarmHistory{}); err != nil {
		t.Fatalf("migrate alarm history: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})

	history := model.AlarmHistory{
		ID:                "alarm-1",
		AlarmConfigID:     "config-1",
		GroupID:           "group-1",
		SceneAutomationID: "scene-1",
		Name:              "Alarm",
		AlarmStatus:       "H",
		TenantID:          "tenant-1",
		CreateAt:          time.Now().UTC(),
		AlarmDeviceList:   `[]`,
	}
	if err := db.Create(&history).Error; err != nil {
		t.Fatalf("seed alarm history: %v", err)
	}
	firstLoaded := history
	staleLoaded := history

	if _, err := ResetLoadedAlarmHistoryWithNote(&firstLoaded, "user-1", "first reset"); err != nil {
		t.Fatalf("first reset: %v", err)
	}
	if _, err := ResetLoadedAlarmHistoryWithNote(&staleLoaded, "user-2", "stale reset"); err == nil {
		t.Fatal("expected stale concurrent reset to fail")
	}

	var persisted model.AlarmHistory
	if err := db.First(&persisted, "id = ?", history.ID).Error; err != nil {
		t.Fatalf("reload reset history: %v", err)
	}
	if persisted.AlarmStatus != "N" || persisted.Remark == nil {
		t.Fatalf("persisted reset = status %q remark %#v", persisted.AlarmStatus, persisted.Remark)
	}
	remark := map[string]interface{}{}
	if err := json.Unmarshal([]byte(*persisted.Remark), &remark); err != nil {
		t.Fatalf("decode reset remark: %v", err)
	}
	if remark["reset_by"] != "user-1" || remark["reset_note"] != "first reset" {
		t.Fatalf("stale reset overwrote first reset: %#v", remark)
	}
}
