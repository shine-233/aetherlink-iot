package dal

import (
	"fmt"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserTOTPDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open totp sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.UserTOTP{}, &model.UserTOTPRecoveryCode{}); err != nil {
		t.Fatalf("migrate totp: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
}

func TestUserTOTPUpsertAndDisable(t *testing.T) {
	setupUserTOTPDB(t)
	if _, err := GetUserTOTP("u1"); err != gorm.ErrRecordNotFound {
		t.Fatalf("expect not found before binding, got %v", err)
	}
	if err := SaveUserTOTPSecret("u1", "cipher-1"); err != nil {
		t.Fatalf("save secret: %v", err)
	}
	row, err := GetUserTOTP("u1")
	if err != nil {
		t.Fatalf("get after save: %v", err)
	}
	if !row.Enabled || row.SecretCipher != "cipher-1" {
		t.Fatalf("unexpected row: enabled=%v cipher=%q", row.Enabled, row.SecretCipher)
	}
	if err := SetTOTPLastUsedStep("u1", 123); err != nil {
		t.Fatalf("set step: %v", err)
	}
	row2, _ := GetUserTOTP("u1")
	if row2.LastUsedStep != 123 {
		t.Fatalf("step not persisted: %d", row2.LastUsedStep)
	}
	if err := DisableUserTOTP("u1"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := GetUserTOTP("u1"); err != gorm.ErrRecordNotFound {
		t.Fatalf("expect removed after disable, got %v", err)
	}
}

func TestUserTOTPRecoveryOneTimeConsumption(t *testing.T) {
	setupUserTOTPDB(t)
	rows := []*model.UserTOTPRecoveryCode{
		{ID: "c1", UserID: "u1", CodeHash: "hash-a"},
		{ID: "c2", UserID: "u1", CodeHash: "hash-b"},
	}
	if err := CreateUserTOTPRecoveryCodes(rows); err != nil {
		t.Fatalf("create codes: %v", err)
	}
	used, err := ConsumeUserTOTPRecoveryCode("u1", "HASH-A")
	if err != nil || !used {
		t.Fatalf("first consume used=%v err=%v", used, err)
	}
	// 重复使用同一码必须失败（原子占位防重放）。
	used2, err := ConsumeUserTOTPRecoveryCode("u1", "hash-a")
	if err != nil || used2 {
		t.Fatalf("replay consume must fail: used=%v err=%v", used2, err)
	}
	left, err := ListUnusedRecoveryCodeHashes("u1")
	if err != nil || len(left) != 1 || left[0] != "hash-b" {
		t.Fatalf("unused list = %v err=%v", left, err)
	}
}
