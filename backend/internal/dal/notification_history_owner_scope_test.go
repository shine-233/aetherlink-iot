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

func TestNotificationHistoryOwnerScopeRequiresEveryLinkedDevice(t *testing.T) {
	db := setupNotificationHistoryDALTestDB(t)
	createNotificationHistoryTestDevice(t, db, "device-a", "tenant-1", "owner-a")
	createNotificationHistoryTestDevice(t, db, "device-b", "tenant-1", "owner-b")
	createNotificationHistoryTestDevice(t, db, "device-c", "tenant-2", "owner-a")

	createNotificationHistoryTestRow(t, "history-own", "tenant-1", "device-a")
	createNotificationHistoryTestRow(t, "history-foreign", "tenant-1", "device-b")
	createNotificationHistoryTestRow(t, "history-mixed", "tenant-1", "device-a", "device-b")
	createNotificationHistoryTestRow(t, "history-unscoped", "tenant-1")
	createNotificationHistoryTestRow(t, "history-other-tenant", "tenant-2", "device-c")
	createNotificationHistoryTestRow(t, "history-corrupt-scope", "tenant-1", "device-a")
	if err := db.Create(&model.NotificationHistoryDevice{
		NotificationHistoryID: "history-corrupt-scope",
		DeviceID:              "device-c",
		TenantID:              "tenant-2",
	}).Error; err != nil {
		t.Fatalf("create malformed cross-tenant link: %v", err)
	}

	req := &model.GetNotificationHistoryListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20},
	}
	ownerA := "owner-a"
	total, list, err := GetNotificationHisoryListByPage(req, []string{"tenant-1"}, &ownerA)
	if err != nil {
		t.Fatalf("query owner-scoped notification history: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "history-own" {
		t.Fatalf("owner-scoped histories = total %d, list %#v; want only history-own", total, notificationHistoryIDs(list))
	}
	if err := db.Delete(&model.Device{}, "id = ?", "device-b").Error; err != nil {
		t.Fatalf("delete foreign device: %v", err)
	}
	total, list, err = GetNotificationHisoryListByPage(req, []string{"tenant-1"}, &ownerA)
	if err != nil {
		t.Fatalf("query owner scope after foreign device deletion: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "history-own" {
		t.Fatalf("post-delete histories = total %d, list %#v; deleted foreign scope must remain hidden", total, notificationHistoryIDs(list))
	}

	total, list, err = GetNotificationHisoryListByPage(req, []string{"tenant-1"}, nil)
	if err != nil {
		t.Fatalf("query tenant-admin notification history: %v", err)
	}
	if total != 5 || len(list) != 5 {
		t.Fatalf("admin histories = total %d, list %#v; want all five tenant histories", total, notificationHistoryIDs(list))
	}
}

func TestCreateNotificationHistoryRejectsCrossTenantDeviceScopeAtomically(t *testing.T) {
	db := setupNotificationHistoryDALTestDB(t)
	createNotificationHistoryTestDevice(t, db, "device-other-tenant", "tenant-2", "owner-a")

	history := notificationHistoryTestRow("history-invalid-scope", "tenant-1")
	if err := CreateNotificationHistory(history, "device-other-tenant"); err == nil {
		t.Fatal("CreateNotificationHistory() error = nil, want cross-tenant scope rejection")
	}

	var historyCount int64
	if err := db.Model(&model.NotificationHistory{}).
		Where("id = ?", history.ID).
		Count(&historyCount).Error; err != nil {
		t.Fatalf("count rolled-back history: %v", err)
	}
	if historyCount != 0 {
		t.Fatalf("history count = %d, want transaction rollback", historyCount)
	}
}

func setupNotificationHistoryDALTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open notification history sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.NotificationHistory{},
		&model.NotificationHistoryDevice{},
		&model.Device{},
	); err != nil {
		t.Fatalf("migrate notification history test tables: %v", err)
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

func createNotificationHistoryTestDevice(t *testing.T, db *gorm.DB, id, tenantID, ownerUserID string) {
	t.Helper()
	device := &model.Device{
		ID:           id,
		Voucher:      "voucher-" + id,
		TenantID:     tenantID,
		IsEnabled:    "enabled",
		OwnerUserID:  &ownerUserID,
		ActivateFlag: "active",
		DeviceNumber: "number-" + id,
		IsOnline:     1,
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("create device %s: %v", id, err)
	}
}

func createNotificationHistoryTestRow(t *testing.T, id, tenantID string, deviceIDs ...string) {
	t.Helper()
	if err := CreateNotificationHistory(notificationHistoryTestRow(id, tenantID), deviceIDs...); err != nil {
		t.Fatalf("create notification history %s: %v", id, err)
	}
}

func notificationHistoryTestRow(id, tenantID string) *model.NotificationHistory {
	status := "SUCCESS"
	content := "content for " + id
	return &model.NotificationHistory{
		ID:               id,
		SendTime:         time.Now().UTC(),
		SendContent:      &content,
		SendTarget:       "ops@example.com",
		SendResult:       &status,
		NotificationType: model.NoticeType_Email,
		TenantID:         tenantID,
	}
}

func notificationHistoryIDs(list []*model.NotificationHistory) []string {
	ids := make([]string, 0, len(list))
	for _, history := range list {
		ids = append(ids, history.ID)
	}
	return ids
}
