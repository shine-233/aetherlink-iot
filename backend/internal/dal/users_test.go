// 文件用途: 覆盖 users.go 登录热路径 raw 链选择器（GetUsersByEmail/GetUsersByPhoneNumber）的回归测试。
// 核心逻辑: 构造 sqlite 内存库用户行，断言邮箱精确匹配、空输入 fail-closed 与手机号双模式匹配。
// 关键注意事项: 这些函数位于登录/刷新热路径，语义漂移会直接破坏认证链路。
// 重构建议: 后续为 GetUserListByPageWithAddress 的租户 scope 单独补充 focused 用例。

package dal

import (
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserDALTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := "users_dal_" + t.Name()
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
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
	})
	return db
}

func createDALTestUser(t *testing.T, db *gorm.DB, id, email, phone string) {
	t.Helper()
	now := time.Now().UTC()
	status := "N"
	user := &model.User{
		ID:          id,
		Name:        &id,
		Email:       email,
		PhoneNumber: phone,
		Status:      &status,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user %s: %v", id, err)
	}
}

func TestGetUsersByEmailReturnsMatchingUser(t *testing.T) {
	db := setupUserDALTestDB(t)
	createDALTestUser(t, db, "user-1", "user1@example.com", "")

	user, err := GetUsersByEmail("user1@example.com")
	if err != nil {
		t.Fatalf("GetUsersByEmail returned error: %v", err)
	}
	if user == nil || user.ID != "user-1" || user.Email != "user1@example.com" {
		t.Fatalf("user = %+v, want user-1", user)
	}
}

func TestGetUsersByEmailMissingAndBlankFailClosed(t *testing.T) {
	db := setupUserDALTestDB(t)
	createDALTestUser(t, db, "user-1", "user1@example.com", "")

	if _, err := GetUsersByEmail("missing@example.com"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing email err = %v, want gorm.ErrRecordNotFound", err)
	}
	// 空白输入必须在触达数据库前拒绝，避免全表扫描式误命中。
	if _, err := GetUsersByEmail("   "); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("blank email err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestGetUsersByPhoneNumberExactAndSuffixMatch(t *testing.T) {
	db := setupUserDALTestDB(t)
	createDALTestUser(t, db, "user-intl", "intl@example.com", "+8613800001111")
	createDALTestUser(t, db, "user-local", "local@example.com", "13900002222")

	intl, err := GetUsersByPhoneNumber("+8613800001111")
	if err != nil {
		t.Fatalf("exact intl match returned error: %v", err)
	}
	if intl == nil || intl.ID != "user-intl" {
		t.Fatalf("intl user = %+v, want user-intl", intl)
	}

	suffix, err := GetUsersByPhoneNumber("2222")
	if err != nil {
		t.Fatalf("suffix match returned error: %v", err)
	}
	if suffix == nil || suffix.ID != "user-local" {
		t.Fatalf("suffix user = %+v, want user-local", suffix)
	}

	if _, err := GetUsersByPhoneNumber(""); err == nil {
		t.Fatal("expected empty phone number to fail closed")
	}
}
