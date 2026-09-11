package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupNotificationGroupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(&model.NotificationGroup{}); err != nil {
		t.Fatalf("migrate notification group: %v", err)
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

func notificationGroupTestClaims() *utils.UserClaims {
	return &utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}
}

func assertDirectAppParamError(t *testing.T, err error) {
	t.Helper()
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("error = %#v, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeParamError {
		t.Fatalf("error code = %d, want %d", appErr.Code, errcode.CodeParamError)
	}
	if appErr.CustomMsg != directAppNotificationTypeMessage {
		t.Fatalf("error message = %q, want %q", appErr.CustomMsg, directAppNotificationTypeMessage)
	}
}

func TestCreateNotificationGroupRejectsDirectAppWithoutPersisting(t *testing.T) {
	db := setupNotificationGroupServiceTestDB(t)
	service := &NotificationGroup{}
	for _, notificationType := range []string{"APP", "EMAIL,APP", "EMAIL, APP", "app"} {
		t.Run(notificationType, func(t *testing.T) {
			_, err := service.CreateNotificationGroup(&model.CreateNotificationGroupReq{
				Name: notificationType, NotificationType: notificationType, Status: "OPEN",
			}, notificationGroupTestClaims())
			assertDirectAppParamError(t, err)
		})
	}
	var count int64
	if err := db.Model(&model.NotificationGroup{}).Count(&count).Error; err != nil {
		t.Fatalf("count notification groups: %v", err)
	}
	if count != 0 {
		t.Fatalf("persisted notification groups = %d, want 0", count)
	}
}

func TestCreateNotificationGroupAllowsMemberWithNestedAppConfig(t *testing.T) {
	db := setupNotificationGroupServiceTestDB(t)
	config := `{"MEMBER":[{"name":"user-1","notificationType":["APP"]}]}`
	created, err := (&NotificationGroup{}).CreateNotificationGroup(&model.CreateNotificationGroupReq{
		Name: "member app", NotificationType: model.NoticeType_Member, Status: "OPEN", NotificationConfig: &config,
	}, notificationGroupTestClaims())
	if err != nil {
		t.Fatalf("CreateNotificationGroup returned error: %v", err)
	}
	var stored model.NotificationGroup
	if err := db.First(&stored, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("read stored notification group: %v", err)
	}
	if stored.NotificationType != model.NoticeType_Member || stored.NotificationConfig == nil || *stored.NotificationConfig != config {
		t.Fatalf("stored member APP group = %#v", stored)
	}
}

func TestLegacyDirectAppExecutionPersistsFailureHistory(t *testing.T) {
	seams := withD2Seams(t)
	n := &NotificationServicesConfig{}
	n.sendUnsupportedDirectAppNotification(
		&model.NotificationGroup{ID: "legacy-app", TenantID: "tenant-a", NotificationType: model.NoticeType_APP},
		&executeNotificationTemplateVars{content: "alarm body", deviceIDs: []string{"device-1"}},
	)
	if len(seams.saved) != 1 {
		t.Fatalf("saved histories = %d, want 1", len(seams.saved))
	}
	history := seams.saved[0]
	if history.NotificationType != model.NoticeType_APP || history.SendResult == nil || *history.SendResult != "FAILURE" {
		t.Fatalf("saved history = %#v, want APP failure", history)
	}
	if history.Remark == nil || *history.Remark != directAppNotificationFailureReason {
		t.Fatalf("saved history remark = %#v, want %q", history.Remark, directAppNotificationFailureReason)
	}
	if history.SendContent == nil || *history.SendContent != "alarm body" {
		t.Fatalf("saved history content = %#v, want alarm body", history.SendContent)
	}
}

func TestUpdateNotificationGroupRejectsLegacyDirectAppEffectiveState(t *testing.T) {
	db := setupNotificationGroupServiceTestDB(t)
	seed := &model.NotificationGroup{ID: "group-legacy-app", Name: "legacy", NotificationType: model.NoticeType_APP, Status: "OPEN", TenantID: "tenant-a"}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed legacy notification group: %v", err)
	}
	name := "unrelated change"
	_, err := (&NotificationGroup{}).UpdateNotificationGroup(seed.ID, &model.UpdateNotificationGroupReq{Name: &name}, notificationGroupTestClaims())
	assertDirectAppParamError(t, err)

	var stored model.NotificationGroup
	if err := db.First(&stored, "id = ?", seed.ID).Error; err != nil {
		t.Fatalf("read legacy notification group: %v", err)
	}
	if stored.Name != "legacy" || stored.NotificationType != model.NoticeType_APP {
		t.Fatalf("rejected legacy update persisted: name=%q type=%q", stored.Name, stored.NotificationType)
	}
}

func TestUpdateNotificationGroupRejectsDirectAppWithoutPersisting(t *testing.T) {
	db := setupNotificationGroupServiceTestDB(t)
	seed := &model.NotificationGroup{ID: "group-email", Name: "before", NotificationType: model.NoticeType_Email, Status: "OPEN", TenantID: "tenant-a"}
	if err := db.Create(seed).Error; err != nil {
		t.Fatalf("seed notification group: %v", err)
	}
	name := "after"
	directApp := model.NoticeType_APP
	_, err := (&NotificationGroup{}).UpdateNotificationGroup(seed.ID, &model.UpdateNotificationGroupReq{Name: &name, NotificationType: &directApp}, notificationGroupTestClaims())
	assertDirectAppParamError(t, err)

	var stored model.NotificationGroup
	if err := db.First(&stored, "id = ?", seed.ID).Error; err != nil {
		t.Fatalf("read stored notification group: %v", err)
	}
	if stored.Name != "before" || stored.NotificationType != model.NoticeType_Email {
		t.Fatalf("rejected update persisted: name=%q type=%q", stored.Name, stored.NotificationType)
	}
}
