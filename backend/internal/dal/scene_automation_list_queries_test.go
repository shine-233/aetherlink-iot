// 文件用途：验证场景自动化列表拆分后的设备/配置筛选、租户隔离和排序契约。
package dal

import (
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

func TestGetSceneAutomationByPageMergesDeviceAndConfigMatches(t *testing.T) {
	db := setupSceneAutomationListTestDB(t)
	deviceID := "scene-device"
	configID := "scene-config"
	if err := db.Create(&model.Device{
		ID: deviceID, Voucher: "scene-voucher", TenantID: "tenant-a", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "scene-device-number", DeviceConfigID: &configID,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	now := time.Now().UTC()
	scenes := []model.SceneAutomation{
		{ID: "scene-device-trigger", Name: "device trigger", Enabled: "Y", TenantID: "tenant-a", Creator: "user", Updator: "user", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "scene-device-action", Name: "device action", Enabled: "Y", TenantID: "tenant-a", Creator: "user", Updator: "user", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "scene-config-trigger", Name: "config trigger", Enabled: "Y", TenantID: "tenant-a", Creator: "user", Updator: "user", CreatedAt: now.Add(-time.Minute)},
		{ID: "scene-other-tenant", Name: "other tenant", Enabled: "Y", TenantID: "tenant-b", Creator: "user", Updator: "user", CreatedAt: now},
	}
	if err := db.Create(&scenes).Error; err != nil {
		t.Fatalf("create scenes: %v", err)
	}
	if err := db.Create(&[]model.DeviceTriggerCondition{
		{ID: "condition-device", SceneAutomationID: "scene-device-trigger", Enabled: "Y", GroupID: "group-1", TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, TriggerSource: &deviceID, TriggerValue: "1", TenantID: "tenant-a"},
		{ID: "condition-config", SceneAutomationID: "scene-config-trigger", Enabled: "Y", GroupID: "group-2", TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE, TriggerSource: &configID, TriggerValue: "1", TenantID: "tenant-a"},
		{ID: "condition-other", SceneAutomationID: "scene-other-tenant", Enabled: "Y", GroupID: "group-3", TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, TriggerSource: &deviceID, TriggerValue: "1", TenantID: "tenant-b"},
	}).Error; err != nil {
		t.Fatalf("create trigger conditions: %v", err)
	}
	if err := db.Create(&model.ActionInfo{ID: "action-device", SceneAutomationID: "scene-device-action", ActionType: model.AUTOMATE_ACTION_TYPE_ONE, ActionTarget: &deviceID}).Error; err != nil {
		t.Fatalf("create action: %v", err)
	}

	count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{
		DeviceId: &deviceID,
		PageReq:  model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-a")
	if err != nil {
		t.Fatalf("GetSceneAutomationByPage(): %v", err)
	}
	if count != 3 || len(list) != 3 {
		t.Fatalf("GetSceneAutomationByPage() count = %d, list = %#v, want 3", count, list)
	}
	wantOrder := []string{"scene-config-trigger", "scene-device-action", "scene-device-trigger"}
	for i, wantID := range wantOrder {
		if list[i].ID != wantID {
			t.Fatalf("list[%d].ID = %q, want %q", i, list[i].ID, wantID)
		}
	}
}

func TestGetSceneAutomationByPageKeepsNoMatchNilList(t *testing.T) {
	setupSceneAutomationListTestDB(t)
	missingDevice := "missing-device"
	count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{DeviceId: &missingDevice}, "tenant-a")
	if err != nil || count != 0 || list != nil {
		t.Fatalf("no-match result = (%d, %#v, %v), want (0, nil, nil)", count, list, err)
	}
}

func setupSceneAutomationListTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open scene automation sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SceneAutomation{}, &model.Device{}, &model.DeviceTriggerCondition{}, &model.ActionInfo{}); err != nil {
		t.Fatalf("migrate scene automation tables: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}
