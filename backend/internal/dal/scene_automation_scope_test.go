// 文件用途：验证场景自动化列表/告警列表/执行日志读路径的 tenant scopes 三态契约
// （ROADMAP C2 自上而下）：0→fail-closed 空结果、1→tenant_id =（旧单租户等价）、
// >1→tenant_id IN（self∪子孙）；含空租户 [""] 平台场景与设备锚点收敛用例。
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

func TestGetSceneAutomationByPageScopesScopeDown(t *testing.T) {
	db := setupSceneAutomationScopeTestDB(t)
	now := time.Now().UTC()
	scenes := []model.SceneAutomation{
		{ID: "scene-hq-1", Name: "hq one", Enabled: "Y", TenantID: "tenant-hq", Creator: "user", Updator: "user", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "scene-hq-2", Name: "hq two", Enabled: "N", TenantID: "tenant-hq", Creator: "user", Updator: "user", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "scene-child-1", Name: "child one", Enabled: "Y", TenantID: "tenant-child", Creator: "user", Updator: "user", CreatedAt: now.Add(-time.Minute)},
		{ID: "scene-foreign", Name: "foreign", Enabled: "Y", TenantID: "tenant-x", Creator: "user", Updator: "user", CreatedAt: now},
		{ID: "scene-platform", Name: "platform", Enabled: "Y", TenantID: "", Creator: "user", Updator: "user", CreatedAt: now.Add(time.Minute)},
	}
	if err := db.Create(&scenes).Error; err != nil {
		t.Fatalf("create scenes: %v", err)
	}

	t.Run("parent scope returns self and descendants only", func(t *testing.T) {
		count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{"tenant-hq", "tenant-child"})
		if err != nil {
			t.Fatalf("GetSceneAutomationByPage(): %v", err)
		}
		if count != 3 || len(list) != 3 {
			t.Fatalf("count = %d, list = %#v, want 3 in-scope rows", count, list)
		}
		for _, scene := range list {
			if scene.TenantID != "tenant-hq" && scene.TenantID != "tenant-child" {
				t.Fatalf("row %q escaped scope with tenant %q", scene.ID, scene.TenantID)
			}
		}
	})

	t.Run("single scope keeps legacy tenant filter", func(t *testing.T) {
		count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{"tenant-child"})
		if err != nil {
			t.Fatalf("GetSceneAutomationByPage(): %v", err)
		}
		if count != 1 || len(list) != 1 || list[0].ID != "scene-child-1" {
			t.Fatalf("count = %d, list = %#v, want only scene-child-1", count, list)
		}
	})

	t.Run("empty tenant scope maps platform rows", func(t *testing.T) {
		count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{""})
		if err != nil {
			t.Fatalf("GetSceneAutomationByPage(): %v", err)
		}
		if count != 1 || len(list) != 1 || list[0].ID != "scene-platform" {
			t.Fatalf("count = %d, list = %#v, want only scene-platform", count, list)
		}
	})

	t.Run("nil and empty scopes fail closed", func(t *testing.T) {
		for _, scopes := range [][]string{nil, []string{}} {
			count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, scopes)
			if err != nil {
				t.Fatalf("GetSceneAutomationByPage(scopes=%v): %v", scopes, err)
			}
			if count != 0 || list != nil {
				t.Fatalf("fail-closed result = (%d, %#v, %v), want (0, nil, nil)", count, list, err)
			}
		}
	})
}

func TestGetSceneAutomationByPageDeviceAnchorStaysWithinScopes(t *testing.T) {
	db := setupSceneAutomationScopeTestDB(t)
	deviceID := "scene-anchor-dev"
	configID := "scene-anchor-config"
	if err := db.Create(&model.Device{
		ID: deviceID, Voucher: "scene-voucher", TenantID: "tenant-hq", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "scene-anchor-number", DeviceConfigID: &configID,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	now := time.Now().UTC()
	if err := db.Create(&[]model.SceneAutomation{
		{ID: "scene-dev-trigger", Name: "dev trigger", Enabled: "Y", TenantID: "tenant-hq", Creator: "user", Updator: "user", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "scene-dev-action", Name: "dev action", Enabled: "Y", TenantID: "tenant-hq", Creator: "user", Updator: "user", CreatedAt: now.Add(-time.Minute)},
		{ID: "scene-child-plain", Name: "child plain", Enabled: "Y", TenantID: "tenant-child", Creator: "user", Updator: "user", CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create scenes: %v", err)
	}
	if err := db.Create(&[]model.DeviceTriggerCondition{
		{ID: "cond-dev", SceneAutomationID: "scene-dev-trigger", Enabled: "Y", GroupID: "g1", TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, TriggerSource: &deviceID, TriggerValue: "1", TenantID: "tenant-hq"},
		{ID: "cond-config", SceneAutomationID: "scene-dev-action", Enabled: "Y", GroupID: "g2", TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE, TriggerSource: &configID, TriggerValue: "1", TenantID: "tenant-hq"},
	}).Error; err != nil {
		t.Fatalf("create trigger conditions: %v", err)
	}

	t.Run("device anchored within expanded scope", func(t *testing.T) {
		count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{DeviceId: &deviceID, PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{"tenant-hq", "tenant-child"})
		if err != nil {
			t.Fatalf("GetSceneAutomationByPage(): %v", err)
		}
		if count != 2 || len(list) != 2 {
			t.Fatalf("count = %d, list = %#v, want 2 hq device/config matches", count, list)
		}
		for _, scene := range list {
			if scene.ID == "scene-child-plain" || scene.TenantID != "tenant-hq" {
				t.Fatalf("row %q (tenant %q) should not surface for hq anchor", scene.ID, scene.TenantID)
			}
		}
	})

	t.Run("single scope keeps legacy device anchored behavior", func(t *testing.T) {
		count, list, err := GetSceneAutomationByPage(&model.GetSceneAutomationByPageReq{DeviceId: &deviceID, PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{"tenant-hq"})
		if err != nil {
			t.Fatalf("GetSceneAutomationByPage(): %v", err)
		}
		if count != 2 || len(list) != 2 {
			t.Fatalf("count = %d, list = %#v, want 2 legacy matches", count, list)
		}
	})
}

func TestGetSceneAutomationWithAlarmByPageReqScopes(t *testing.T) {
	db := setupSceneAutomationScopeTestDB(t)
	deviceID := "scene-alarm-dev"
	if err := db.Create(&model.Device{
		ID: deviceID, Voucher: "scene-voucher", TenantID: "tenant-hq", IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "scene-alarm-number",
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	now := time.Now().UTC()
	if err := db.Create(&[]model.SceneAutomation{
		{ID: "scene-alarm-hit", Name: "alarm hit", Enabled: "Y", TenantID: "tenant-hq", Creator: "user", Updator: "user", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "scene-alarm-plain", Name: "alarm plain", Enabled: "Y", TenantID: "tenant-hq", Creator: "user", Updator: "user", CreatedAt: now.Add(-time.Minute)},
		{ID: "scene-alarm-child", Name: "alarm child", Enabled: "Y", TenantID: "tenant-child", Creator: "user", Updator: "user", CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create scenes: %v", err)
	}
	alarmTarget := "alarm-config-1"
	if err := db.Create(&[]model.DeviceTriggerCondition{
		{ID: "alarm-cond", SceneAutomationID: "scene-alarm-hit", Enabled: "Y", GroupID: "g1", TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE, TriggerSource: &deviceID, TriggerValue: "1", TenantID: "tenant-hq"},
	}).Error; err != nil {
		t.Fatalf("create trigger conditions: %v", err)
	}
	// 仅 scene-alarm-hit 携带告警动作；scene-alarm-plain 与 device 无任何关联，不应进入候选集。
	if err := db.Create(&[]model.ActionInfo{
		{ID: "alarm-action", SceneAutomationID: "scene-alarm-hit", ActionType: model.AUTOMATE_ACTION_TYPE_ALARM, ActionTarget: &alarmTarget},
	}).Error; err != nil {
		t.Fatalf("create alarm action: %v", err)
	}

	t.Run("alarm anchored within single legacy scope", func(t *testing.T) {
		count, list, err := GetSceneAutomationWithAlarmByPageReq(&model.GetSceneAutomationsWithAlarmByPageReq{DeviceId: &deviceID, PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{"tenant-hq"})
		if err != nil {
			t.Fatalf("GetSceneAutomationWithAlarmByPageReq(): %v", err)
		}
		if count != 1 || len(list) != 1 || list[0].ID != "scene-alarm-hit" {
			t.Fatalf("count = %d, list = %#v, want only scene-alarm-hit", count, list)
		}
	})

	t.Run("alarm anchored within expanded scope keeps hq result", func(t *testing.T) {
		count, list, err := GetSceneAutomationWithAlarmByPageReq(&model.GetSceneAutomationsWithAlarmByPageReq{DeviceId: &deviceID, PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{"tenant-hq", "tenant-child"})
		if err != nil {
			t.Fatalf("GetSceneAutomationWithAlarmByPageReq(): %v", err)
		}
		if count != 1 || len(list) != 1 || list[0].ID != "scene-alarm-hit" {
			t.Fatalf("count = %d, list = %#v, want only scene-alarm-hit", count, list)
		}
	})

	t.Run("empty scopes fail closed", func(t *testing.T) {
		count, list, err := GetSceneAutomationWithAlarmByPageReq(&model.GetSceneAutomationsWithAlarmByPageReq{DeviceId: &deviceID, PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{})
		if err != nil {
			t.Fatalf("GetSceneAutomationWithAlarmByPageReq(): %v", err)
		}
		if count != 0 || list != nil {
			t.Fatalf("fail-closed result = (%d, %#v, %v), want (0, nil, nil)", count, list, err)
		}
	})

	t.Run("missing both anchors returns no match instead of panic", func(t *testing.T) {
		count, list, err := GetSceneAutomationWithAlarmByPageReq(&model.GetSceneAutomationsWithAlarmByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, []string{"tenant-hq"})
		if err != nil {
			t.Fatalf("GetSceneAutomationWithAlarmByPageReq(): %v", err)
		}
		if count != 0 || list != nil {
			t.Fatalf("no-anchor result = (%d, %#v, %v), want (0, nil, nil)", count, list, err)
		}
	})
}

func TestGetSceneAutomationLogScopes(t *testing.T) {
	db := setupSceneAutomationScopeTestDB(t)
	now := time.Now().UTC()
	logs := []model.SceneAutomationLog{
		{SceneAutomationID: "scene-hq", TenantID: "tenant-hq", ExecutedAt: now.Add(-2 * time.Minute), ExecutionResult: "S", Detail: "hq ok"},
		{SceneAutomationID: "scene-hq", TenantID: "tenant-hq", ExecutedAt: now.Add(-time.Minute), ExecutionResult: "F", Detail: "hq fail"},
		{SceneAutomationID: "scene-child", TenantID: "tenant-child", ExecutedAt: now.Add(-time.Minute), ExecutionResult: "S", Detail: "child ok"},
		{SceneAutomationID: "scene-platform", TenantID: "", ExecutedAt: now.Add(-time.Minute), ExecutionResult: "S", Detail: "platform ok"},
		{SceneAutomationID: "scene-foreign", TenantID: "tenant-x", ExecutedAt: now.Add(-time.Minute), ExecutionResult: "S", Detail: "foreign ok"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	t.Run("parent scope reads descendant scene logs", func(t *testing.T) {
		req := &model.GetSceneAutomationLogReq{SceneAutomationId: "scene-child", PageReq: model.PageReq{Page: 1, PageSize: 20}}
		count, list, err := GetSceneAutomationLog(req, []string{"tenant-hq", "tenant-child"})
		if err != nil {
			t.Fatalf("GetSceneAutomationLog(): %v", err)
		}
		if count != 1 || len(list) != 1 || list[0].Detail != "child ok" {
			t.Fatalf("count = %d, list = %#v, want only child log", count, list)
		}
	})

	t.Run("single scope keeps legacy strict filter", func(t *testing.T) {
		req := &model.GetSceneAutomationLogReq{SceneAutomationId: "scene-hq", PageReq: model.PageReq{Page: 1, PageSize: 20}}
		count, list, err := GetSceneAutomationLog(req, []string{"tenant-hq"})
		if err != nil {
			t.Fatalf("GetSceneAutomationLog(): %v", err)
		}
		if count != 2 || len(list) != 2 {
			t.Fatalf("count = %d, list = %#v, want 2 hq logs", count, list)
		}
	})

	t.Run("result and time filters combine with scopes", func(t *testing.T) {
		result := "F"
		start := now.Add(-90 * time.Second)
		end := now
		req := &model.GetSceneAutomationLogReq{SceneAutomationId: "scene-hq", ExecutionResult: &result, ExecutionStartTime: &start, ExecutionEndTime: &end, PageReq: model.PageReq{Page: 1, PageSize: 20}}
		count, list, err := GetSceneAutomationLog(req, []string{"tenant-hq", "tenant-child"})
		if err != nil {
			t.Fatalf("GetSceneAutomationLog(): %v", err)
		}
		if count != 1 || len(list) != 1 || list[0].ExecutionResult != "F" {
			t.Fatalf("count = %d, list = %#v, want single F hq log", count, list)
		}
	})

	t.Run("platform empty tenant scope reads platform logs", func(t *testing.T) {
		req := &model.GetSceneAutomationLogReq{SceneAutomationId: "scene-platform", PageReq: model.PageReq{Page: 1, PageSize: 20}}
		count, list, err := GetSceneAutomationLog(req, []string{""})
		if err != nil {
			t.Fatalf("GetSceneAutomationLog(): %v", err)
		}
		if count != 1 || len(list) != 1 || list[0].Detail != "platform ok" {
			t.Fatalf("count = %d, list = %#v, want only platform log", count, list)
		}
	})

	t.Run("empty scopes fail closed", func(t *testing.T) {
		for _, scopes := range [][]string{nil, []string{}} {
			req := &model.GetSceneAutomationLogReq{SceneAutomationId: "scene-hq", PageReq: model.PageReq{Page: 1, PageSize: 20}}
			count, list, err := GetSceneAutomationLog(req, scopes)
			if err != nil {
				t.Fatalf("GetSceneAutomationLog(scopes=%v): %v", scopes, err)
			}
			if count != 0 || len(list) != 0 {
				t.Fatalf("fail-closed result = (%d, %#v, %v), want (0, [], nil)", count, list, err)
			}
		}
	})

	t.Run("out of scope scene fails closed", func(t *testing.T) {
		req := &model.GetSceneAutomationLogReq{SceneAutomationId: "scene-foreign", PageReq: model.PageReq{Page: 1, PageSize: 20}}
		count, list, err := GetSceneAutomationLog(req, []string{"tenant-hq", "tenant-child"})
		if err != nil {
			t.Fatalf("GetSceneAutomationLog(): %v", err)
		}
		if count != 0 || len(list) != 0 {
			t.Fatalf("out-of-scope result = (%d, %#v, %v), want (0, [], nil)", count, list, err)
		}
	})
}

func setupSceneAutomationScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open scene automation scope sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SceneAutomation{}, &model.Device{}, &model.DeviceTriggerCondition{}, &model.ActionInfo{}, &model.SceneAutomationLog{}); err != nil {
		t.Fatalf("migrate scene automation scope tables: %v", err)
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
