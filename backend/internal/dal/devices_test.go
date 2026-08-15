// 文件用途: 覆盖 DAL 层手写查询、缓存或聚合逻辑的回归测试，验证数据访问边界不会漂移。
// 核心逻辑: 构造最小依赖场景并断言查询条件、缓存键、事务副作用或租户过滤结果。
// 关键注意事项: 测试应显式覆盖租户隔离、权限前置假设和事务失败路径，避免只验证成功路径。
// 重构建议: 随 DAL 查询拆分同步拆小测试夹具，并优先补齐跨租户、空依赖和半提交风险用例。

package dal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDeviceVoucherNotIncludedInNotFoundError(t *testing.T) {
	voucher := `{"username":"device","password":"secret"}`
	err := deviceVoucherNotFoundError(gorm.ErrRecordNotFound)

	if strings.Contains(err.Error(), voucher) {
		t.Fatalf("voucher leaked in error: %s", err)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected wrapped gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestGetDeviceCurrentStatusMissingDeviceIsOffline(t *testing.T) {
	setupDeviceDALTestDB(t)

	status, err := GetDeviceCurrentStatus("missing-device")
	if err != nil {
		t.Fatalf("expected missing devices to be treated as offline, got err %v", err)
	}
	if status != "OFF-LINE" {
		t.Fatalf("status = %q, want OFF-LINE", status)
	}
}

func TestGetDeviceCurrentStatusOnline(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()
	if err := db.Create(&model.Device{
		ID:           "device-online",
		Voucher:      `{"username":"device-online"}`,
		TenantID:     "tenant-1",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
		DeviceNumber: "device-online",
		IsOnline:     1,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	status, err := GetDeviceCurrentStatus("device-online")
	if err != nil {
		t.Fatalf("GetDeviceCurrentStatus returned error: %v", err)
	}
	if status != "ON-LINE" {
		t.Fatalf("status = %q, want ON-LINE", status)
	}
}

func TestUpdateDeviceStatusReportsOnlyRealStatusChanges(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()
	deviceID := "device-status-change"
	if err := db.Create(&model.Device{
		ID:           deviceID,
		Voucher:      `{"username":"device-status-change"}`,
		TenantID:     "tenant-1",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		CreatedAt:    &now,
		UpdateAt:     &now,
		DeviceNumber: "device-status-change",
		IsOnline:     0,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	changed, err := UpdateDeviceStatus(deviceID, 1)
	if err != nil {
		t.Fatalf("first UpdateDeviceStatus returned error: %v", err)
	}
	if !changed {
		t.Fatalf("first UpdateDeviceStatus changed = false, want true")
	}

	changed, err = UpdateDeviceStatus(deviceID, 1)
	if err != nil {
		t.Fatalf("second UpdateDeviceStatus returned error: %v", err)
	}
	if changed {
		t.Fatalf("second UpdateDeviceStatus changed = true, want false for unchanged status")
	}

	var device model.Device
	if err := db.Where("id = ?", deviceID).First(&device).Error; err != nil {
		t.Fatalf("load updated device: %v", err)
	}
	if device.IsOnline != 1 {
		t.Fatalf("IsOnline = %d, want 1", device.IsOnline)
	}
}

func TestGetDeviceTemplateIdByDeviceId(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()
	templateID := "template-1"
	configID := "config-1"
	if err := db.Create(&model.DeviceConfig{
		ID:               configID,
		Name:             "config",
		DeviceTemplateID: &templateID,
		DeviceType:       "1",
		TenantID:         "tenant-1",
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("create device config: %v", err)
	}
	if err := db.Create(&model.Device{
		ID:             "device-with-template",
		Voucher:        `{"username":"device-with-template"}`,
		TenantID:       "tenant-1",
		IsEnabled:      "enabled",
		ActivateFlag:   "active",
		CreatedAt:      &now,
		UpdateAt:       &now,
		DeviceNumber:   "device-with-template",
		DeviceConfigID: &configID,
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	got, err := GetDeviceTemplateIdByDeviceId("device-with-template")
	if err != nil {
		t.Fatalf("GetDeviceTemplateIdByDeviceId returned error: %v", err)
	}
	if got != templateID {
		t.Fatalf("template id = %q, want %q", got, templateID)
	}
}

func TestGetDeviceListByPageSharedStatusMatchesServiceSemantics(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()

	blankRecipient := `{"rdi_share_recipients":[{"user_id":"   ","email":"blank@example.com"}]}`
	spacedShared := `{"rdi_share_recipients": [{"user_id":"user-1","email":"shared@example.com"}]}`
	plainUnshared := `{"note":"no share recipients"}`
	plainShared := `{"rdi_share_recipients":[{"user_id":"user-2"}]}`

	devices := []model.Device{
		{
			ID:             "device-blank-recipient",
			Voucher:        `{"username":"device-blank-recipient"}`,
			TenantID:       "tenant-1",
			IsEnabled:      "enabled",
			ActivateFlag:   "active",
			CreatedAt:      &now,
			UpdateAt:       &now,
			DeviceNumber:   "device-blank-recipient",
			AdditionalInfo: &blankRecipient,
		},
		{
			ID:             "device-spaced-shared",
			Voucher:        `{"username":"device-spaced-shared"}`,
			TenantID:       "tenant-1",
			IsEnabled:      "enabled",
			ActivateFlag:   "active",
			CreatedAt:      &now,
			UpdateAt:       &now,
			DeviceNumber:   "device-spaced-shared",
			AdditionalInfo: &spacedShared,
		},
		{
			ID:             "device-plain-unshared",
			Voucher:        `{"username":"device-plain-unshared"}`,
			TenantID:       "tenant-1",
			IsEnabled:      "enabled",
			ActivateFlag:   "active",
			CreatedAt:      &now,
			UpdateAt:       &now,
			DeviceNumber:   "device-plain-unshared",
			AdditionalInfo: &plainUnshared,
		},
		{
			ID:             "device-plain-shared",
			Voucher:        `{"username":"device-plain-shared"}`,
			TenantID:       "tenant-1",
			IsEnabled:      "enabled",
			ActivateFlag:   "active",
			CreatedAt:      &now,
			UpdateAt:       &now,
			DeviceNumber:   "device-plain-shared",
			AdditionalInfo: &plainShared,
		},
	}
	for _, device := range devices {
		device := device
		if err := db.Create(&device).Error; err != nil {
			t.Fatalf("create device %s: %v", device.ID, err)
		}
	}

	sharedStatus := "shared"
	sharedReq := &model.GetDeviceListByPageReq{
		SharedStatus: &sharedStatus,
		PageReq:      model.PageReq{Page: 1, PageSize: 20},
	}
	sharedCount, sharedList, err := GetDeviceListByPage(sharedReq, "tenant-1")
	if err != nil {
		t.Fatalf("GetDeviceListByPage shared returned error: %v", err)
	}
	if sharedCount != 2 {
		t.Fatalf("shared count = %d, want 2", sharedCount)
	}
	sharedIDs := map[string]bool{}
	for _, item := range sharedList {
		sharedIDs[item.ID] = true
	}
	if !sharedIDs["device-spaced-shared"] || !sharedIDs["device-plain-shared"] {
		t.Fatalf("shared ids = %#v, want spaced-shared and plain-shared", sharedIDs)
	}
	if sharedIDs["device-blank-recipient"] || sharedIDs["device-plain-unshared"] {
		t.Fatalf("shared ids unexpectedly include unshared devices: %#v", sharedIDs)
	}

	unsharedStatus := "unshared"
	unsharedReq := &model.GetDeviceListByPageReq{
		SharedStatus: &unsharedStatus,
		PageReq:      model.PageReq{Page: 1, PageSize: 20},
	}
	unsharedCount, unsharedList, err := GetDeviceListByPage(unsharedReq, "tenant-1")
	if err != nil {
		t.Fatalf("GetDeviceListByPage unshared returned error: %v", err)
	}
	if unsharedCount != 2 {
		t.Fatalf("unshared count = %d, want 2", unsharedCount)
	}
	unsharedIDs := map[string]bool{}
	for _, item := range unsharedList {
		unsharedIDs[item.ID] = true
	}
	if !unsharedIDs["device-blank-recipient"] || !unsharedIDs["device-plain-unshared"] {
		t.Fatalf("unshared ids = %#v, want blank-recipient and plain-unshared", unsharedIDs)
	}
	if unsharedIDs["device-spaced-shared"] || unsharedIDs["device-plain-shared"] {
		t.Fatalf("unshared ids unexpectedly include shared devices: %#v", unsharedIDs)
	}
}

func TestGetDeviceListByPageWarnStatusNormalKeepsTenantAndActiveFilters(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()

	devices := []model.Device{
		{
			ID:           "tenant-1-normal-alarm",
			Voucher:      `{"username":"tenant-1-normal-alarm"}`,
			TenantID:     "tenant-1",
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "tenant-1-normal-alarm",
		},
		{
			ID:           "tenant-1-no-alarm",
			Voucher:      `{"username":"tenant-1-no-alarm"}`,
			TenantID:     "tenant-1",
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "tenant-1-no-alarm",
		},
		{
			ID:           "tenant-1-active-alarm",
			Voucher:      `{"username":"tenant-1-active-alarm"}`,
			TenantID:     "tenant-1",
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "tenant-1-active-alarm",
		},
		{
			ID:           "tenant-2-no-alarm",
			Voucher:      `{"username":"tenant-2-no-alarm"}`,
			TenantID:     "tenant-2",
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "tenant-2-no-alarm",
		},
		{
			ID:           "tenant-1-inactive-no-alarm",
			Voucher:      `{"username":"tenant-1-inactive-no-alarm"}`,
			TenantID:     "tenant-1",
			IsEnabled:    "enabled",
			ActivateFlag: "inactive",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "tenant-1-inactive-no-alarm",
		},
	}
	for _, device := range devices {
		device := device
		if err := db.Create(&device).Error; err != nil {
			t.Fatalf("create device %s: %v", device.ID, err)
		}
	}

	normalStatus := "N"
	alarmID := "latest-normal-alarm"
	alarmDeviceID := "tenant-1-normal-alarm"
	alarmTenantID := "tenant-1"
	if err := db.Create(&model.LatestDeviceAlarm{
		ID:          &alarmID,
		DeviceID:    &alarmDeviceID,
		TenantID:    &alarmTenantID,
		AlarmStatus: &normalStatus,
	}).Error; err != nil {
		t.Fatalf("create latest device alarm: %v", err)
	}
	activeStatus := "H"
	activeAlarmID := "latest-active-alarm"
	activeAlarmDeviceID := "tenant-1-active-alarm"
	if err := db.Create(&model.LatestDeviceAlarm{
		ID:          &activeAlarmID,
		DeviceID:    &activeAlarmDeviceID,
		TenantID:    &alarmTenantID,
		AlarmStatus: &activeStatus,
	}).Error; err != nil {
		t.Fatalf("create active latest device alarm: %v", err)
	}
	inactiveAlarmID := "latest-inactive-device-alarm"
	inactiveAlarmDeviceID := "tenant-1-inactive-no-alarm"
	if err := db.Create(&model.LatestDeviceAlarm{
		ID:          &inactiveAlarmID,
		DeviceID:    &inactiveAlarmDeviceID,
		TenantID:    &alarmTenantID,
		AlarmStatus: &activeStatus,
	}).Error; err != nil {
		t.Fatalf("create inactive-device latest alarm: %v", err)
	}

	req := &model.GetDeviceListByPageReq{
		WarnStatus: &normalStatus,
		PageReq:    model.PageReq{Page: 1, PageSize: 20},
	}
	count, list, err := GetDeviceListByPage(req, "tenant-1")
	if err != nil {
		t.Fatalf("GetDeviceListByPage warn_status=N returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	got := map[string]bool{}
	for _, item := range list {
		got[item.ID] = true
	}
	if !got["tenant-1-normal-alarm"] || !got["tenant-1-no-alarm"] {
		t.Fatalf("ids = %#v, want tenant-1 normal and no-alarm devices", got)
	}
	if got["tenant-1-active-alarm"] || got["tenant-2-no-alarm"] || got["tenant-1-inactive-no-alarm"] {
		t.Fatalf("ids leaked active alarm, tenant, or inactive device: %#v", got)
	}

	alarmedStatus := "Y"
	count, list, err = GetDeviceListByPage(&model.GetDeviceListByPageReq{
		WarnStatus: &alarmedStatus,
		PageReq:    model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-1")
	if err != nil {
		t.Fatalf("GetDeviceListByPage warn_status=Y returned error: %v", err)
	}
	if count != 1 || len(list) != 1 || list[0].ID != "tenant-1-active-alarm" {
		t.Fatalf("warn_status=Y result = count %d list %#v, want only tenant-1-active-alarm", count, list)
	}

	unknownStatus := "ACTIVE"
	if _, _, err := GetDeviceListByPage(&model.GetDeviceListByPageReq{
		WarnStatus: &unknownStatus,
		PageReq:    model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-1"); err == nil {
		t.Fatal("expected unsupported warn_status to fail closed")
	}

	alarmDeviceCount, err := (&LatestDeviceAlarmQuery{}).CountDevicesByTenantAndStatus(context.Background(), "tenant-1", nil)
	if err != nil {
		t.Fatalf("count active alarm devices: %v", err)
	}
	if alarmDeviceCount != 1 {
		t.Fatalf("active alarm device count = %d, want only the active device", alarmDeviceCount)
	}
}

func TestGetDeviceListByPageFiltersLatestReportAndNeverReported(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	tenantID := "tenant-1"
	for _, id := range []string{"reported-recent", "reported-old", "never-reported"} {
		device := &model.Device{
			ID:           id,
			Voucher:      `{"username":"` + id + `"}`,
			TenantID:     tenantID,
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: id,
		}
		if err := db.Create(device).Error; err != nil {
			t.Fatalf("create device %s: %v", id, err)
		}
	}
	for _, current := range []model.TelemetryCurrentData{
		{DeviceID: "reported-recent", Key: "temperature", T: now.Add(-time.Hour), TenantID: &tenantID},
		{DeviceID: "reported-old", Key: "temperature", T: now.Add(-72 * time.Hour), TenantID: &tenantID},
	} {
		current := current
		if err := db.Create(&current).Error; err != nil {
			t.Fatalf("create telemetry current row: %v", err)
		}
	}

	neverReported := true
	count, list, err := GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq:       model.PageReq{Page: 1, PageSize: 20},
		NeverReported: &neverReported,
	}, tenantID)
	if err != nil {
		t.Fatalf("filter never reported: %v", err)
	}
	if count != 1 || len(list) != 1 || list[0].ID != "never-reported" || list[0].Ts != nil {
		t.Fatalf("never-reported result = count %d list %#v", count, list)
	}

	after := now.Add(-24 * time.Hour).UnixMilli()
	count, list, err = GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq:           model.PageReq{Page: 1, PageSize: 20},
		LastReportedAfter: &after,
	}, tenantID)
	if err != nil {
		t.Fatalf("filter last reported after: %v", err)
	}
	if count != 1 || len(list) != 1 || list[0].ID != "reported-recent" || list[0].Ts == nil {
		t.Fatalf("recently-reported result = count %d list %#v", count, list)
	}

	before := now.Add(-24 * time.Hour).UnixMilli()
	count, list, err = GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq:            model.PageReq{Page: 1, PageSize: 20},
		LastReportedBefore: &before,
	}, tenantID)
	if err != nil {
		t.Fatalf("filter last reported before: %v", err)
	}
	if count != 1 || len(list) != 1 || list[0].ID != "reported-old" || list[0].Ts == nil {
		t.Fatalf("stale-reported result = count %d list %#v", count, list)
	}

	if _, _, err := GetDeviceListByPage(&model.GetDeviceListByPageReq{
		NeverReported:     &neverReported,
		LastReportedAfter: &after,
	}, tenantID); err == nil {
		t.Fatal("expected conflicting never-reported and time range filter to fail")
	}
}

func setupDeviceDALTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Device{}, &model.DeviceConfig{}, &model.TelemetryCurrentData{}, &model.LatestDeviceAlarm{}, &model.DeviceStatusHistory{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
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
