// 文件用途: 覆盖 device_shadow.go 影子消息 CRUD 的回归测试（ROADMAP A3）。
// 核心逻辑: 构造不同状态/到期时间的影子行，断言待投递查询、状态流转与清理语义。
// 关键注意事项: 时间比较必须参数化（sqlite 兼容）；pending 查询只含未过期行。
package dal

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupShadowDALTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := "shadow_dal_" + t.Name()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(&model.DeviceShadowMessage{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func createShadowRow(t *testing.T, db *gorm.DB, id, deviceID, status string, expiresAt time.Time) {
	t.Helper()
	payload := `{"method":"set","params":{"power":1}}`
	msg := &model.DeviceShadowMessage{
		ID:          id,
		DeviceID:    deviceID,
		MessageType: "command",
		Payload:     &payload,
		TTLSeconds:  3600,
		Status:      status,
		CreatedAt:   ptrTime(time.Now().UTC().Add(-time.Minute)),
		ExpiresAt:   expiresAt,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("create shadow message %s: %v", id, err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestShadowPendingLifecycle(t *testing.T) {
	db := setupShadowDALTestDB(t)
	now := time.Now().UTC()
	createShadowRow(t, db, "sm-1", "dev-1", "pending", now.Add(time.Hour))

	pending, err := GetPendingShadowMessages("dev-1")
	if err != nil {
		t.Fatalf("GetPendingShadowMessages: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "sm-1" {
		t.Fatalf("pending = %#v, want [sm-1]", pending)
	}

	if err := MarkShadowMessageDelivered("sm-1"); err != nil {
		t.Fatalf("MarkShadowMessageDelivered: %v", err)
	}
	pending, _ = GetPendingShadowMessages("dev-1")
	if len(pending) != 0 {
		t.Fatalf("pending after delivery = %#v, want empty", pending)
	}

	all, err := GetAllShadowMessages("dev-1", "")
	if err != nil {
		t.Fatalf("GetAllShadowMessages: %v", err)
	}
	if len(all) != 1 || all[0].Status != "delivered" || all[0].DeliveredAt == nil {
		t.Fatalf("all = %#v, want single delivered row with delivered_at", all)
	}
}

func TestShadowPendingExcludesExpiredRows(t *testing.T) {
	db := setupShadowDALTestDB(t)
	now := time.Now().UTC()
	createShadowRow(t, db, "sm-live", "dev-1", "pending", now.Add(time.Hour))
	createShadowRow(t, db, "sm-dead", "dev-1", "pending", now.Add(-time.Minute))

	pending, err := GetPendingShadowMessages("dev-1")
	if err != nil {
		t.Fatalf("GetPendingShadowMessages: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "sm-live" {
		t.Fatalf("pending = %#v, want only sm-live (expired row must be excluded)", pending)
	}

	affected, err := ExpireDueShadowMessages()
	if err != nil {
		t.Fatalf("ExpireDueShadowMessages: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expired rows = %d, want 1", affected)
	}
	counts, err := CountShadowMessagesByDevice("dev-1")
	if err != nil {
		t.Fatalf("CountShadowMessagesByDevice: %v", err)
	}
	if counts["expired"] != 1 || counts["pending"] != 1 {
		t.Fatalf("counts = %#v, want expired=1 pending=1", counts)
	}
}

func TestShadowCancelAndStaleCleanup(t *testing.T) {
	db := setupShadowDALTestDB(t)
	now := time.Now().UTC()
	createShadowRow(t, db, "sm-cancel", "dev-1", "pending", now.Add(time.Hour))
	createShadowRow(t, db, "sm-stale", "dev-2", "canceled", now.Add(-8*24*time.Hour))
	createShadowRow(t, db, "sm-fresh", "dev-2", "delivered", now.Add(-time.Minute))

	if err := CancelShadowMessage("sm-cancel"); err != nil {
		t.Fatalf("CancelShadowMessage: %v", err)
	}
	if err := CancelShadowMessage("sm-cancel"); err == nil {
		t.Fatal("second cancel must fail with not found")
	}

	statusFilter, err := GetAllShadowMessages("dev-1", "canceled")
	if err != nil {
		t.Fatalf("GetAllShadowMessages filtered: %v", err)
	}
	if len(statusFilter) != 1 || statusFilter[0].ID != "sm-cancel" {
		t.Fatalf("filtered = %#v, want [sm-cancel]", statusFilter)
	}

	deleted, err := DeleteStaleShadowMessages()
	if err != nil {
		t.Fatalf("DeleteStaleShadowMessages: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (only stale canceled row)", deleted)
	}
	var remaining int64
	if err := db.Model(&model.DeviceShadowMessage{}).Where("id = ?", "sm-stale").Count(&remaining).Error; err != nil {
		t.Fatalf("count remaining: %v", err)
	}
	if remaining != 0 {
		t.Fatal("stale row must be physically deleted")
	}
}
